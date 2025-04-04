package process

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/bibendi/gruf-relay/internal/codec"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Process struct {
	Name        string
	Addr        string
	Client      *grpc.ClientConn
	ctx         context.Context
	wg          *sync.WaitGroup
	cmd         *exec.Cmd
	mu          sync.Mutex
	running     bool
	stopping    bool
	cmdDoneChan chan error
	startOnce   sync.Once
}

func NewProcess(ctx context.Context, wg *sync.WaitGroup, name, addr string) *Process {
	wg.Add(1)
	return &Process{
		Name:        name,
		Addr:        addr,
		ctx:         ctx,
		wg:          wg,
		cmdDoneChan: make(chan error, 1),
		startOnce:   sync.Once{},
	}
}

func (p *Process) String() string {
	return fmt.Sprintf("%s (%s)", p.Name, p.Addr)
}

func (p *Process) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		log.Printf("Server %s is already running", p)
		return nil
	}

	if p.stopping {
		log.Printf("Server %s is already stopping", p)
		return nil
	}

	log.Printf("Starting server %s", p)
	p.cmd = p.buildCmd()
	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start server %s: %w", p, err)
	}

	// TODO: Establishing gRPC connection after the first successfull healthcheck.
	time.Sleep(3 * time.Second)
	if err := p.initGrpcClient(); err != nil {
		return fmt.Errorf("failed to init gRPC client to server %s: %w", p, err)
	}

	p.running = true

	p.startOnce.Do(func() {
		go p.waitShoutdown()
	})
	go p.waitCmdDone()

	return nil
}

func (p *Process) shoutdown() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	defer p.wg.Done()

	if !p.running {
		log.Printf("Server %s is not running", p)
		return nil
	}

	if p.stopping {
		log.Printf("Server %s is already stopping", p)
		return nil
	}

	log.Printf("Stopping server %s", p)
	p.stopping = true

	if p.Client != nil {
		log.Printf("Closing client connection to %s", p)
		if err := p.Client.Close(); err != nil {
			log.Printf("failed closing client connection to %s: %v", p, err)
		}
	}

	// Check if the process is still alive
	if p.cmd.ProcessState != nil && p.cmd.ProcessState.Exited() {
		log.Printf("Server %s already exited", p)
		return nil
	}

	// Send SIGTERM
	if err := p.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		log.Printf("Failed to send SIGTERM to server %s: %v", p, err)
		if err := p.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill server %s: %w", p, err)
		}
	}

	select {
	case <-p.cmdDoneChan:
		log.Printf("Server %s has stopped", p)
	case <-time.After(5 * time.Second):
		log.Printf("Timeout waiting for server %s to exit, sending SIGKILL", p)
		if err := p.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill server %s: %w", p, err)
		}
	}

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

func (p *Process) waitShoutdown() {
	<-p.ctx.Done()

	if err := p.shoutdown(); err != nil {
		log.Printf("Failed to shutdown server %s: %v", p, err)
	}
}

func (p *Process) waitCmdDone() {
	err := p.cmd.Wait()
	if err != nil {
		log.Printf("Server %s exited unexpectedly with error: %v", p, err)
	} else {
		log.Printf("Server %s exited normally", p)
	}

	if p.stopping {
		p.cmdDoneChan <- err
		close(p.cmdDoneChan)
		return
	}

	p.mu.Lock()
	p.running = false
	p.mu.Unlock()
	time.Sleep(2 * time.Second)

	if err := p.Start(); err != nil {
		log.Printf("Failed to restart server %s: %v", p, err)
	}
}
