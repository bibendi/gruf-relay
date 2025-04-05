package server

import (
	"context"
	"fmt"
	"log"
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

func (s *Server) Start() {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	log.Printf("Starting gRPC server on %s", addr)

	go func() {
		if err := s.grpcServer.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	go s.waitShoutdown()
}

func (s *Server) waitShoutdown() {
	<-s.ctx.Done()
	s.shoutdown()
}

func (s *Server) shoutdown() {
	log.Println("Stopping gRPC server")
	s.grpcServer.GracefulStop()
}
