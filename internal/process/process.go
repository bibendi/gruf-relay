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

	"github.com/bibendi/gruf-relay/internal/codec"
	"github.com/bibendi/gruf-relay/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Manager struct {
	servers   []Server
	mu        sync.Mutex
	Processes map[string]*Process
}

type Server struct {
	Name    string
	Command []string
	Port    int
}

type Process struct {
	Name     string
	Port     int
	cmd      *exec.Cmd
	mu       sync.Mutex
	running  bool
	stopping bool
	Client   *grpc.ClientConn
}

func NewManager(cfg *config.Config) (*Manager, error) {
	processes := make(map[string]*Process, cfg.Workers.Count)
	servers := make([]Server, cfg.Workers.Count)
	// Use a loop with index to correctly initialize the slices
	for i := 0; i < cfg.Workers.Count; i++ { // Corrected loop condition
		port := cfg.Workers.StartPort + i
		name := fmt.Sprintf("worker-%d", i+1)
		servers[i] = Server{
			Name:    name,
			Command: []string{"bundle", "exec", "gruf", "--host", fmt.Sprintf("0.0.0.0:%d", port), "--health-check", "--backtrace-on-error"},
			Port:    port,
		}

		process := &Process{
			Name:    name,
			Port:    port,
			cmd:     nil,
			Client:  nil,
			running: false,
		}
		processes[name] = process
	}
	return &Manager{servers: servers, Processes: processes}, nil
}

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
	cmd.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", server.Port))
	process.cmd = cmd
	cmd.Stdout = log.Writer()
	cmd.Stderr = log.Writer()

	log.Printf("Starting server %s on port %d with command: %v", process.Name, server.Port, server.Command)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start server %s: %w", process.Name, err)
	}

	process.running = true

	// Горутина для ожидания завершения процесса
	// TODO: restart process
	go func() {
		err := cmd.Wait()
		if process.stopping {
			return
		}
		if err != nil {
			log.Printf("Server %s exited unexpectedly with error: %v", process.Name, err)
		} else {
			log.Printf("Server %s exited normally", process.Name)
		}
		process.mu.Lock()
		process.running = false
		process.cmd = nil
		process.mu.Unlock()
	}()

	// Установление gRPC коннекта
	client, err := grpc.NewClient(fmt.Sprintf("0.0.0.0:%d", server.Port), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithCodec(codec.Codec()))
	if err != nil {
		log.Fatalf("Failed connect to backend: %v", err)
	}
	process.Client = client

	return nil
}

func (m *Manager) StopAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, process := range m.Processes {
		if err := m.stopProcess(process); err != nil {
			log.Printf("failed to stop server %s: %v", process.Name, err)
		}
	}
	return nil
}

func (m *Manager) stopProcess(process *Process) error {
	process.mu.Lock()
	defer process.mu.Unlock()

	if !process.running {
		return nil
	}

	if process.stopping {
		return fmt.Errorf("server %s is already stopping", process.Name)
	}
	process.stopping = true
	defer func() {
		process.stopping = false
	}()

	log.Printf("Stopping server %s", process.Name)

	if process.Client != nil {
		log.Printf("Closing client connection on %s", process.Name)
		if err := process.Client.Close(); err != nil {
			log.Printf("failed closing client connection on %s: %v", process.Name, err)
		}
	}

	// Проверяем, жив ли еще процесс
	if process.cmd.ProcessState != nil && process.cmd.ProcessState.Exited() {
		log.Printf("Server %s already exited, skipping stop", process.Name)
		process.running = false
		process.cmd = nil
		return nil // Процесс уже завершен, ничего не делаем
	}

	// Отправляем SIGTERM
	if err := process.cmd.Process.Signal(syscall.SIGTERM); err != nil {
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
			// Игнорируем ошибку wait: no child processes
			if !strings.Contains(err.Error(), "no child processes") {
				log.Printf("Server %s exited with error: %v", process.Name, err)
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

func (p *Process) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

func (m *Manager) GetServers() []Server {
	m.mu.Lock()
	defer m.mu.Unlock()

	serversCopy := make([]Server, len(m.servers))
	copy(serversCopy, m.servers)
	return serversCopy
}

func (m *Manager) IsServerRunning(serverName string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	process, ok := m.Processes[serverName]
	if !ok {
		return false
	}

	return process.IsRunning()
}
