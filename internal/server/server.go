package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"github.com/bibendi/gruf-relay/internal/codec"
	"github.com/bibendi/gruf-relay/internal/config"
	"github.com/bibendi/gruf-relay/internal/proxy"
	"google.golang.org/grpc"
)

type Server struct {
	host       string
	port       int
	grpcServer *grpc.Server
	ctx        context.Context
}

type ServiceHandler func(srv interface{}, stream grpc.ServerStream) error

func NewServer(ctx context.Context, cfg *config.Config, proxy *proxy.Proxy) *Server {
	server := grpc.NewServer(
		grpc.CustomCodec(codec.Codec()),
		grpc.UnknownServiceHandler(proxy.HandleRequest),
	)

	return &Server{
		host:       cfg.Host,
		port:       cfg.Port,
		grpcServer: server,
		ctx:        ctx,
	}
}

func (s *Server) Serve() error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	slog.Info("Starting gRPC server", slog.String("addr", addr))

	if err := s.grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("gRPC server has failed: %v", err)
	}

	return nil
}

func (s *Server) Shoutdown() {
	slog.Info("Stopping gRPC server")
	s.grpcServer.GracefulStop()
}
