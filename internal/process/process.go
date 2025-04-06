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

func NewProcess(ctx context.Context, wg *sync.WaitGroup, name, addr string) *Process {
	return &Process{
		Name:        name,
		Addr:        addr,
		ctx:         ctx,
		wg:          wg,
		cmdDoneChan: make(chan error, 1),
		startOnce:   sync.Once{},
	}
}

func (p *Process) String() string {
	return fmt.Sprintf("%s (%s)", p.Name, p.Addr)
}

func (p *Process) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		slog.Error("Server is already running", slog.Any("server", p))
		return nil
	}

	p.client = nil

	p.cmd = p.buildCmd()
	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start server %s: %w", p, err)
	}

	p.running = true

	p.startOnce.Do(func() {
		p.wg.Add(1)
		go p.waitCtxDone()
	})
	go p.waitCmdDone()

	slog.Info("Server started", slog.Any("server", p))
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

	slog.Info("Stopping server", slog.Any("server", p))
	p.stopping = true

	if p.client != nil {
		slog.Info("Closing client connection", slog.Any("server", p))
		if err := p.client.Close(); err != nil {
			slog.Error("Failed to close client connection", slog.Any("server", p), slog.Any("error", err))
		}
	}
	p.client = nil

	if !p.running {
		return errors.New("server is not running")
	}
	p.running = false

	// Check if the process is still alive
	if p.cmd.ProcessState != nil && p.cmd.ProcessState.Exited() {
		slog.Error("Server is already exited", slog.Any("server", p))
		return nil
	}

	// Send SIGTERM
	slog.Debug("Sending SIGTERM to server", slog.Any("server", p))
	if err := p.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		slog.Error("Failed to send SIGTERM to server", slog.Any("server", p), slog.Any("error", err))
		if err := p.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill server %s: %w", p, err)
		}
	}

	select {
	case <-p.cmdDoneChan:
		slog.Info("Server stopped", slog.Any("server", p))
	case <-time.After(5 * time.Second):
		slog.Error("Timeout waiting for server to exit, sending SIGKILL", slog.Any("server", p))
		if err := p.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill server %s: %w", p, err)
		}
	}

	return nil
}

func (p *Process) waitCtxDone() {
	<-p.ctx.Done()

	if err := p.shoutdown(); err != nil {
		slog.Error("Failed to shutdown server", slog.Any("server", p), slog.Any("error", err))
	}
}

func (p *Process) waitCmdDone() {
	err := p.cmd.Wait()
	if err != nil {
		slog.Error("Server exited unexpectedly", slog.Any("server", p), slog.Any("error", err))
	} else {
		slog.Info("Server exited normally", slog.Any("server", p))
	}

	if p.stopping {
		slog.Debug("Closing cmdDoneChan", slog.Any("server", p))
		p.cmdDoneChan <- err
		close(p.cmdDoneChan)
		slog.Debug("cmdDoneChan closed", slog.Any("server", p))
		return
	}

	slog.Debug("Setting server as not running", slog.Any("server", p))
	p.mu.Lock()
	p.running = false
	p.mu.Unlock()

	time.Sleep(2 * time.Second)

	if err := p.Start(); err != nil {
		slog.Error("Failed to restart server", slog.Any("server", p), slog.Any("error", err))
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
	return cmd
}

func (p *Process) cmdArgs() []string {
	return []string{"bundle", "exec", "gruf", "--host", p.Addr, "--health-check", "--backtrace-on-error"}
}
