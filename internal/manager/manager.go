package manager

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/bibendi/gruf-relay/internal/config"
	"github.com/bibendi/gruf-relay/internal/log"
	"github.com/bibendi/gruf-relay/internal/worker"
)

type Manager struct {
	workers map[string]worker.Worker
}

func NewManager(cfg config.Workers) *Manager {
	workers := make(map[string]worker.Worker, cfg.Count)

	for i := range cfg.Count {
		name := fmt.Sprintf("worker-%d", i+1)
		port := cfg.StartPort + i
		metricsPort := port + 100
		workers[name] = worker.NewWorker(name, port, metricsPort, cfg.MetricsPath)
	}

	return &Manager{
		workers: workers,
	}
}

func (m *Manager) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	defer wg.Wait()

	log.Info("Starting manager", slog.Int("workers_count", len(m.workers)))
	errChan := make(chan error, 1)
	defer close(errChan)

	errCtx, cancel := context.WithCancel(ctx)

	for _, w := range m.workers {
		wg.Add(1)
		go func(w worker.Worker) {
			defer wg.Done()
			if err := w.Run(errCtx); err != nil {
				select {
				case errChan <- err:
					log.Error("Failed to run worker", slog.Any("error", err), slog.Any("worker", w))
				default:
				}
			}
		}(w)
	}

	var err error
	select {
	case err = <-errChan:
	case <-ctx.Done():
	}

	cancel()
	return err
}

func (m *Manager) GetWorkers() map[string]worker.Worker {
	return m.workers
}

func (m *Manager) GetWorkerNames() []string {
	names := make([]string, 0, len(m.workers))
	for k := range m.workers {
		names = append(names, k)
	}

	return names
}
