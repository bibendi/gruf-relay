//go:generate mockgen -source=healthcheck.go -destination=healthcheck_mock.go -package=healthcheck
package healthcheck

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/bibendi/gruf-relay/internal/config"
	"github.com/bibendi/gruf-relay/internal/log"
	"github.com/bibendi/gruf-relay/internal/worker"
	"google.golang.org/grpc/connectivity"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

type Balancer interface {
	AddWorker(worker.Worker)
	RemoveWorker(worker.Worker)
}

type HealthCheckFunc func(ctx context.Context, w worker.Worker) (healthpb.HealthCheckResponse_ServingStatus, error)

type Checker struct {
	workers       map[string]worker.Worker
	lb            Balancer
	interval      time.Duration
	timeout       time.Duration
	workerStates  map[string]connectivity.State
	mu            sync.RWMutex
	healthCheckFn HealthCheckFunc
}

func NewChecker(cfg config.HealthCheck, workers map[string]worker.Worker, lb Balancer, healthCheckFn HealthCheckFunc) *Checker {
	if healthCheckFn == nil {
		healthCheckFn = defaultHealthCheck
	}

	return &Checker{
		workers:       workers,
		lb:            lb,
		interval:      cfg.Interval,
		timeout:       cfg.Timeout,
		workerStates:  make(map[string]connectivity.State),
		healthCheckFn: healthCheckFn,
	}
}

func (c *Checker) Run(ctx context.Context) {
	log.Info("Starting Health checking")

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.checkAll()
		case <-ctx.Done():
			log.Info("Stopping health checker")
			return
		}
	}
}

func (c *Checker) GetServerState(name string) connectivity.State {
	c.mu.RLock()
	defer c.mu.RUnlock()
	state, ok := c.workerStates[name]
	if !ok {
		return connectivity.Shutdown
	}
	return state
}

func (c *Checker) checkAll() {
	var wg sync.WaitGroup
	for _, w := range c.workers {
		wg.Add(1)
		go func(w worker.Worker) {
			defer wg.Done()
			state := c.checkWorker(w)
			c.updateWorkerState(w.String(), state)
		}(w)
	}

	wg.Wait()
}

func (c *Checker) checkWorker(w worker.Worker) connectivity.State {
	if !w.IsRunning() {
		c.lb.RemoveWorker(w)
		log.Error("Worker is not running", slog.Any("worker", w), slog.Any("state", connectivity.Shutdown))
		return connectivity.Shutdown
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	status, err := c.healthCheckFn(ctx, w)
	if err != nil {
		c.lb.RemoveWorker(w)
		log.Error("Health check failed", slog.Any("worker", w), slog.Any("error", err), slog.Any("state", connectivity.TransientFailure))
		return connectivity.TransientFailure
	}

	var state connectivity.State
	switch status {
	case healthpb.HealthCheckResponse_SERVING:
		state = connectivity.Ready
		c.lb.AddWorker(w)
	case healthpb.HealthCheckResponse_NOT_SERVING:
		state = connectivity.TransientFailure
		c.lb.RemoveWorker(w)
	default:
		state = connectivity.TransientFailure
		c.lb.RemoveWorker(w)
	}

	log.Info("Worker is healthy", slog.Any("worker", w), slog.Any("state", state))
	return state
}

func (c *Checker) updateWorkerState(name string, state connectivity.State) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.workerStates[name] = state
}

func defaultHealthCheck(ctx context.Context, w worker.Worker) (healthpb.HealthCheckResponse_ServingStatus, error) {
	grpcClient, err := w.GetClient()
	if err != nil {
		return healthpb.HealthCheckResponse_UNKNOWN, err
	}

	healthClient := healthpb.NewHealthClient(grpcClient)
	req := &healthpb.HealthCheckRequest{}
	resp, err := healthClient.Check(ctx, req)
	if err != nil {
		return healthpb.HealthCheckResponse_UNKNOWN, err
	}

	return resp.Status, nil
}
