package main

import (
	"context"
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
	"github.com/bibendi/gruf-relay/internal/logger"
	"github.com/bibendi/gruf-relay/internal/manager"
	"github.com/bibendi/gruf-relay/internal/metrics"
	"github.com/bibendi/gruf-relay/internal/probes"
	"github.com/bibendi/gruf-relay/internal/proxy"
	"github.com/bibendi/gruf-relay/internal/server"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig("config/gruf-relay.yml")
	if err != nil {
		panic(fmt.Sprintf("Failed to load config: %s", err))
	}

	// Initialize Logger
	log := logger.NewLogger(cfg.LogLevel, cfg.LogFormat)
	slog.SetDefault(log)

	log.Info("Starting gRPC Relay")
	log.Debug("Configuration loaded")
	log.Info("Logger initialized", slog.String("level", cfg.LogLevel), slog.String("format", cfg.LogFormat))

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	// Initialize probes
	isStarted := &atomic.Value{}
	isStarted.Store(false)

	// Initialize Process Manager
	pm := manager.NewManager(ctx, &wg, &cfg.Workers)

	// Initialize gRPC processes
	if err := pm.StartAll(); err != nil {
		log.Error("Failed to start servers", slog.Any("error", err))
		cancel()
		os.Exit(1)
	}

	// Start Load Balancer
	lb := loadbalance.NewRandomBalancer()
	go func() {
		defer wg.Done()
		lb.Start(ctx)
	}()

	// Initialize Health Checker
	hc := healthcheck.NewChecker(ctx, &wg, pm.Processes, cfg, lb)
	hc.Start()

	if cfg.Probes.Enabled {
		probes := probes.NewProbes(ctx, &wg, &cfg.Probes, isStarted, pm, hc)
		probes.Start()
	}

	if cfg.Metrics.Enabled {
		metrics, err := metrics.NewScraper(ctx, &wg, pm, &cfg.Metrics)
		if err != nil {
			log.Error("Failed to create scraper", slog.Any("error", err))
			cancel()
			os.Exit(1)
		}
		go metrics.Start()
	}

	// Initialize gRPC Proxy
	grpcProxy := proxy.NewProxy(lb)

	// Initialize gRPC server
	grpcServer := server.NewServer(ctx, cfg, grpcProxy)

	go handleSignals(grpcServer, log)
	isStarted.Store(true)

	if err := grpcServer.Serve(); err != nil {
		log.Error("Failed to serve gRPC server", slog.Any("error", err))
	}

	cancel()
	wg.Wait()
	log.Info("Goodbye!")
}

func handleSignals(server *server.Server, log *slog.Logger) {
	// Graceful shutdown
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	<-signalCh
	log.Info("Received termination signal, initiating graceful shutdown...")
	server.Shoutdown()
}
