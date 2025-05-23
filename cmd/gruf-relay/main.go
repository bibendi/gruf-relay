package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/bibendi/gruf-relay/internal/config"
	"github.com/bibendi/gruf-relay/internal/healthcheck"
	"github.com/bibendi/gruf-relay/internal/loadbalance"
	"github.com/bibendi/gruf-relay/internal/log"
	"github.com/bibendi/gruf-relay/internal/manager"
	"github.com/bibendi/gruf-relay/internal/metrics"
	"github.com/bibendi/gruf-relay/internal/probes"
	"github.com/bibendi/gruf-relay/internal/proxy"
	"github.com/bibendi/gruf-relay/internal/server"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = ""

	printVersion = flag.Bool("v", false, "Print version and exit")
)

func main() {
	flag.Parse()

	if *printVersion {
		fmt.Printf("gruf-relay\nversion: %s\ncommit: %s\nbuilt date: %s\n", version, commit, buildDate)
		os.Exit(0)
	}

	log.Info("Starting gRPC Relay")

	cfg := config.DefaultConfig()
	log.Debug("Configuration loaded")
	log.Info("Logger initialized", "level", cfg.Log.Level)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	// Initialize readiness probe
	isStarted := &atomic.Value{}
	isStarted.Store(false)

	// Run Worker Manager
	m := manager.NewManager(cfg.Workers)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := m.Run(ctx); err != nil {
			log.Error("Failed to start servers", slog.Any("error", err))
			cancel()
		}
	}()

	// Run Load Balancer
	lb := loadbalance.NewLoadBalancer()
	wg.Add(1)
	go func() {
		defer wg.Done()
		lb.Run(ctx)
	}()

	// Run Health Checker
	hc := healthcheck.NewChecker(cfg.HealthCheck, m.GetWorkers(), lb, nil)
	wg.Add(1)
	go func() {
		defer wg.Done()
		hc.Run(ctx)
	}()

	// Run probes
	if cfg.Probes.Enabled {
		probes := probes.NewProbes(cfg.Probes, isStarted, m, hc)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := probes.Serve(ctx); err != nil {
				log.Error("Failed to serve probes", slog.Any("error", err))
				cancel()
			}
		}()
	}

	// Run metrics
	if cfg.Metrics.Enabled {
		metrics := metrics.NewScraper(cfg.Metrics, m)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := metrics.Serve(ctx); err != nil {
				log.Error("Failed to serve metrics", slog.Any("error", err))
				cancel()
			}
		}()
	}

	// Run gRPC server
	grpcProxy := proxy.NewProxy(lb, cfg.Server.ProxyTimeout)
	grpcServer := server.NewServer(cfg.Server, grpcProxy)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := grpcServer.Serve(ctx); err != nil {
			log.Error("Failed to serve gRPC requests", slog.Any("error", err))
			cancel()
		}
	}()

	// Ready to work!
	isStarted.Store(true)

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	exitCode := 0
	select {
	case <-ctx.Done():
		log.Error("Shutdown initiated by context", slog.Any("error", ctx.Err()))
		exitCode = 1
	case sig := <-signalCh:
		log.Info("Received termination signal, initiating graceful shutdown...", slog.Any("signal", sig))
		cancel()
	}

	wg.Wait()

	log.Info("Goodbye!")
	os.Exit(exitCode)
}
