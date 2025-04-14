package loadbalance

import (
	"context"
	"log/slog"
	"math/rand"
	"sync"
	"sync/atomic"

	"slices"

	"github.com/bibendi/gruf-relay/internal/logger"
	"github.com/bibendi/gruf-relay/internal/process"
)

var log = logger.NewPackageLogger("package", "loadbalance")

type RandomBalancer struct {
	addChan      chan process.Process
	removeChan   chan process.Process
	processes    atomic.Value
	processNames map[string]bool
	mu           sync.Mutex
}

func NewRandomBalancer() *RandomBalancer {
	rb := &RandomBalancer{
		addChan:      make(chan process.Process),
		removeChan:   make(chan process.Process),
		processNames: make(map[string]bool),
	}
	rb.processes.Store([]process.Process{})
	return rb
}

func (rb *RandomBalancer) Start(ctx context.Context) {
	log.Info("Starting load balancer")

	for {
		select {
		case p := <-rb.addChan:
			rb.onAddProcess(p)
		case p := <-rb.removeChan:
			rb.onRemoveProcess(p)
		case <-ctx.Done():
			log.Info("Stopping load balancer")
			return
		}
	}
}

func (rb *RandomBalancer) AddProcess(p process.Process) {
	rb.addChan <- p
}

func (rb *RandomBalancer) RemoveProcess(p process.Process) {
	rb.removeChan <- p
}

func (rb *RandomBalancer) Next() process.Process {
	processes := rb.processes.Load().([]process.Process)
	if len(processes) == 0 {
		return nil
	}

	index := rand.Intn(len(processes))
	return processes[index]
}

func (rb *RandomBalancer) onAddProcess(p process.Process) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if _, ok := rb.processNames[p.String()]; ok {
		return
	}
	log.Debug("Adding process to load balancer", slog.Any("process", p))
	currentProcesses := rb.processes.Load().([]process.Process)
	rb.processes.Store(append(currentProcesses, p))
	rb.processNames[p.String()] = true
}

func (rb *RandomBalancer) onRemoveProcess(p process.Process) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if _, ok := rb.processNames[p.String()]; !ok {
		return
	}
	log.Debug("Removing process from load balancer", slog.Any("process", p))
	currentProcesses := rb.processes.Load().([]process.Process)
	var newProcesses []process.Process
	for i, cp := range currentProcesses {
		if cp.String() == p.String() {
			newProcesses = slices.Delete(currentProcesses, i, i+1)
			break
		}
	}
	delete(rb.processNames, p.String())
	rb.processes.Store(newProcesses)
}
