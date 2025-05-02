//go:generate mockgen -source=server.go -destination=server_mock.go -package=server
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/bibendi/gruf-relay/internal/codec"
	"github.com/bibendi/gruf-relay/internal/config"
	"github.com/bibendi/gruf-relay/internal/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/keepalive"
)

type Proxy interface {
	HandleRequest(any, grpc.ServerStream) error
}

type Server struct {
	host  string
	port  int
	proxy Proxy
}

func NewServer(cfg config.Server, proxy Proxy) *Server {
	return &Server{
		host:  cfg.Host,
		port:  cfg.Port,
		proxy: proxy,
	}
}

func (s *Server) Serve(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	log.Info("Starting gRPC server", slog.String("addr", addr))

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	encoding.RegisterCodec(codec.Codec())

	server := grpc.NewServer(
		grpc.UnknownServiceHandler(s.proxy.HandleRequest),
		grpc.NumStreamWorkers(0),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     15 * time.Second, // If a client is idle for 15 seconds, send a GOAWAY
			MaxConnectionAge:      30 * time.Second, // If any connection is alive for more than 30 seconds, send a GOAWAY
			MaxConnectionAgeGrace: 5 * time.Second,  // Allow 5 seconds for pending RPCs to complete before forcibly closing connections
			Time:                  5 * time.Second,  // Ping the client if it is idle for 5 seconds to ensure the connection is still active
			Timeout:               1 * time.Second,  // Wait 1 second for the ping ack before assuming the connection is dead
		}),
	)

	errChan := make(chan error, 1)
	defer close(errChan)
	go func() {
		if err := server.Serve(lis); err != nil {
			errChan <- fmt.Errorf("gRPC server has failed: %v", err)
		}
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		log.Info("Stopping gRPC server")
		server.GracefulStop()
	}
	return nil
}
