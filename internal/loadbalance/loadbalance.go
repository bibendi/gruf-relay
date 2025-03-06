// internal/loadbalance/loadbalance.go
package loadbalance

import (
	"sync"

	"github.com/bibendi/gruf-relay/internal/process"
)

// Balancer определяет интерфейс для алгоритмов балансировки нагрузки.
type Balancer interface {
	Next() *process.Process // Выбирает следующий доступный процесс.  Возвращает nil, если нет доступных процессов.
}

// RoundRobin реализует алгоритм Round Robin.
type RoundRobin struct {
	processes      map[string]*process.Process
	mu             sync.Mutex
	nextIndex      int
	processManager *process.Manager // Добавлено для получения списка процессов и проверки их состояния
}

// NewRoundRobin создает новый RoundRobin balancer.
func NewRoundRobin(pm *process.Manager) *RoundRobin {
	return &RoundRobin{
		processes:      pm.Processes,
		processManager: pm,
		nextIndex:      0,
	}
}

// Next выбирает следующий процесс, используя алгоритм Round Robin.
// Возвращает nil, если нет доступных процессов.
func (rr *RoundRobin) Next() *process.Process {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	availableProcesses := rr.getAvailableProcesses() // Фильтруем только доступные процессы
	if len(availableProcesses) == 0 {
		return nil // Нет доступных процессов
	}

	index := rr.nextIndex % len(availableProcesses)
	proc := availableProcesses[index]
	rr.nextIndex++

	return proc
}

// getAvailableProcesses возвращает слайс процессов, которые считаются "доступными".
// Сейчас это просто означает, что процесс запущен.
func (rr *RoundRobin) getAvailableProcesses() []*process.Process {
	available := []*process.Process{}
	for _, p := range rr.processManager.Processes {
		if p.IsRunning() { // Используем метод IsRunning, чтобы проверить состояние процесса
			available = append(available, p)
		}
	}
	return available
}
