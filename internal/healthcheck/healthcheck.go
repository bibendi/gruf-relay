// internal/healthcheck/healthcheck.go
package healthcheck

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/bibendi/gruf-relay/internal/config"
	"github.com/bibendi/gruf-relay/internal/loadbalance"
	"github.com/bibendi/gruf-relay/internal/logger"
	"github.com/bibendi/gruf-relay/internal/process"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

var log = logger.NewPackageLogger("package", "healthcheck")

type Checker struct {
	processes    map[string]process.Process
	lb           *loadbalance.RandomBalancer
	interval     time.Duration
	host         string
	serverStates map[string]connectivity.State
	mu           sync.RWMutex
	ctx          context.Context
	wg           *sync.WaitGroup
}

func NewChecker(ctx context.Context, wg *sync.WaitGroup, processes map[string]process.Process, cfg *config.Config, lb *loadbalance.RandomBalancer) *Checker {
	return &Checker{
		processes:    processes,
		lb:           lb,
		interval:     cfg.HealthCheckInterval,
		host:         cfg.Host,
		serverStates: make(map[string]connectivity.State),
		ctx:          ctx,
		wg:           wg,
	}
}

func (c *Checker) Start() {
	c.wg.Add(1)
	go c.run()
	log.Info("Health checker started")
}

func (c *Checker) GetServerState(name string) connectivity.State {
	c.mu.RLock()
	defer c.mu.RUnlock()
	state, ok := c.serverStates[name]
	if !ok {
		return connectivity.Shutdown
	}
	return state
}

func (c *Checker) run() {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	defer c.wg.Done()

	for {
		select {
		case <-ticker.C:
			c.checkAll()
		case <-c.ctx.Done():
			log.Info("Health checker stopped")
			return
		}
	}
}

// TODO: check servers in parallel
func (c *Checker) checkAll() {
	for _, server := range c.processes {
		c.checkServer(server)
	}
}

func (c *Checker) checkServer(p process.Process) {
	if !p.IsRunning() {
		c.lb.RemoveProcess(p)
		c.updateServerState(p.String(), connectivity.Shutdown)
		log.Error("Server is not running", slog.Any("server", p), slog.Any("state", connectivity.Shutdown))
		return
	}

	conn, err := grpc.Dial(p.Addr(), grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(3*time.Second))
	if err != nil {
		c.lb.RemoveProcess(p)
		c.updateServerState(p.String(), connectivity.TransientFailure)
		log.Error("Failed to dial server", slog.Any("server", p), slog.Any("error", err), slog.Any("state", connectivity.TransientFailure))
		return
	}
	defer conn.Close()

	client := healthpb.NewHealthClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req := &healthpb.HealthCheckRequest{}
	resp, err := client.Check(ctx, req)
	if err != nil {
		c.lb.RemoveProcess(p)
		c.updateServerState(p.String(), connectivity.TransientFailure)
		log.Error("Health check failed for server", slog.Any("server", p), slog.Any("error", err), slog.Any("state", connectivity.TransientFailure))
		return
	}

	var state connectivity.State
	switch resp.Status {
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

	c.updateServerState(p.String(), state)
	log.Info("Server is healthy", slog.Any("server", p), slog.Any("status", resp.Status), slog.Any("state", state))
}

func (c *Checker) updateServerState(name string, state connectivity.State) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.serverStates[name] = state
}
