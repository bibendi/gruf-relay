package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bibendi/gruf-relay/internal/config"
	"github.com/bibendi/gruf-relay/internal/healthcheck"
	"github.com/bibendi/gruf-relay/internal/loadbalance"
	"github.com/bibendi/gruf-relay/internal/manager"
	"github.com/bibendi/gruf-relay/internal/proxy"
	"github.com/bibendi/gruf-relay/internal/server"
)

// TODO:
// - cleanenv
// - slog
// - Testify
// - Metrics
// - Coverage
func main() {
	log.Println("Starting Gruf Relay...")

	// Load configuration
	cfg, err := config.LoadConfig("config/gruf-relay.yml")
	if err != nil {
		// FIXME: Exit with panic
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Printf("Configuration loaded: %+v", cfg)

	// Initialize Process Manager
	pm := manager.NewManager(cfg)
	log.Println("Process manager initialized")

	// Start Ruby servers
	if err := pm.StartAll(); err != nil {
		// FIXME: Exit with panic
		log.Fatalf("Failed to start ruby servers: %v", err)
	}
	log.Println("Ruby servers started")

	// Initialize Health Checker
	hc := healthcheck.NewChecker(pm, cfg)
	hc.Start()
	log.Println("Health checker started")

	// Initialize Load Balancer
	lb := loadbalance.NewRoundRobin(pm)
	log.Printf("Load balancer initialized, %v", lb)

	// Initialize gRPC Proxy
	grpcProxy := proxy.NewProxy(lb)
	log.Println("gRPC proxy initialized")

	// Create gRPC server
	grpcServer := server.NewServer(cfg, grpcProxy)
	grpcServer.Start()
	log.Println("gRPC server started")

	// Graceful shutdown
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	<-signalCh
	log.Println("Received termination signal, initiating graceful shutdown...")

	// Stop gRPC server
	grpcServer.Stop()

	// Stop processes
	if err := pm.StopAll(); err != nil {
		log.Printf("Error stopping ruby servers: %v", err)
	}

	log.Println("Goodbye")
}
