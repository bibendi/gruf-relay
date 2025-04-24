//go:generate mockgen -source=process.go -destination=process_mock.go -package=process
package process

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/bibendi/gruf-relay/internal/codec"
	"github.com/bibendi/gruf-relay/internal/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Process interface {
	Run(context.Context) error
	IsRunning() bool
	String() string
	Addr() string
	MetricsAddr() string
	GetClient() (*grpc.ClientConn, error)
}

type processImpl struct {
	Name        string
	port        int
	metricsPort int
	metricsPath string
	log         log.Logger
	client      *grpc.ClientConn
	cmd         Command
	mu          sync.Mutex
	running     bool
	stopping    bool
	cmdDoneChan chan error
	cmdExecutor CommandExecutor
}

type Option func(*processImpl)

func WithExecutor(executor CommandExecutor) Option {
	return func(p *processImpl) {
		p.cmdExecutor = executor
	}
}

func NewProcess(name string, port, metricsPort int, metricsPath string, opts ...Option) *processImpl {
	logger := log.With(slog.String("worker", name))
	p := &processImpl{
		Name:        name,
		port:        port,
		metricsPort: metricsPort,
		metricsPath: metricsPath,
		cmdDoneChan: make(chan error, 1),
		log:         logger,
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

func (p *processImpl) String() string {
	return p.Name
}

func (p *processImpl) Addr() string {
	return fmt.Sprintf("0.0.0.0:%d", p.port)
}

func (p *processImpl) MetricsAddr() string {
	return fmt.Sprintf("0.0.0.0:%d%s", p.metricsPort, p.metricsPath)
}

func (p *processImpl) Run(ctx context.Context) error {
	if err := p.start(); err != nil {
		return err
	}

	<-ctx.Done()
	if err := p.shutdown(); err != nil {
		p.log.Error("Failed to shutdown worker", slog.Any("error", err))
		return err
	}
	return nil
}

func (p *processImpl) start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		p.log.Error("Worker is already running")
		return nil
	}

	p.log.Info("Starting worker")

	p.client = nil

	p.buildCmd()
	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start worker %s: %w", p, err)
	}

	p.running = true

	go p.waitCmdDone()

	p.log.Info("Worker started")
	return nil
}

func (p *processImpl) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

func (p *processImpl) GetClient() (*grpc.ClientConn, error) {
	if p.client == nil {
		p.mu.Lock()
		defer p.mu.Unlock()
		if p.client == nil {
			client, err := grpc.NewClient(
				p.Addr(),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
				grpc.WithCodec(codec.Codec()))
			if err != nil {
				return nil, fmt.Errorf("failed creating new client for worker %s: %v", p, err)
			}
			p.client = client
		}
	}
	return p.client, nil
}

func (p *processImpl) shutdown() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.log.Info("Stopping worker")
	p.stopping = true

	if p.client != nil {
		p.log.Info("Closing client connection")
		if err := p.client.Close(); err != nil {
			p.log.Error("Failed to close client connection", slog.Any("error", err))
		}
	}
	p.client = nil

	if !p.running {
		return errors.New("worker is not running")
	}
	p.running = false

	// Check if the process is still alive
	if state := p.cmd.ProcessState(); state != nil && state.Exited() {
		p.log.Error("Worker is already exited")
		return nil
	}

	// Send SIGTERM
	p.log.Debug("Sending SIGTERM to worker")
	if err := p.cmd.Stop(); err != nil {
		p.log.Error("Failed to send SIGTERM to worker", slog.Any("error", err))
		if err := p.cmd.Kill(); err != nil {
			return fmt.Errorf("failed to kill worker %s: %w", p, err)
		}
	}

	select {
	case <-p.cmdDoneChan:
		p.log.Info("Worker stopped")
	case <-time.After(5 * time.Second):
		p.log.Error("Timeout waiting for worker to exit, sending SIGKILL")
		if err := p.cmd.Kill(); err != nil {
			return fmt.Errorf("failed to kill worker %s: %w", p, err)
		}
	}

	return nil
}

func (p *processImpl) waitCmdDone() {
	err := p.cmd.Wait()
	if err != nil {
		p.log.Error("Worker exited unexpectedly", slog.Any("error", err))
	} else {
		p.log.Info("Worker exited normally")
	}

	if p.stopping {
		p.log.Debug("Closing cmdDoneChan")
		p.cmdDoneChan <- err
		close(p.cmdDoneChan)
		p.log.Debug("cmdDoneChan closed")
		return
	}

	p.log.Debug("Setting worker as not running")
	p.mu.Lock()
	p.running = false
	p.mu.Unlock()

	time.Sleep(2 * time.Second)

	if err := p.start(); err != nil {
		p.log.Error("Failed to restart worker", slog.Any("error", err))
	}
}

func (p *processImpl) buildCmd() {
	args := p.cmdArgs()
	p.cmd = p.cmdExecutor.NewCommand(args[0], args[1:]...)

	cmdEnv := os.Environ()
	cmdEnv = append(cmdEnv, fmt.Sprintf("PROMETHEUS_EXPORTER_PORT=%d", p.metricsPort))
	cmdEnv = append(cmdEnv, fmt.Sprintf("PROMETHEUS_EXPORTER_PATH=%s", p.metricsPath))
	p.cmd.SetEnv(cmdEnv)
	p.log.Debug("Command built", "command", args)
}

func (p *processImpl) cmdArgs() []string {
	return []string{"bundle", "exec", "gruf", "--host", p.Addr(), "--health-check", "--backtrace-on-error"}
}
