package loadbalance

import (
	"sync"

	"github.com/bibendi/gruf-relay/internal/manager"
	"github.com/bibendi/gruf-relay/internal/process"
)

type Balancer interface {
	Next() *process.Process
}

type RoundRobin struct {
	processes map[string]*process.Process
	mu        sync.Mutex
	nextIndex int
	pm        *manager.Manager
}

func NewRoundRobin(pm *manager.Manager) *RoundRobin {
	return &RoundRobin{
		processes: pm.GetProcesses(),
		pm:        pm,
		nextIndex: 0,
	}
}

func (rr *RoundRobin) Next() *process.Process {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	availableProcesses := rr.getAvailableProcesses()
	if len(availableProcesses) == 0 {
		return nil
	}

	index := rr.nextIndex % len(availableProcesses)
	proc := availableProcesses[index]
	rr.nextIndex++

	return proc
}

func (rr *RoundRobin) getAvailableProcesses() []*process.Process {
	available := []*process.Process{}
	// FIXME: Remove access by mutex
	for _, p := range rr.pm.GetProcesses() {
		// FIXME: Check process health
		if p.IsRunning() {
			available = append(available, p)
		}
	}
	return available
}
