package manager

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/bibendi/gruf-relay/internal/config"
	"github.com/bibendi/gruf-relay/internal/process"
)

type Manager struct {
	Processes map[string]*process.Process
	log       *slog.Logger
}

func NewManager(ctx context.Context, wg *sync.WaitGroup, log *slog.Logger, cfg *config.Config) *Manager {
	processes := make(map[string]*process.Process, cfg.Workers.Count)

	for i := range cfg.Workers.Count {
		name := fmt.Sprintf("worker-%d", i+1)
		port := cfg.Workers.StartPort + i
		metricsPort := port + 100
		processes[name] = process.NewProcess(ctx, wg, log, name, port, metricsPort, cfg.Workers.MetricsPath)
	}

	return &Manager{
		Processes: processes,
		log:       log,
	}
}

func (m *Manager) StartAll() error {
	for _, process := range m.Processes {
		if err := process.Start(); err != nil {
			return fmt.Errorf("failed to start server %s: %w", process, err)
		}
	}

	m.log.Info("Servers started", slog.Int("count", len(m.Processes)))

	return nil
}
