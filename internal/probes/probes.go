package probes

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/bibendi/gruf-relay/internal/config"
	"github.com/bibendi/gruf-relay/internal/healthcheck"
	"github.com/bibendi/gruf-relay/internal/logger"
	"github.com/bibendi/gruf-relay/internal/manager"
	"google.golang.org/grpc/connectivity"
)

var log = logger.NewPackageLogger("package", "probes")

type Probes struct {
	ctx    context.Context
	wg     *sync.WaitGroup
	port   int
	server *http.Server
}

func NewProbes(ctx context.Context, wg *sync.WaitGroup, cfg *config.Probes, isStarted *atomic.Value, pm *manager.Manager, hc *healthcheck.Checker) *Probes {
	mux := http.NewServeMux()

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: mux,
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}

	probes := &Probes{
		ctx:    ctx,
		wg:     wg,
		port:   cfg.Port,
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
	log.Info("Probes server started", slog.Int("port", p.port))
}

func (p *Probes) waitCtxDone() {
	<-p.ctx.Done()
	log.Info("Stopping probes server")
	if err := p.server.Shutdown(context.Background()); err != nil {
		log.Error("Failed to shutdown probes server", slog.Any("err", err))
	}
}

func (p *Probes) run() {
	defer p.wg.Done()

	if err := p.server.ListenAndServe(); err != nil {
		if err != http.ErrServerClosed {
			log.Error("Probes server failed", slog.Any("err", err))
		}
	}
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
