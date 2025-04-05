package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/bibendi/gruf-relay/internal/config"
	"github.com/bibendi/gruf-relay/internal/healthcheck"
	"github.com/bibendi/gruf-relay/internal/loadbalance"
	"github.com/bibendi/gruf-relay/internal/manager"
	"github.com/bibendi/gruf-relay/internal/proxy"
	"github.com/bibendi/gruf-relay/internal/server"
)

// TODO:
// - slog
// - Testify
// - Metrics
// - k8s probes
// - Coverage
func main() {
	log.Println("Starting gRPC Gruf Relay")

	// Load configuration
	cfg, err := config.LoadConfig("config/gruf-relay.yml")
	if err != nil {
		panic(fmt.Sprintf("Failed to load config: %v", err))
	}

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	// Initialize Process Manager
	pm := manager.NewManager(ctx, &wg, cfg)

	// Initialize gRPC processes
	if err := pm.StartAll(); err != nil {
		panic(fmt.Sprintf("Failed to start ruby servers: %v", err))
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
	grpcServer.Start()

	// Graceful shutdown
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	<-signalCh
	log.Println("Received termination signal, initiating graceful shutdown...")
	cancel()
	wg.Wait()
	log.Println("Goodbye!")
}
