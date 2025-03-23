// internal/healthcheck/healthcheck.go
package healthcheck

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/bibendi/gruf-relay/internal/config"
	"github.com/bibendi/gruf-relay/internal/process"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

type Checker struct {
	pm           *process.Manager
	interval     time.Duration
	host         string
	serverStates map[string]connectivity.State
	stopChan     chan struct{}
	mu           sync.RWMutex
}

func NewChecker(pm *process.Manager, cfg *config.Config) *Checker {
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
	servers := c.pm.GetServers()

	for _, server := range servers {
		c.checkServer(server)
	}
}

func (c *Checker) checkServer(server process.Server) {
	address := fmt.Sprintf("%s:%d", c.host, server.Port)

	if !c.pm.IsServerRunning(server.Name) {
		c.updateServerState(server.Name, connectivity.Shutdown)
		log.Printf("Server %s is not running, state: %s", server.Name, connectivity.Shutdown)
		return
	}

	conn, err := grpc.Dial(address, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(3*time.Second)) // Reduced timeout
	if err != nil {
		c.updateServerState(server.Name, connectivity.TransientFailure)
		log.Printf("Failed to dial server %s: %v, state: %s", server.Name, err, connectivity.TransientFailure)
		return
	}
	defer conn.Close()

	client := healthpb.NewHealthClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req := &healthpb.HealthCheckRequest{}
	resp, err := client.Check(ctx, req)
	if err != nil {
		c.updateServerState(server.Name, connectivity.TransientFailure)
		log.Printf("Health check failed for server %s: %v, state: %s", server.Name, err, connectivity.TransientFailure)
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

	c.updateServerState(server.Name, state)
	log.Printf("Server %s health check status: %s, state: %s", server.Name, resp.Status, state)
}

func (c *Checker) updateServerState(serverName string, state connectivity.State) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.serverStates[serverName] = state
}

func (c *Checker) GetServerState(serverName string) connectivity.State {
	c.mu.RLock()
	defer c.mu.RUnlock()
	state, ok := c.serverStates[serverName]
	if !ok {
		return connectivity.Shutdown
	}
	return state
}
