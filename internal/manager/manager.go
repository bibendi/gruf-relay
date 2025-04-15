package manager

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/bibendi/gruf-relay/internal/config"
	"github.com/bibendi/gruf-relay/internal/logger"
	"github.com/bibendi/gruf-relay/internal/process"
)

var log = logger.AppLogger.With("package", "manager")

type Manager struct {
	Processes map[string]process.Process
}

func NewManager(ctx context.Context, wg *sync.WaitGroup) *Manager {
	cfg := config.AppConfig.Workers
	processes := make(map[string]process.Process, cfg.Count)

	for i := range cfg.Count {
		name := fmt.Sprintf("worker-%d", i+1)
		port := cfg.StartPort + i
		metricsPort := port + 100
		processes[name] = process.NewProcess(ctx, wg, name, port, metricsPort, cfg.MetricsPath)
	}

	return &Manager{
		Processes: processes,
	}
}

func (m *Manager) StartAll() error {
	var wg sync.WaitGroup
	errChan := make(chan error, 1)
	defer close(errChan)

	for _, p := range m.Processes {
		wg.Add(1)
		go func(p process.Process) {
			defer wg.Done()
			if err := p.Start(); err != nil {
				select {
				case errChan <- err:
				default:
				}
			}
		}(p)
	}

	wg.Wait()

	select {
	case err := <-errChan:
		return err
	default:
	}

	log.Info("All servers started", slog.Int("count", len(m.Processes)))

	return nil
}
