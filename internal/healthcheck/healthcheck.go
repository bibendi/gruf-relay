//go:generate mockgen -source=healthcheck.go -destination=healthcheck_mock.go -package=healthcheck
package healthcheck

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/bibendi/gruf-relay/internal/config"
	"github.com/bibendi/gruf-relay/internal/log"
	"github.com/bibendi/gruf-relay/internal/process"
	"github.com/onsi/ginkgo/v2"
	"google.golang.org/grpc/connectivity"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

type Balancer interface {
	AddProcess(process.Process)
	RemoveProcess(process.Process)
}

type HealthCheckFunc func(ctx context.Context, p process.Process) (healthpb.HealthCheckResponse_ServingStatus, error)

type Checker struct {
	processes     map[string]process.Process
	lb            Balancer
	interval      time.Duration
	timeout       time.Duration
	workerStates  map[string]connectivity.State
	mu            sync.RWMutex
	healthCheckFn HealthCheckFunc
}

func NewChecker(cfg config.HealthCheck, processes map[string]process.Process, lb Balancer, healthCheckFn HealthCheckFunc) *Checker {
	if healthCheckFn == nil {
		healthCheckFn = defaultHealthCheck
	}

	return &Checker{
		processes:     processes,
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
	for _, p := range c.processes {
		wg.Add(1)
		go func(p process.Process) {
			defer wg.Done()
			defer ginkgo.GinkgoRecover()
			state := c.checkWorker(p)
			c.updateWorkerState(p.String(), state)
		}(p)
	}

	wg.Wait()
}

func (c *Checker) checkWorker(p process.Process) connectivity.State {
	if !p.IsRunning() {
		c.lb.RemoveProcess(p)
		log.Error("Worker is not running", slog.Any("worker", p), slog.Any("state", connectivity.Shutdown))
		return connectivity.Shutdown
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	status, err := c.healthCheckFn(ctx, p)
	if err != nil {
		c.lb.RemoveProcess(p)
		log.Error("Health check failed", slog.Any("worker", p), slog.Any("error", err), slog.Any("state", connectivity.TransientFailure))
		return connectivity.TransientFailure
	}

	var state connectivity.State
	switch status {
	case healthpb.HealthCheckResponse_SERVING:
		state = connectivity.Ready
		c.lb.AddProcess(p)
	case healthpb.HealthCheckResponse_NOT_SERVING:
		state = connectivity.TransientFailure
		c.lb.RemoveProcess(p)
	default:
		state = connectivity.TransientFailure
		c.lb.RemoveProcess(p)
	}

	log.Info("Worker is healthy", slog.Any("worker", p), slog.Any("state", state))
	return state
}

func (c *Checker) updateWorkerState(name string, state connectivity.State) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.workerStates[name] = state
}

func defaultHealthCheck(ctx context.Context, p process.Process) (healthpb.HealthCheckResponse_ServingStatus, error) {
	grpcClient, err := p.GetClient()
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
