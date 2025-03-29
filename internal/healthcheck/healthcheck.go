// internal/healthcheck/healthcheck.go
package healthcheck

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/bibendi/gruf-relay/internal/config"
	"github.com/bibendi/gruf-relay/internal/manager"
	"github.com/bibendi/gruf-relay/internal/process"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

type Checker struct {
	pm           *manager.Manager
	interval     time.Duration
	host         string
	serverStates map[string]connectivity.State
	stopChan     chan struct{}
	mu           sync.RWMutex
}

func NewChecker(pm *manager.Manager, cfg *config.Config) *Checker {
	return &Checker{
		pm:           pm,
		interval:     cfg.HealthCheckInterval,
		host:         cfg.Host,
		serverStates: make(map[string]connectivity.State),
		stopChan:     make(chan struct{}),
	}
}

func (c *Checker) Start() {
	go c.run()
	log.Println("Health checker started")
}

func (c *Checker) run() {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.checkAll()
		case <-c.stopChan:
			log.Println("Health checker stopped")
			return
		}
	}
}

func (c *Checker) Stop() {
	close(c.stopChan)
}

func (c *Checker) checkAll() {
	for _, server := range c.pm.Processes {
		c.checkServer(server)
	}
}

func (c *Checker) checkServer(p *process.Process) {
	if !p.IsRunning() {
		c.updateServerState(p.Name, connectivity.Shutdown)
		log.Printf("Server %s is not running, state: %s", p, connectivity.Shutdown)
		return
	}

	conn, err := grpc.Dial(p.Addr, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(3*time.Second))
	if err != nil {
		c.updateServerState(p.Name, connectivity.TransientFailure)
		log.Printf("Failed to dial server %s: %v, state: %s", p, err, connectivity.TransientFailure)
		return
	}
	defer conn.Close()

	client := healthpb.NewHealthClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req := &healthpb.HealthCheckRequest{}
	resp, err := client.Check(ctx, req)
	if err != nil {
		c.updateServerState(p.Name, connectivity.TransientFailure)
		log.Printf("Health check failed for server %s: %v, state: %s", p, err, connectivity.TransientFailure)
		return
	}

	var state connectivity.State
	switch resp.Status {
	case healthpb.HealthCheckResponse_SERVING:
		state = connectivity.Ready
	case healthpb.HealthCheckResponse_NOT_SERVING:
		state = connectivity.TransientFailure
	default:
		state = connectivity.TransientFailure
	}

	c.updateServerState(p.Name, state)
	log.Printf("Server %s health check status: %s, state: %s", p, resp.Status, state)
}

func (c *Checker) updateServerState(name string, state connectivity.State) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.serverStates[name] = state
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
