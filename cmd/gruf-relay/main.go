package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bibendi/gruf-relay/internal/config"
	"github.com/bibendi/gruf-relay/internal/healthcheck"
	"github.com/bibendi/gruf-relay/internal/process"
	// "github.com/bibendi/gruf-relay/internal/loadbalance"
	// "github.com/bibendi/gruf-relay/internal/proxy"
)

const shutdownTimeout = 3 * time.Second // Время на завершение работы

func main() {
	log.Println("Starting Gruf Relay...")

	// 1. Загрузка конфигурации
	cfg, err := config.LoadConfig("config.yaml") // Или из переменных окружения
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Printf("Configuration loaded: %+v", cfg) // Выводим конфигурацию для отладки

	// 2. Инициализация Process Manager
	pm, err := process.NewManager(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize process manager: %v", err)
	}
	log.Println("Process manager initialized")

	// 3. Запуск Ruby серверов
	if err := pm.StartAll(); err != nil {
		log.Fatalf("Failed to start ruby servers: %v", err)
	}
	log.Println("Ruby servers started")

	// 4. Инициализация Health Checker
	hc := healthcheck.NewChecker(pm, cfg)
	log.Println("Health checker initialized")

	// 5. Запуск Health Checker
	hc.Start()
	log.Println("Health checker started")

	// // 6. Инициализация Load Balancer
	// lb := loadbalance.NewRoundRobin(pm) // Можно выбрать другой алгоритм
	// log.Println("Load balancer initialized (Round Robin)")

	// // 7. Инициализация GRPC Proxy Server
	// grpcProxy := proxy.NewGRPCProxyServer(lb)
	// log.Println("GRPC proxy server initialized")

	// // 8. Запуск GRPC сервера
	// grpcServerCtx, grpcServerCancel := context.WithCancel(context.Background())
	// go func() {
	// 	log.Printf("Starting GRPC server on port %d", cfg.ProxyPort)
	// 	if err := proxy.StartGRPCServer(grpcServerCtx, cfg.ProxyPort, grpcProxy); err != nil {
	// 		log.Fatalf("Failed to start GRPC server: %v", err)
	// 	}
	// 	log.Println("GRPC server stopped") // This will be printed when the GRPC server exits
	// }()

	// 9. Обработка сигналов завершения (Ctrl+C)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutdown signal received...")

	// // 10. Graceful shutdown
	// log.Println("Shutting down GRPC server...")
	// grpcServerCancel()          // Signal the GRPC server to stop
	time.Sleep(shutdownTimeout) // Give the server time to shutdown

	log.Println("Stopping health checker...")
	hc.Stop()

	log.Println("Stopping ruby servers...")
	if err := pm.StopAll(); err != nil {
		log.Printf("Error stopping ruby servers: %v", err) // Use Printf instead of Fatalf
	}

	log.Println("Gruf Relay stopped")
}
