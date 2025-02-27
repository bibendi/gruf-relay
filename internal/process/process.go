// internal/process/process.go
package process

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall" // Добавлен импорт syscall
	"time"

	"github.com/bibendi/gruf-relay/internal/config"
)

// Manager управляет жизненным циклом Ruby GRPC серверов.
type Manager struct {
	servers []Server
	mu      sync.Mutex // Защита от гонок при доступе к servers
}

// Server представляет собой один Ruby GRPC сервер.
type Server struct {
	Name    string
	Command []string
	Port    int // Added Port
	process *exec.Cmd
}

// NewManager создает новый экземпляр Process Manager.
func NewManager(cfg *config.Config) (*Manager, error) {
	servers := make([]Server, cfg.Workers.Count)
	for i := range cfg.Workers.Count {
		port := cfg.Workers.StartPort + i
		servers[i] = Server{
			Name:    fmt.Sprintf("worker-%d", i+1), // Unique name
			Command: []string{"bundle", "exec", "gruf", "--host", fmt.Sprintf("0.0.0.0:%d", port), "--health-check"},
			Port:    port, // Assign the port
			process: nil,  // Пока процесс не запущен
		}
	}
	return &Manager{servers: servers}, nil
}

// StartAll запускает все Ruby GRPC серверы.
func (m *Manager) StartAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.servers {
		if err := m.startServer(&m.servers[i]); err != nil {
			return fmt.Errorf("failed to start server %s: %w", m.servers[i].Name, err)
		}
	}
	return nil
}

// startServer запускает один Ruby GRPC сервер.
func (m *Manager) startServer(server *Server) error {
	if server.process != nil {
		return fmt.Errorf("server %s is already running", server.Name)
	}

	cmd := exec.Command(server.Command[0], server.Command[1:]...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", server.Port)) // Add PORT env variable
	server.process = cmd

	// Перенаправляем вывод процесса в лог
	cmd.Stdout = log.Writer()
	cmd.Stderr = log.Writer()

	log.Printf("Starting server %s on port %d with command: %v", server.Name, server.Port, server.Command) // Added port logging
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start server %s: %w", server.Name, err)
	}

	// Горутина для ожидания завершения процесса
	go func() {
		err := cmd.Wait()
		if err != nil {
			log.Printf("Server %s exited with error: %v", server.Name, err)
		} else {
			log.Printf("Server %s exited normally", server.Name)
		}
		m.mu.Lock()
		server.process = nil // Сбрасываем process после завершения
		m.mu.Unlock()
	}()

	return nil
}

// StopAll останавливает все Ruby GRPC серверы.
func (m *Manager) StopAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.servers {
		if err := m.stopServer(&m.servers[i]); err != nil {
			log.Printf("failed to stop server %s: %v", m.servers[i].Name, err) // Log the error instead of returning
		}
	}
	return nil
}

// stopServer останавливает один Ruby GRPC сервер.
func (m *Manager) stopServer(server *Server) error {
	if server.process == nil {
		return nil // Процесс не начинался, ничего не делаем
	}

	log.Printf("Stopping server %s", server.Name)

	// Проверяем, жив ли еще процесс
	if server.process.ProcessState != nil && server.process.ProcessState.Exited() {
		log.Printf("Server %s already exited, skipping stop", server.Name)
		server.process = nil
		return nil // Процесс уже завершен, ничего не делаем
	}

	// Отправляем SIGTERM
	if err := server.process.Process.Signal(syscall.SIGTERM); err != nil { // Изменено: отправляем SIGTERM
		log.Printf("Failed to send SIGTERM to server %s: %v", server.Name, err)
		// Если не удалось отправить SIGTERM, отправляем SIGKILL
		if err := server.process.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill server %s: %w", server.Name, err)
		}
	}

	// Wait for the process to exit (with a timeout)
	waitChan := make(chan error, 1)
	go func() {
		waitChan <- server.process.Wait()
	}()

	select {
	case err := <-waitChan:
		if err != nil {
			log.Printf("Server %s exited with error: %v", server.Name, err)
			// Игнорируем ошибку wait: no child processes
			if !strings.Contains(err.Error(), "no child processes") {
				// Если это другая ошибка, возвращаем ее
				return err
			}
		} else {
			log.Printf("Server %s exited normally after SIGTERM", server.Name)
		}
	case <-time.After(5 * time.Second): // Timeout after 5 seconds
		log.Printf("Server %s did not exit after SIGTERM, sending SIGKILL", server.Name)
		if err := server.process.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill server %s: %w", server.Name, err)
		}
		err := <-waitChan // Wait for the process to exit after SIGKILL
		if err != nil {
			log.Printf("Server %s exited with error after SIGKILL: %v", server.Name, err)
		}
	}

	server.process = nil
	return nil
}

// IsRunning проверяет, запущен ли сервер.
func (m *Manager) IsRunning(serverName string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, server := range m.servers {
		if server.Name == serverName {
			return server.process != nil
		}
	}
	return false // Server not found
}

// GetServers returns a copy of the server list.
func (m *Manager) GetServers() []Server {
	m.mu.Lock()
	defer m.mu.Unlock()

	serversCopy := make([]Server, len(m.servers))
	for i := range m.servers {
		serversCopy[i] = m.servers[i]
	}
	return serversCopy
}
