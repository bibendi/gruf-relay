package loadbalance

import (
	"context"
	"log/slog"
	"math/rand"
	"sync"
	"sync/atomic"

	"slices"

	"github.com/bibendi/gruf-relay/internal/process"
)

type RandomBalancer struct {
	addChan      chan *process.Process
	removeChan   chan *process.Process
	processes    atomic.Value
	processNames map[string]bool
	done         chan struct{}
	mu           sync.Mutex
	ctx          context.Context
	wg           *sync.WaitGroup
}

func NewRandomBalancer(ctx context.Context, wg *sync.WaitGroup) *RandomBalancer {
	rb := &RandomBalancer{
		addChan:      make(chan *process.Process),
		removeChan:   make(chan *process.Process),
		done:         make(chan struct{}),
		processNames: make(map[string]bool),
		ctx:          ctx,
		wg:           wg,
	}
	rb.processes.Store([]*process.Process{})
	return rb
}

func (rb *RandomBalancer) Start() {
	rb.wg.Add(1)
	go rb.waitCtxDone()
	go rb.balance()
	slog.Info("Load balancer started")
}

func (rb *RandomBalancer) AddProcess(p *process.Process) {
	rb.addChan <- p
}

func (rb *RandomBalancer) RemoveProcess(p *process.Process) {
	rb.removeChan <- p
}

func (rb *RandomBalancer) Next() *process.Process {
	processes := rb.processes.Load().([]*process.Process)
	if len(processes) == 0 {
		return nil
	}

	index := rand.Intn(len(processes))
	return processes[index]
}

func (rb *RandomBalancer) waitCtxDone() {
	<-rb.ctx.Done()
	close(rb.done)
}

func (rb *RandomBalancer) balance() {
	for {
		select {
		case p := <-rb.addChan:
			rb.mu.Lock()
			if _, ok := rb.processNames[p.Name]; ok {
				rb.mu.Unlock()
				continue
			}
			currentProcesses := rb.processes.Load().([]*process.Process)
			rb.processes.Store(append(currentProcesses, p))
			rb.processNames[p.Name] = true
			rb.mu.Unlock()
		case p := <-rb.removeChan:
			rb.mu.Lock()
			if _, ok := rb.processNames[p.Name]; !ok {
				rb.mu.Unlock()
				continue
			}
			currentProcesses := rb.processes.Load().([]*process.Process)
			var newProcesses []*process.Process
			for i, cp := range currentProcesses {
				if cp.Name == p.Name {
					newProcesses = slices.Delete(currentProcesses, i, i+1)
					break
				}
			}
			delete(rb.processNames, p.Name)
			rb.processes.Store(newProcesses)

			rb.mu.Unlock()
		case <-rb.done:
			slog.Info("Stopping load balancer")
			rb.wg.Done()
			return
		}
	}
}
