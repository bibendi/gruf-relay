package manager

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/bibendi/gruf-relay/internal/config"
	"github.com/bibendi/gruf-relay/internal/log"
	"github.com/bibendi/gruf-relay/internal/process"
	"github.com/onsi/ginkgo/v2"
)

type Manager struct {
	Processes map[string]process.Process
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
		Processes: processes,
	}
}

func (m *Manager) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	defer wg.Wait()

	log.Info("Starting manager", slog.Int("servers_count", len(m.Processes)))
	errChan := make(chan error, 1)
	defer close(errChan)

	errCtx, cancel := context.WithCancel(ctx)

	for _, p := range m.Processes {
		wg.Add(1)
		go func(p process.Process) {
			defer wg.Done()
			defer ginkgo.GinkgoRecover()
			if err := p.Run(errCtx); err != nil {
				select {
				case errChan <- err:
					log.Error("Failed to run server", slog.Any("error", err), slog.Any("worker", p))
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
