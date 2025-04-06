package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/bibendi/gruf-relay/internal/config"
	"github.com/bibendi/gruf-relay/internal/healthcheck"
	"github.com/bibendi/gruf-relay/internal/loadbalance"
	"github.com/bibendi/gruf-relay/internal/logger"
	"github.com/bibendi/gruf-relay/internal/manager"
	"github.com/bibendi/gruf-relay/internal/proxy"
	"github.com/bibendi/gruf-relay/internal/server"
)

// TODO:
// - Testify
// - Metrics
// - k8s probes
// - Coverage
func main() {
	log.Println("Starting gRPC Gruf Relay")

	// Load configuration
	cfg, err := config.LoadConfig("config/gruf-relay.yml")
	if err != nil {
		slog.Error("Failed to load config", slog.Any("error", err))
		os.Exit(1)
	}

	// Initialize Logger
	logger.InitLogger(cfg.LogLevel, cfg.LogFormat)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	// Initialize Process Manager
	pm := manager.NewManager(ctx, &wg, cfg)

	// Initialize gRPC processes
	if err := pm.StartAll(); err != nil {
		slog.Error("Failed to start servers", slog.Any("error", err))
		os.Exit(1)
	}

	// Initialize Load Balancer
	lb := loadbalance.NewRandomBalancer(ctx, &wg)
	lb.Start()

	// Initialize Health Checker
	hc := healthcheck.NewChecker(ctx, &wg, pm, cfg, lb)
	hc.Start()

	// Initialize gRPC Proxy
	grpcProxy := proxy.NewProxy(lb)

	// Initialize gRPC server
	grpcServer := server.NewServer(ctx, cfg, grpcProxy)

	go waitTermination(grpcServer)

	if err := grpcServer.Serve(); err != nil {
		slog.Error("Failed to serve gRPC server", slog.Any("error", err))
	}

	cancel()
	wg.Wait()
	log.Println("Goodbye!")
}

func waitTermination(server *server.Server) {
	// Graceful shutdown
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	<-signalCh
	slog.Info("Received termination signal, initiating graceful shutdown...")
	server.Shoutdown()
}
