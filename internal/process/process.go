package process

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bibendi/gruf-relay/internal/codec"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Process struct {
	Name     string
	Addr     string
	Client   *grpc.ClientConn
	ctx      context.Context
	wg       *sync.WaitGroup
	cmd      *exec.Cmd
	mu       sync.Mutex
	running  bool
	stopping bool
	done     chan struct{}
}

func NewProcess(ctx context.Context, wg *sync.WaitGroup, name, addr string) *Process {
	wg.Add(1)
	return &Process{
		Name: name,
		Addr: addr,
		ctx:  ctx,
		wg:   wg,
	}
}

func (p *Process) String() string {
	return fmt.Sprintf("%s (%s)", p.Name, p.Addr)
}

func (p *Process) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return fmt.Errorf("server %s is already running", p)
	}

	p.done = make(chan struct{})

	cmd := p.buildCmd()
	p.cmd = cmd

	log.Printf("Starting server %s", p)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start server %s: %w", p, err)
	}

	// TODO: Establishing gRPC connection after the first successfull healthcheck.
	//       Stop the process after 5 failed healthchecks.
	time.Sleep(3 * time.Second)
	if err := p.initGrpcClient(); err != nil {
		return fmt.Errorf("failed to init gRPC client to server %s: %w", p, err)
	}

	p.running = true

	go p.waitCtxDone()
	go p.monitor()

	return nil
}

// TODO: refactor this method
func (p *Process) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	defer p.wg.Done()

	if !p.running {
		log.Printf("Trying to stop not running server %s", p)
		return nil
	}

	if p.stopping {
		return fmt.Errorf("server %s is already stopping", p)
	}
	p.stopping = true
	defer func() {
		p.stopping = false
	}()

	close(p.done)

	log.Printf("Stopping server %s", p)

	if p.Client != nil {
		log.Printf("Closing client connection to %s", p)
		if err := p.Client.Close(); err != nil {
			log.Printf("failed closing client connection to %s: %v", p, err)
		}
	}

	// Check if the process is still alive
	if p.cmd.ProcessState != nil && p.cmd.ProcessState.Exited() {
		log.Printf("Server %s already exited, skipping stop", p)
		p.running = false
		p.cmd = nil
		return nil // Process has already completed, do nothing
	}

	// Send SIGTERM
	if err := p.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		log.Printf("Failed to send SIGTERM to server %s: %v", p, err)
		// If sending SIGTERM fails, send SIGKILL
		if err := p.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill server %s: %w", p, err)
		}
	}

	// Wait for the process to exit (with a timeout)
	waitChan := make(chan error, 1)
	go func() {
		waitChan <- p.cmd.Wait()
	}()

	select {
	case err := <-waitChan:
		if err != nil {
			// Ignore "wait: no child processes" error
			if !strings.Contains(err.Error(), "no child processes") {
				log.Printf("Server %s exited with error: %v", p, err)
				return err
			}
		} else {
			log.Printf("Server %s exited normally after SIGTERM", p)
		}
	case <-time.After(5 * time.Second):
		log.Printf("Server %s did not exit after SIGTERM, sending SIGKILL", p)
		if err := p.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill server %s: %w", p, err)
		}

		err := <-waitChan // Wait for the process to exit after SIGKILL
		if err != nil {
			log.Printf("Server %s exited with error after SIGKILL: %v", p, err)
		}
	}

	p.running = false
	p.cmd = nil
	return nil
}

func (p *Process) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

func (p *Process) buildCmd() *exec.Cmd {
	args := p.cmdArgs()
	cmd := exec.Command(args[0], args[1:]...)
	// Allow to exec programs in the current directory
	if errors.Is(cmd.Err, exec.ErrDot) {
		cmd.Err = nil
	}

	cmd.Stdout = log.Writer()
	cmd.Stderr = log.Writer()
	return cmd
}

func (p *Process) cmdArgs() []string {
	return []string{"bundle", "exec", "gruf", "--host", p.Addr, "--health-check", "--backtrace-on-error"}
}

func (p *Process) initGrpcClient() error {
	client, err := grpc.NewClient(p.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithCodec(codec.Codec()))
	p.Client = client
	return err
}

func (p *Process) waitCtxDone() {
	select {
	case <-p.ctx.Done():
		p.Stop()
	case <-p.done:
		// do nothing
	}
}

// Goroutine to wait for the process to complete
func (p *Process) monitor() {
	if err := p.cmd.Wait(); err != nil {
		if p.stopping {
			return
		}
		log.Printf("Server %s exited unexpectedly with error: %v", p, err)
	} else {
		log.Printf("Server %s exited normally", p)
	}

	p.mu.Lock()
	p.running = false
	p.cmd = nil
	p.mu.Unlock()

	time.Sleep(2 * time.Second)
	p.Start()
}
