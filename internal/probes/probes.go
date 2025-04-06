package probes

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/bibendi/gruf-relay/internal/healthcheck"
	"github.com/bibendi/gruf-relay/internal/manager"
	"google.golang.org/grpc/connectivity"
)

type Probes struct {
	ctx    context.Context
	wg     *sync.WaitGroup
	port   int
	server *http.Server
}

func NewProbes(ctx context.Context, wg *sync.WaitGroup, port int, isStarted *atomic.Value, pm *manager.Manager, hc *healthcheck.Checker) *Probes {
	mux := http.NewServeMux()

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}

	probes := &Probes{
		ctx:    ctx,
		wg:     wg,
		port:   port,
		server: server,
	}

	mux.HandleFunc("/startup", probes.handleStartupProbe(isStarted))
	mux.HandleFunc("/readiness", probes.handleReadinessProbe(pm, hc))
	mux.HandleFunc("/liveness", probes.handleLivenessrobe(pm, hc))

	return probes
}

func (p *Probes) Start() {
	p.wg.Add(1)
	go p.waitCtxDone()
	go p.run()
	slog.Info("Probes server started", slog.Int("port", p.port))
}

func (p *Probes) waitCtxDone() {
	<-p.ctx.Done()
	slog.Info("Stopping probes server")
	if err := p.server.Shutdown(context.Background()); err != nil {
		slog.Error("Failed to shutdown probes server", slog.Any("err", err))
	}
}

func (p *Probes) run() {
	defer p.wg.Done()

	if err := p.server.ListenAndServe(); err != nil {
		if err != http.ErrServerClosed {
			slog.Error("Probes server failed", slog.Any("err", err))
		}
	}
}

func (p *Probes) handleStartupProbe(isStarted *atomic.Value) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slog.Info("Received startup request")
		if isStarted.Load() == false {
			slog.Error("Relay is not started yet, returning 503")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func (p *Probes) handleReadinessProbe(pm *manager.Manager, hc *healthcheck.Checker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slog.Info("Received readiness request")
		for name, process := range pm.Processes {
			if state := hc.GetServerState(name); state == connectivity.TransientFailure || state == connectivity.Shutdown {
				slog.Error("Readiness probe failed", slog.Any("process", process), slog.Any("state", state))
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	}
}

func (p *Probes) handleLivenessrobe(pm *manager.Manager, hc *healthcheck.Checker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slog.Info("Received liveness request")
		for name, process := range pm.Processes {
			if state := hc.GetServerState(name); state == connectivity.Shutdown {
				slog.Error("Liveness probe failed", slog.Any("process", process), slog.Any("state", state))
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	}
}
