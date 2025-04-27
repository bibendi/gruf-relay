package loadbalance

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"

	"slices"

	"github.com/bibendi/gruf-relay/internal/log"
	"github.com/bibendi/gruf-relay/internal/worker"
)

type LoadBalancer struct {
	addChan     chan worker.Worker
	removeChan  chan worker.Worker
	workers     atomic.Value
	workerNames map[string]bool
	mu          sync.Mutex
	nextIndex   uint64
}

func NewLoadBalancer() *LoadBalancer {
	lb := &LoadBalancer{
		addChan:     make(chan worker.Worker),
		removeChan:  make(chan worker.Worker),
		workerNames: make(map[string]bool),
	}
	lb.workers.Store([]worker.Worker{})
	return lb
}

func (lb *LoadBalancer) Run(ctx context.Context) {
	log.Info("Starting load balancer")

	for {
		select {
		case w := <-lb.addChan:
			lb.onAddWorker(w)
		case w := <-lb.removeChan:
			lb.onRemoveWorker(w)
		case <-ctx.Done():
			log.Info("Stopping load balancer")
			return
		}
	}
}

func (lb *LoadBalancer) AddWorker(w worker.Worker) {
	lb.addChan <- w
}

func (lb *LoadBalancer) RemoveWorker(w worker.Worker) {
	lb.removeChan <- w
}

func (lb *LoadBalancer) Next() worker.Worker {
	workers := lb.workers.Load().([]worker.Worker)

	n := uint64(len(workers))
	if n == 0 {
		return nil
	}

	next := atomic.AddUint64(&lb.nextIndex, 1) % n
	return workers[next]
}

func (lb *LoadBalancer) onAddWorker(w worker.Worker) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if _, ok := lb.workerNames[w.String()]; ok {
		return
	}
	log.Debug("Adding worker to load balancer", slog.Any("worker", w))
	currentWorkers := lb.workers.Load().([]worker.Worker)
	lb.workers.Store(append(currentWorkers, w))
	lb.workerNames[w.String()] = true
}

func (lb *LoadBalancer) onRemoveWorker(w worker.Worker) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if _, ok := lb.workerNames[w.String()]; !ok {
		return
	}
	log.Debug("Removing worker from load balancer", slog.Any("worker", w))
	currentWorkers := lb.workers.Load().([]worker.Worker)
	var newWorkers []worker.Worker
	for i, cw := range currentWorkers {
		if cw.String() == w.String() {
			newWorkers = slices.Delete(currentWorkers, i, i+1)
			break
		}
	}
	delete(lb.workerNames, w.String())
	lb.workers.Store(newWorkers)
}
