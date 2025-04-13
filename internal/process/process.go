package process

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/bibendi/gruf-relay/internal/codec"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Process struct {
	Name        string
	Addr        string
	MetricsAddr string
	port        int
	metricsPort int
	metricsPath string
	log         *slog.Logger
	client      *grpc.ClientConn
	ctx         context.Context
	wg          *sync.WaitGroup
	cmd         *exec.Cmd
	mu          sync.Mutex
	running     bool
	stopping    bool
	cmdDoneChan chan error
	startOnce   sync.Once
}

func NewProcess(ctx context.Context, wg *sync.WaitGroup, logger *slog.Logger, name string, port, metricsPort int, metricsPath string) *Process {
	log := logger.With(slog.String("process", name))
	return &Process{
		Name:        name,
		Addr:        fmt.Sprintf("0.0.0.0:%d", port),
		MetricsAddr: fmt.Sprintf("0.0.0.0:%d/%s", metricsPort, metricsPath),
		port:        port,
		metricsPort: metricsPort,
		metricsPath: metricsPath,
		ctx:         ctx,
		wg:          wg,
		cmdDoneChan: make(chan error, 1),
		startOnce:   sync.Once{},
		log:         log,
	}
}

func (p *Process) String() string {
	return p.Name
}

func (p *Process) Start() error {
	p.wg.Add(1)
	go p.waitCtxDone()
	if err := p.start(); err != nil {
		return err
	}
	return nil
}

func (p *Process) start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		p.log.Error("Server is already running")
		return nil
	}

	p.client = nil

	p.cmd = p.buildCmd()
	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start server %s: %w", p, err)
	}

	p.running = true

	go p.waitCmdDone()

	p.log.Info("Server started")
	return nil
}

func (p *Process) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

func (p *Process) GetClient() (*grpc.ClientConn, error) {
	if p.client == nil {
		p.mu.Lock()
		defer p.mu.Unlock()
		if p.client == nil {
			client, err := grpc.NewClient(
				p.Addr,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
				grpc.WithCodec(codec.Codec()))
			if err != nil {
				return nil, fmt.Errorf("failed creating new client for server %s: %v", p, err)
			}
			p.client = client
		}
	}
	return p.client, nil
}

func (p *Process) shoutdown() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	defer p.wg.Done()

	p.log.Info("Stopping server")
	p.stopping = true

	if p.client != nil {
		p.log.Info("Closing client connection")
		if err := p.client.Close(); err != nil {
			p.log.Error("Failed to close client connection", slog.Any("error", err))
		}
	}
	p.client = nil

	if !p.running {
		return errors.New("server is not running")
	}
	p.running = false

	// Check if the process is still alive
	if p.cmd.ProcessState != nil && p.cmd.ProcessState.Exited() {
		p.log.Error("Server is already exited")
		return nil
	}

	// Send SIGTERM
	p.log.Debug("Sending SIGTERM to server")
	if err := p.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		p.log.Error("Failed to send SIGTERM to server", slog.Any("error", err))
		if err := p.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill server %s: %w", p, err)
		}
	}

	select {
	case <-p.cmdDoneChan:
		p.log.Info("Server stopped")
	case <-time.After(5 * time.Second):
		p.log.Error("Timeout waiting for server to exit, sending SIGKILL")
		if err := p.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill server %s: %w", p, err)
		}
	}

	return nil
}

func (p *Process) waitCtxDone() {
	<-p.ctx.Done()

	if err := p.shoutdown(); err != nil {
		p.log.Error("Failed to shutdown server", slog.Any("error", err))
	}
}

func (p *Process) waitCmdDone() {
	err := p.cmd.Wait()
	if err != nil {
		p.log.Error("Server exited unexpectedly", slog.Any("error", err))
	} else {
		p.log.Info("Server exited normally")
	}

	if p.stopping {
		p.log.Debug("Closing cmdDoneChan")
		p.cmdDoneChan <- err
		close(p.cmdDoneChan)
		p.log.Debug("cmdDoneChan closed")
		return
	}

	p.log.Debug("Setting server as not running")
	p.mu.Lock()
	p.running = false
	p.mu.Unlock()

	time.Sleep(2 * time.Second)

	if err := p.start(); err != nil {
		p.log.Error("Failed to restart server", slog.Any("error", err))
	}
}

func (p *Process) buildCmd() *exec.Cmd {
	args := p.cmdArgs()
	cmd := exec.Command(args[0], args[1:]...)
	// Allow to exec programs in the current directory
	if errors.Is(cmd.Err, exec.ErrDot) {
		cmd.Err = nil
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("PROMETHEUS_EXPORTER_PORT=%d", p.metricsPort))
	cmd.Env = append(cmd.Env, fmt.Sprintf("PROMETHEUS_EXPORTER_PATH=%s", p.metricsPath))
	return cmd
}

func (p *Process) cmdArgs() []string {
	return []string{"bundle", "exec", "gruf", "--host", p.Addr, "--health-check", "--backtrace-on-error"}
}
