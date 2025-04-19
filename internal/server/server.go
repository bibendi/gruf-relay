package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"github.com/bibendi/gruf-relay/internal/codec"
	"github.com/bibendi/gruf-relay/internal/config"
	"github.com/bibendi/gruf-relay/internal/log"
	"github.com/bibendi/gruf-relay/internal/proxy"
	"google.golang.org/grpc"
)

type Server struct {
	host  string
	port  int
	proxy *proxy.Proxy
}

type ServiceHandler func(srv interface{}, stream grpc.ServerStream) error

func NewServer(cfg config.Server, proxy *proxy.Proxy) *Server {
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

	server := grpc.NewServer(
		grpc.CustomCodec(codec.Codec()),
		grpc.UnknownServiceHandler(s.proxy.HandleRequest),
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
