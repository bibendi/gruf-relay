package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/bibendi/gruf-relay/internal/log"
	"google.golang.org/grpc"
)

type PulledClientConn interface {
	Conn() *grpc.ClientConn
	Return()
}

type clientConnBuilder func() (*grpc.ClientConn, error)

type connectionPool struct {
	connections []*grpc.ClientConn
	available   chan int
	mu          sync.Mutex
	log         log.Logger
	builder     clientConnBuilder
}

type pooledClientConn struct {
	conn  *grpc.ClientConn
	pool  *connectionPool
	index int
	log   log.Logger
}

func newConnectionPool(size int, logger log.Logger, builder clientConnBuilder) *connectionPool {
	pool := connectionPool{
		connections: make([]*grpc.ClientConn, size),
		available:   make(chan int, size),
		log:         logger,
		builder:     builder,
	}
	for i := range size {
		pool.available <- i
	}

	return &pool
}

func (cp *connectionPool) fetchConn(ctx context.Context) (*pooledClientConn, error) {
	var idx int
	select {
	case idx = <-cp.available:
		cp.log.Debug("Got connection from pool", slog.Int("index", idx))
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	if cp.connections[idx] != nil {
		return newPooledClientConn(idx, cp), nil
	}

	cp.mu.Lock()
	defer cp.mu.Unlock()

	if cp.connections[idx] == nil {
		client, err := cp.builder()
		if err != nil {
			cp.available <- idx
			return nil, fmt.Errorf("failed creating new gRPC client connection: %v", err)
		}
		cp.connections[idx] = client
	}

	return newPooledClientConn(idx, cp), nil
}

func (cp *connectionPool) close() {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	for i, conn := range cp.connections {
		if conn != nil {
			if err := conn.Close(); err != nil {
				cp.log.Error("Failed to close client connection", slog.Int("index", i), slog.Any("error", err))
			}
			cp.connections[i] = nil
		}
	}
}

func newPooledClientConn(idx int, cp *connectionPool) *pooledClientConn {
	return &pooledClientConn{
		conn:  cp.connections[idx],
		pool:  cp,
		index: idx,
		log:   cp.log,
	}
}

func (pcc *pooledClientConn) Conn() *grpc.ClientConn {
	return pcc.conn
}

func (pcc *pooledClientConn) Return() {
	pcc.log.Debug("Returning connection to pool", slog.Int("index", pcc.index))
	pcc.pool.available <- pcc.index
}
