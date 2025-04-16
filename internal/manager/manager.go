package manager

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/bibendi/gruf-relay/internal/config"
	log "github.com/bibendi/gruf-relay/internal/logger"
	"github.com/bibendi/gruf-relay/internal/process"
)

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
		processes[name] = process.NewProcess(name, port, metricsPort, cfg.MetricsPath)
	}

	return &Manager{
		Processes: processes,
	}
}

func (m *Manager) Run(ctx context.Context) error {
	var wg sync.WaitGroup

	log.Info("Starting manager", slog.Int("servers_count", len(m.Processes)))
	errChan := make(chan error, 1)
	defer close(errChan)

	errCtx, cancel := context.WithCancel(ctx)

	for _, p := range m.Processes {
		wg.Add(1)
		go func(p process.Process) {
			defer wg.Done()
			if err := p.Run(errCtx); err != nil {
				select {
				case errChan <- err:
					log.Error("Failed to run server", slog.Any("error", err), slog.Any("server", p))
					cancel()
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

	return nil
}
