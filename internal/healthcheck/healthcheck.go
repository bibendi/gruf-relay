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
	healthpb "google.golang.org/grpc/health/grpc_health_v1" // Импорт protobuf для health check
)

// Checker выполняет периодическую проверку здоровья Ruby GRPC серверов.
type Checker struct {
	pm           *process.Manager
	interval     time.Duration
	host         string                        // Added host
	serverStates map[string]connectivity.State // Состояние серверов
	stopChan     chan struct{}                 // Канал для остановки healthcheck
	mu           sync.RWMutex                  // Add a read-write mutex
}

// NewChecker создает новый экземпляр Health Checker.
func NewChecker(pm *process.Manager, cfg *config.Config) *Checker { // Modified to accept config
	return &Checker{
		pm:           pm,
		interval:     cfg.HealthCheckInterval,
		host:         cfg.Host, // Assign the host
		serverStates: make(map[string]connectivity.State),
		stopChan:     make(chan struct{}),
	}
}

// Start запускает Health Checker.
func (c *Checker) Start() {
	go c.run() // Launch the run method in a goroutine
	log.Println("Health checker started")
}

// run содержит основную логику проверки здоровья в цикле.
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

// Stop останавливает Health Checker.
func (c *Checker) Stop() {
	close(c.stopChan)
}

// checkAll проверяет состояние всех Ruby GRPC серверов.
func (c *Checker) checkAll() {
	servers := c.pm.GetServers() // Get a copy of the server list

	for _, server := range servers {
		c.checkServer(server)
	}
}

// checkServer проверяет состояние одного Ruby GRPC сервера с использованием GRPC health check.
func (c *Checker) checkServer(server process.Server) {
	address := fmt.Sprintf("%s:%d", c.host, server.Port)

	// 1. Проверяем, запущен ли процесс
	if !c.pm.IsServerRunning(server.Name) {
		c.updateServerState(server.Name, connectivity.Shutdown)
		log.Printf("Server %s is not running, state: %s", server.Name, connectivity.Shutdown)
		return
	}

	// 2. Пытаемся установить соединение
	conn, err := grpc.Dial(address, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(3*time.Second)) // Reduced timeout
	if err != nil {
		c.updateServerState(server.Name, connectivity.TransientFailure)
		log.Printf("Failed to dial server %s: %v, state: %s", server.Name, err, connectivity.TransientFailure)
		return
	}
	defer conn.Close()

	// 3. Выполняем GRPC health check
	client := healthpb.NewHealthClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

	req := &healthpb.HealthCheckRequest{} // Пустой запрос, так как проверяем общий статус сервера
	resp, err := client.Check(ctx, req)
	if err != nil {
			c.updateServerState(server.Name, connectivity.TransientFailure)
		log.Printf("Health check failed for server %s: %v, state: %s", server.Name, err, connectivity.TransientFailure)
		return
		}

	// 4. Интерпретируем результат health check
	var state connectivity.State
	switch resp.Status {
	case healthpb.HealthCheckResponse_SERVING:
		state = connectivity.Ready
	case healthpb.HealthCheckResponse_NOT_SERVING:
		state = connectivity.TransientFailure
	default:
		state = connectivity.TransientFailure // Или другое подходящее состояние для UNKNOWN
	}

	c.updateServerState(server.Name, state)
	log.Printf("Server %s health check status: %s, state: %s", server.Name, resp.Status, state)
}

// updateServerState обновляет состояние сервера в map.
func (c *Checker) updateServerState(serverName string, state connectivity.State) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.serverStates[serverName] = state
}

// GetServerState возвращает состояние сервера.
func (c *Checker) GetServerState(serverName string) connectivity.State {
	c.mu.RLock() // Use RLock for read access
	defer c.mu.RUnlock()
	state, ok := c.serverStates[serverName]
	if !ok {
		return connectivity.Shutdown // Or another appropriate default
	}
	return state
}
