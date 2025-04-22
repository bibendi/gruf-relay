package manager

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/bibendi/gruf-relay/internal/config"
	"github.com/bibendi/gruf-relay/internal/log"
	"github.com/bibendi/gruf-relay/internal/process"
)

type Manager struct {
	processes map[string]process.Process
}

func NewManager(cfg config.Workers) *Manager {
	processes := make(map[string]process.Process, cfg.Count)

	for i := range cfg.Count {
		name := fmt.Sprintf("worker-%d", i+1)
		port := cfg.StartPort + i
		metricsPort := port + 100
		processes[name] = process.NewProcess(name, port, metricsPort, cfg.MetricsPath)
	}

	return &Manager{
		processes: processes,
	}
}

func (m *Manager) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	defer wg.Wait()

	log.Info("Starting manager", slog.Int("workers_count", len(m.processes)))
	errChan := make(chan error, 1)
	defer close(errChan)

	errCtx, cancel := context.WithCancel(ctx)

	for _, p := range m.processes {
		wg.Add(1)
		go func(p process.Process) {
			defer wg.Done()
			if err := p.Run(errCtx); err != nil {
				select {
				case errChan <- err:
					log.Error("Failed to run worker", slog.Any("error", err), slog.Any("worker", p))
				default:
				}
			}
		}(p)
	}

	var err error
	select {
	case err = <-errChan:
	case <-ctx.Done():
	}

	cancel()
	return err
}

func (m *Manager) GetWorkers() map[string]process.Process {
	return m.processes
}

func (m *Manager) GetWorkerNames() []string {
	names := make([]string, 0, len(m.processes))
	for k := range m.processes {
		names = append(names, k)
	}

	return names
}
