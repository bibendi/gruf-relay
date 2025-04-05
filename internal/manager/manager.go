package manager

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/bibendi/gruf-relay/internal/config"
	"github.com/bibendi/gruf-relay/internal/process"
)

type Manager struct {
	Processes map[string]*process.Process
	mu        sync.Mutex
}

func NewManager(ctx context.Context, wg *sync.WaitGroup, cfg *config.Config) *Manager {
	processes := make(map[string]*process.Process, cfg.Workers.Count)

	for i := range cfg.Workers.Count {
		name := fmt.Sprintf("worker-%d", i+1)
		port := cfg.Workers.StartPort + i
		addr := fmt.Sprintf("0.0.0.0:%d", port)
		processes[name] = process.NewProcess(ctx, wg, name, addr)
	}

	return &Manager{Processes: processes}
}

func (m *Manager) StartAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, process := range m.Processes {
		if err := process.Start(); err != nil {
			return fmt.Errorf("failed to start server %s: %w", process, err)
		}
	}

	log.Println("Servers started")

	return nil
}
