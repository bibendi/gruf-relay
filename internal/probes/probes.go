package probes

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync/atomic"

	"github.com/bibendi/gruf-relay/internal/config"
	"github.com/bibendi/gruf-relay/internal/healthcheck"
	"github.com/bibendi/gruf-relay/internal/log"
	"github.com/bibendi/gruf-relay/internal/manager"
	"google.golang.org/grpc/connectivity"
)

type Probes struct {
	port         int
	appIsStarted *atomic.Value
	pm           *manager.Manager
	hc           *healthcheck.Checker
}

func NewProbes(cfg config.Probes, isStarted *atomic.Value, pm *manager.Manager, hc *healthcheck.Checker) *Probes {
	probes := &Probes{
		port:         cfg.Port,
		pm:           pm,
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
	mux.HandleFunc("/readiness", p.handleReadinessProbe(p.pm, p.hc))
	mux.HandleFunc("/liveness", p.handleLivenessrobe(p.pm, p.hc))

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

func (p *Probes) handleReadinessProbe(pm *manager.Manager, hc *healthcheck.Checker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Info("Received readiness request")
		for name, process := range pm.Processes {
			if state := hc.GetServerState(name); state == connectivity.TransientFailure || state == connectivity.Shutdown {
				log.Error("Readiness probe failed", slog.Any("process", process), slog.Any("state", state))
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	}
}

func (p *Probes) handleLivenessrobe(pm *manager.Manager, hc *healthcheck.Checker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Info("Received liveness request")
		for name, process := range pm.Processes {
			if state := hc.GetServerState(name); state == connectivity.Shutdown {
				log.Error("Liveness probe failed", slog.Any("process", process), slog.Any("state", state))
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	}
}
