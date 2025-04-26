package loadbalance

import (
	"context"
	"log/slog"
	"math/rand"
	"sync"
	"sync/atomic"

	"slices"

	"github.com/bibendi/gruf-relay/internal/log"
	"github.com/bibendi/gruf-relay/internal/worker"
)

type RandomBalancer struct {
	addChan     chan worker.Worker
	removeChan  chan worker.Worker
	workers     atomic.Value
	workerNames map[string]bool
	mu          sync.Mutex
}

func NewRandomBalancer() *RandomBalancer {
	rb := &RandomBalancer{
		addChan:     make(chan worker.Worker),
		removeChan:  make(chan worker.Worker),
		workerNames: make(map[string]bool),
	}
	rb.workers.Store([]worker.Worker{})
	return rb
}

func (rb *RandomBalancer) Run(ctx context.Context) {
	log.Info("Starting load balancer")

	for {
		select {
		case w := <-rb.addChan:
			rb.onAddWorker(w)
		case w := <-rb.removeChan:
			rb.onRemoveWorker(w)
		case <-ctx.Done():
			log.Info("Stopping load balancer")
			return
		}
	}
}

func (rb *RandomBalancer) AddWorker(w worker.Worker) {
	rb.addChan <- w
}

func (rb *RandomBalancer) RemoveWorker(w worker.Worker) {
	rb.removeChan <- w
}

func (rb *RandomBalancer) Next() worker.Worker {
	workers := rb.workers.Load().([]worker.Worker)
	if len(workers) == 0 {
		return nil
	}

	index := rand.Intn(len(workers))
	return workers[index]
}

func (rb *RandomBalancer) onAddWorker(w worker.Worker) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if _, ok := rb.workerNames[w.String()]; ok {
		return
	}
	log.Debug("Adding worker to load balancer", slog.Any("worker", w))
	currentWorkers := rb.workers.Load().([]worker.Worker)
	rb.workers.Store(append(currentWorkers, w))
	rb.workerNames[w.String()] = true
}

func (rb *RandomBalancer) onRemoveWorker(w worker.Worker) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if _, ok := rb.workerNames[w.String()]; !ok {
		return
	}
	log.Debug("Removing worker from load balancer", slog.Any("worker", w))
	currentWorkers := rb.workers.Load().([]worker.Worker)
	var newWorkers []worker.Worker
	for i, cw := range currentWorkers {
		if cw.String() == w.String() {
			newWorkers = slices.Delete(currentWorkers, i, i+1)
			break
		}
	}
	delete(rb.workerNames, w.String())
	rb.workers.Store(newWorkers)
}
