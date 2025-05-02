//go:generate mockgen -source=worker.go -destination=worker_mock.go -package=worker
package worker

import (
	"context"
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

type Worker interface {
	Run(context.Context) error
	IsRunning() bool
	String() string
	Addr() string
	MetricsAddr() string
	FetchClientConn(ctx context.Context) (PulledClientConn, error)
}

type workerImpl struct {
	Name        string
	addr        string
	port        int
	metricsPort int
	metricsPath string
	poolSize    int
	log         log.Logger
	connPool    *connectionPool
	cmd         Command
	mu          sync.Mutex
	running     bool
	stopping    bool
	cmdDoneChan chan error
	cmdExecutor CommandExecutor
}

type Option func(*workerImpl)

func WithExecutor(executor CommandExecutor) Option {
	return func(w *workerImpl) {
		w.cmdExecutor = executor
	}
}

func NewWorker(name string, port, metricsPort int, metricsPath string, poolSize int, opts ...Option) *workerImpl {
	logger := log.With(slog.String("worker", name))
	addr := fmt.Sprintf("0.0.0.0:%d", port)

	w := &workerImpl{
		Name:        name,
		addr:        addr,
		port:        port,
		metricsPort: metricsPort,
		metricsPath: metricsPath,
		poolSize:    poolSize,
		cmdDoneChan: make(chan error, 1),
		log:         logger,
		connPool: newConnectionPool(poolSize, logger, func() (*grpc.ClientConn, error) {
			return grpc.NewClient(
				addr,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
				grpc.WithDefaultCallOptions(grpc.ForceCodec(codec.Codec())))
		}),
	}

	for _, opt := range opts {
		opt(w)
	}

	if w.cmdExecutor == nil {
		w.cmdExecutor = &DefaultCommandExecutor{}
	}

	return w
}

func (w *workerImpl) String() string {
	return w.Name
}

func (w *workerImpl) Addr() string {
	return w.addr
}

func (w *workerImpl) MetricsAddr() string {
	return fmt.Sprintf("0.0.0.0:%d%s", w.metricsPort, w.metricsPath)
}

func (w *workerImpl) Run(ctx context.Context) error {
	if err := w.start(); err != nil {
		return err
	}

	<-ctx.Done()
	if err := w.shutdown(); err != nil {
		w.log.Error("Failed to shutdown worker", slog.Any("error", err))
		return err
	}
	return nil
}

func (w *workerImpl) start() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		w.log.Error("Worker is already running")
		return nil
	}

	if w.stopping {
		w.log.Warn("Worker is stopping, will not start again")
		return nil
	}

	w.log.Info("Starting worker")

	w.connPool.close()

	w.buildCmd()
	if err := w.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start worker %s: %w", w, err)
	}

	w.running = true

	go w.waitCmdDone()

	w.log.Info("Worker started")
	return nil
}

func (w *workerImpl) IsRunning() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running
}

func (w *workerImpl) FetchClientConn(ctx context.Context) (PulledClientConn, error) {
	w.log.Debug("Waiting for available connection")
	// TODO: add ability to configure timeout
	fetchCtx, fetchCancel := context.WithTimeout(ctx, 5*time.Second)
	defer fetchCancel()

	conn, err := w.connPool.fetchConn(fetchCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch gRPC client connection: %v", err)
	}
	return conn, nil
}

func (w *workerImpl) shutdown() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.log.Info("Stopping worker")
	w.stopping = true

	w.connPool.close()

	if !w.running {
		w.log.Warn("Worker is not running, no need to shutdown")
		return nil
	}
	w.running = false

	// Check if the process is still alive
	if state := w.cmd.ProcessState(); state != nil && state.Exited() {
		w.log.Warn("Worker is already exited")
		return nil
	}

	// Send SIGTERM
	w.log.Debug("Sending SIGTERM to worker")
	if err := w.cmd.Stop(); err != nil {
		w.log.Error("Failed to send SIGTERM to worker", slog.Any("error", err))
		if err := w.cmd.Kill(); err != nil {
			return fmt.Errorf("failed to kill worker %s: %w", w, err)
		}
	}

	select {
	case <-w.cmdDoneChan:
		w.log.Info("Worker stopped")
	case <-time.After(5 * time.Second):
		w.log.Error("Timeout waiting for worker to exit, sending SIGKILL")
		if err := w.cmd.Kill(); err != nil {
			return fmt.Errorf("failed to kill worker %s: %w", w, err)
		}
	}

	return nil
}

func (w *workerImpl) waitCmdDone() {
	err := w.cmd.Wait()
	if err != nil {
		w.log.Error("Worker exited unexpectedly", slog.Any("error", err))
	} else {
		w.log.Info("Worker exited normally")
	}

	if w.stopping {
		w.log.Debug("Closing cmdDoneChan")
		w.cmdDoneChan <- err
		close(w.cmdDoneChan)
		w.log.Debug("cmdDoneChan closed")
		return
	}

	w.log.Debug("Setting worker as not running")
	w.mu.Lock()
	w.running = false
	w.mu.Unlock()

	time.Sleep(1 * time.Second)

	if err := w.start(); err != nil {
		w.log.Error("Failed to restart worker", slog.Any("error", err))
	}
}

func (w *workerImpl) buildCmd() {
	args := w.cmdArgs()
	w.cmd = w.cmdExecutor.NewCommand(args[0], args[1:]...)

	cmdEnv := os.Environ()
	cmdEnv = append(cmdEnv, fmt.Sprintf("PROMETHEUS_EXPORTER_PORT=%d", w.metricsPort))
	cmdEnv = append(cmdEnv, fmt.Sprintf("PROMETHEUS_EXPORTER_PATH=%s", w.metricsPath))
	cmdEnv = append(cmdEnv, fmt.Sprintf("RAILS_MAX_THREADS=%d", w.poolSize))
	w.cmd.SetEnv(cmdEnv)
	w.log.Debug("Command built", "command", args)
}

func (w *workerImpl) cmdArgs() []string {
	return []string{"bundle", "exec", "gruf", "--host", w.Addr(), "--health-check", "--backtrace-on-error"}
}
