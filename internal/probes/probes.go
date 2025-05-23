//go:generate mockgen -source=probes.go -destination=probes_mock.go -package=probes
package probes

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync/atomic"

	"github.com/bibendi/gruf-relay/internal/config"
	"github.com/bibendi/gruf-relay/internal/log"
	"google.golang.org/grpc/connectivity"
)

type HealthChecker interface {
	GetServerState(name string) connectivity.State
}

type Manager interface {
	GetWorkerNames() []string
}

type Probes struct {
	port         int
	appIsStarted *atomic.Value
	m            Manager
	hc           HealthChecker
}

func NewProbes(cfg config.Probes, isStarted *atomic.Value, m Manager, hc HealthChecker) *Probes {
	probes := &Probes{
		port:         cfg.Port,
		m:            m,
		hc:           hc,
		appIsStarted: isStarted,
	}

	return probes
}

func (p *Probes) Serve(ctx context.Context) error {
	log.Info("Starting probes server", slog.Int("port", p.port))

	mux := http.NewServeMux()

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", p.port),
		Handler: mux,
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}

	mux.HandleFunc("/startup", p.handleStartupProbe(p.appIsStarted))
	mux.HandleFunc("/readiness", p.handleReadinessProbe(p.m, p.hc))
	mux.HandleFunc("/liveness", p.handleLivenessrobe(p.m, p.hc))

	errChan := make(chan error, 1)
	defer close(errChan)

	go func() {
		if err := server.ListenAndServe(); err != nil {
			if err != http.ErrServerClosed {
				log.Error("Probes server failed", slog.Any("error", err))
				errChan <- err
			}
		}
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		log.Info("Stopping probes server")
		if err := server.Shutdown(context.Background()); err != nil {
			log.Error("Failed to shutdown probes server", slog.Any("error", err))
			return err
		}
	}
	return nil
}

func (p *Probes) handleStartupProbe(isStarted *atomic.Value) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Info("Received startup request")
		if isStarted.Load() == false {
			log.Error("Relay is not started yet, returning 503")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func (p *Probes) handleReadinessProbe(m Manager, hc HealthChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Info("Received readiness request")
		for _, name := range m.GetWorkerNames() {
			if state := hc.GetServerState(name); state == connectivity.TransientFailure || state == connectivity.Shutdown {
				log.Error("Readiness probe failed", slog.Any("worker", name), slog.String("state", state.String()))
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	}
}

func (p *Probes) handleLivenessrobe(m Manager, hc HealthChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Info("Received liveness request")
		for _, name := range m.GetWorkerNames() {
			if state := hc.GetServerState(name); state == connectivity.Shutdown {
				log.Error("Liveness probe failed", slog.Any("worker", name), slog.String("state", state.String()))
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	}
}
