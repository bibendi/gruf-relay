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

// checkServer проверяет состояние одного Ruby GRPC сервера.
func (c *Checker) checkServer(server process.Server) {
	address := fmt.Sprintf("%s:%d", c.host, server.Port) // Use c.host

	// 1. Проверяем, запущен ли процесс
	if !c.pm.IsRunning(server.Name) {
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

	// 3. Проверяем состояние соединения
	state := conn.GetState()
	c.updateServerState(server.Name, state)
	log.Printf("Server %s state: %s", server.Name, state)

	// 4. Если соединение не готово, ждем изменения состояния
	if state != connectivity.Ready {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second) // Reduced timeout
		defer cancel()
		if !conn.WaitForStateChange(ctx, state) {
			log.Printf("Server %s did not become ready in time", server.Name)
			c.updateServerState(server.Name, connectivity.TransientFailure)
		} else {
			newState := conn.GetState()
			c.updateServerState(server.Name, newState)
			log.Printf("Server %s state changed to: %s", server.Name, newState)
		}
	}
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
