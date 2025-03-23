package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bibendi/gruf-relay/internal/config"
	"github.com/bibendi/gruf-relay/internal/healthcheck"
	"github.com/bibendi/gruf-relay/internal/loadbalance"
	"github.com/bibendi/gruf-relay/internal/process"
	"github.com/bibendi/gruf-relay/internal/proxy"
	"github.com/bibendi/gruf-relay/internal/server"
)

// TODO:
// - TLS Support
// - Metrics
// - Coverage
// - Testify
// - slog
// - cleanenv
func main() {
	log.Println("Starting Gruf Relay...")

	// 1. Load configuration
	cfg, err := config.LoadConfig("config/gruf-relay.yml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Printf("Configuration loaded: %+v", cfg)

	// 2. Initialize Process Manager
	pm, err := process.NewManager(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize process manager: %v", err)
	}
	log.Println("Process manager initialized")

	// 3. Start Ruby servers
	if err := pm.StartAll(); err != nil {
		log.Fatalf("Failed to start ruby servers: %v", err)
	}
	log.Println("Ruby servers started")

	// 4. Initialize Health Checker
	hc := healthcheck.NewChecker(pm, cfg)
	log.Println("Health checker initialized")

	// 5. Start Health Checker
	hc.Start()
	log.Println("Health checker started")

	// 6. Initialize Load Balancer
	lb := loadbalance.NewRoundRobin(pm) // You can select another algorithm
	log.Printf("Load balancer initialized (Round Robin), %v", lb)

	// 7. Initialize GRPC Proxy
	grpcProxy := proxy.NewProxy(lb)
	log.Println("GRPC proxy initialized")

	// 8. Create GRPC server
	grpcServer := server.NewServer(cfg, grpcProxy)
	grpcServer.Start()
	log.Println("GRPC server started")

	// 11. Graceful shutdown
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	<-signalCh
	log.Println("Received termination signal, initiating graceful shutdown...")

	// 12. Stop GRPC server
	grpcServer.Stop()

	// 13. Stop processes
	if err := pm.StopAll(); err != nil {
		log.Printf("Error stopping ruby servers: %v", err)
	}

	log.Println("Goodbye")
}
