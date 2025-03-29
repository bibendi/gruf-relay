package manager

import (
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

func NewManager(cfg *config.Config) *Manager {
	processes := make(map[string]*process.Process, cfg.Workers.Count)

	for i := range cfg.Workers.Count {
		name := fmt.Sprintf("worker-%d", i+1)
		port := cfg.Workers.StartPort + i
		addr := fmt.Sprintf("0.0.0.0:%d", port)
		processes[name] = process.NewProcess(name, addr)
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
	return nil
}

func (m *Manager) StopAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, process := range m.Processes {
		if err := process.Stop(); err != nil {
			log.Printf("failed to stop server %s: %v", process, err)
		}
	}
	return nil
}
