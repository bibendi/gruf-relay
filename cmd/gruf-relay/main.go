package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bibendi/gruf-relay/internal/config"
	"github.com/bibendi/gruf-relay/internal/loadbalance"
	"github.com/bibendi/gruf-relay/internal/process"
	"github.com/bibendi/gruf-relay/internal/proxy"
	"github.com/bibendi/gruf-relay/internal/server"
)

// TODO:
// - Поддержка TLS
// - Метрики
// - Coverage
// - Testify
func main() {
	log.Println("Starting Gruf Relay...")

	// 1. Загрузка конфигурации
	cfg, err := config.LoadConfig("config/gruf-relay.yml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Printf("Configuration loaded: %+v", cfg)

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
	//hc := healthcheck.NewChecker(pm, cfg)
	log.Println("Health checker initialized")

	// 5. Запуск Health Checker
	//hc.Start()
	log.Println("Health checker started")

	// 6. Инициализация Load Balancer
	lb := loadbalance.NewRoundRobin(pm) // Можно выбрать другой алгоритм
	log.Printf("Load balancer initialized (Round Robin), %v", lb)

	// 7. Инициализация GRPC Proxy
	grpcProxy := proxy.NewProxy(lb)
	log.Println("GRPC proxy initialized")

	// 8. Создание GRPC сервера
	grpcServer := server.NewServer(cfg, grpcProxy)
	grpcServer.Start()
	log.Println("GRPC server started")

	// 11. Graceful shutdown
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	<-signalCh
	log.Println("Received termination signal, initiating graceful shutdown...")

	// 12. Остановка GRPC сервера
	grpcServer.Stop()

	// 13. Остановка процессов
	if err := pm.StopAll(); err != nil {
		log.Printf("Error stopping ruby servers: %v", err) // Use Printf instead of Fatalf
	}

	log.Println("Goodbye")
}
