// internal/process/process.go
package process

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bibendi/gruf-relay/internal/config"
)

// Manager управляет жизненным циклом Ruby GRPC серверов.
type Manager struct {
	servers   []Server
	mu        sync.Mutex          // Защита от гонок при доступе к servers
	Processes map[string]*Process // Слайс для хранения информации о запущенных процессах. Добавлено!
}

// Server представляет собой конфигурацию Ruby GRPC сервера (то, как мы его запускаем)
type Server struct {
	Name    string
	Command []string
	Port    int // Added Port
}

// Process представляет собой запущенный процесс Ruby GRPC сервера.
type Process struct { // Переименовано из SingleProcess в Process
	Name    string
	Port    int // Added Port
	cmd     *exec.Cmd
	mu      sync.Mutex
	running bool
}

// NewManager создает новый экземпляр Process Manager.
func NewManager(cfg *config.Config) (*Manager, error) {
	processes := make(map[string]*Process, cfg.Workers.Count)
	servers := make([]Server, cfg.Workers.Count)
	// Use a loop with index to correctly initialize the slices
	for i := 0; i < cfg.Workers.Count; i++ { // Corrected loop condition
		port := cfg.Workers.StartPort + i
		name := fmt.Sprintf("worker-%d", i+1)
		servers[i] = Server{
			Name:    name, // Unique name
			Command: []string{"bundle", "exec", "gruf", "--host", fmt.Sprintf("0.0.0.0:%d", port), "--health-check", "--backtrace-on-error"},
			Port:    port, // Assign the port
		}

		process := &Process{
			Name:    name,
			Port:    port,
			cmd:     nil,
			running: false,
		}
		processes[name] = process
	}
	return &Manager{servers: servers, Processes: processes}, nil
}

// StartAll запускает все Ruby GRPC серверы.
func (m *Manager) StartAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, server := range m.servers {
		if err := m.startProcess(m.Processes[server.Name], m.servers[i]); err != nil {
			return fmt.Errorf("failed to start server %s: %w", server.Name, err)
		}
	}
	return nil
}

// startProcess запускает один Ruby GRPC сервер.
func (m *Manager) startProcess(process *Process, server Server) error {
	process.mu.Lock()
	defer process.mu.Unlock()

	if process.running {
		return fmt.Errorf("server %s is already running", process.Name)
	}

	cmd := exec.Command(server.Command[0], server.Command[1:]...)
	if errors.Is(cmd.Err, exec.ErrDot) {
		cmd.Err = nil
	}
	cmd.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", server.Port)) // Add PORT env variable
	process.cmd = cmd
	// Перенаправляем вывод процесса в лог
	cmd.Stdout = log.Writer()
	cmd.Stderr = log.Writer()

	log.Printf("Starting server %s on port %d with command: %v", process.Name, server.Port, server.Command) // Added port logging
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start server %s: %w", process.Name, err)
	}

	process.running = true // устанавливаем флаг running в true

	// Горутина для ожидания завершения процесса
	go func() {
		err := cmd.Wait()
		if err != nil {
			log.Printf("Server %s exited with error: %v", process.Name, err)
		} else {
			log.Printf("Server %s exited normally", process.Name)
		}
		process.mu.Lock()
		process.running = false // устанавливаем флаг running в false при завершении
		process.cmd = nil       // Сбрасываем process после завершения
		process.mu.Unlock()
	}()

	return nil
}

// StopAll останавливает все Ruby GRPC серверы.
func (m *Manager) StopAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, process := range m.Processes {
		if err := m.stopProcess(process); err != nil {
			log.Printf("failed to stop server %s: %v", process.Name, err) // Log the error instead of returning
		}
	}
	return nil
}

// stopProcess останавливает один Ruby GRPC сервер.
func (m *Manager) stopProcess(process *Process) error {
	process.mu.Lock()
	defer process.mu.Unlock()

	if !process.running {
		return nil // Процесс не начинался, ничего не делаем
	}

	log.Printf("Stopping server %s", process.Name)

	// Проверяем, жив ли еще процесс
	if process.cmd.ProcessState != nil && process.cmd.ProcessState.Exited() {
		log.Printf("Server %s already exited, skipping stop", process.Name)
		process.running = false
		process.cmd = nil
		return nil // Процесс уже завершен, ничего не делаем
	}

	// Отправляем SIGTERM
	if err := process.cmd.Process.Signal(syscall.SIGTERM); err != nil { // Изменено: отправляем SIGTERM
		log.Printf("Failed to send SIGTERM to server %s: %v", process.Name, err)
		// Если не удалось отправить SIGTERM, отправляем SIGKILL
		if err := process.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill server %s: %w", process.Name, err)
		}
	}

	// Wait for the process to exit (with a timeout)
	waitChan := make(chan error, 1)
	go func() {
		waitChan <- process.cmd.Wait()
	}()

	select {
	case err := <-waitChan:
		if err != nil {
			log.Printf("Server %s exited with error: %v", process.Name, err)
			// Игнорируем ошибку wait: no child processes
			if !strings.Contains(err.Error(), "no child processes") {
				// Если это другая ошибка, возвращаем ее
				return err
			}
		} else {
			log.Printf("Server %s exited normally after SIGTERM", process.Name)
		}
	case <-time.After(5 * time.Second): // Timeout after 5 seconds
		log.Printf("Server %s did not exit after SIGTERM, sending SIGKILL", process.Name)
		if err := process.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill server %s: %w", process.Name, err)
		}
		err := <-waitChan // Wait for the process to exit after SIGKILL
		if err != nil {
			log.Printf("Server %s exited with error after SIGKILL: %v", process.Name, err)
		}
	}

	process.running = false
	process.cmd = nil
	return nil
}

// IsRunning проверяет, запущен ли процесс.
func (p *Process) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

// GetServers returns a copy of the server list.
func (m *Manager) GetServers() []Server {
	m.mu.Lock()
	defer m.mu.Unlock()

	serversCopy := make([]Server, len(m.servers))
	copy(serversCopy, m.servers)
	return serversCopy
}

// IsServerRunning проверяет, запущен ли конкретный сервер
func (m *Manager) IsServerRunning(serverName string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	process, ok := m.Processes[serverName]
	if !ok {
		return false
	}

	return process.IsRunning()
}
