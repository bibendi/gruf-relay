//go:generate mockgen -source=proxy.go -destination=proxy_mock.go -package proxy
//go:generate mockgen -destination=stream_mock.go -package proxy google.golang.org/grpc ServerStream
package proxy

import (
	"context"
	"io"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/bibendi/gruf-relay/internal/log"
	"github.com/bibendi/gruf-relay/internal/worker"
)

var (
	downstreamDescForProxying = &grpc.StreamDesc{
		ServerStreams: true,
		ClientStreams: true,
	}
)

type Balancer interface {
	Next() worker.Worker
}

type PulledClientConn interface {
	Conn() *grpc.ClientConn
	Return()
}

type Proxy struct {
	Balancer       Balancer
	requestTimeout time.Duration
}

func NewProxy(balancer Balancer, requestTimeout time.Duration) *Proxy {
	return &Proxy{
		Balancer:       balancer,
		requestTimeout: requestTimeout,
	}
}

func (p *Proxy) HandleRequest(srv any, upstream grpc.ServerStream) error {
	ctx := upstream.Context()

	fullMethod, ok := grpc.Method(ctx)
	if !ok {
		return status.Error(codes.Internal, "method unknown")
	}
	log.Info("Handle gRPC request", slog.String("method", fullMethod))

	worker := p.Balancer.Next()
	log.Debug("Selected worker", slog.Any("worker", worker))
	if worker == nil {
		return status.Error(codes.Unavailable, "server unavailable")
	}

	var client PulledClientConn
	var err error
	client, err = worker.FetchClientConn(ctx)
	if err != nil {
		return status.Errorf(codes.Unavailable, "failed getting grpc client connection: %v", err)
	}
	defer client.Return()

	timeoutCtx, cancel := context.WithTimeout(ctx, p.requestTimeout)
	defer cancel()

	md, _ := metadata.FromIncomingContext(ctx)
	outCtx := metadata.NewOutgoingContext(timeoutCtx, md.Copy())
	log.Debug("Request metadata", slog.Any("metadata", md))
	downstreamCtx, downstreamCancel := context.WithCancel(outCtx)
	defer downstreamCancel()

	downstream, err := grpc.NewClientStream(downstreamCtx, downstreamDescForProxying, client.Conn(), fullMethod)
	if err != nil {
		return status.Errorf(codes.Unavailable, "failed creating downstream: %v", err)
	}

	log.Info("Proxying request", slog.String("method", fullMethod), slog.Any("worker", worker))

	upstreamErrChan := proxyRequest(upstream, downstream)
	downstreamErrChan := proxyResponse(downstream, upstream)

	for {
		select {
		case err, ok := <-upstreamErrChan:
			if !ok {
				upstreamErrChan = nil
				continue
			}

			if err == io.EOF {
				if err := downstream.CloseSend(); err != nil {
					return status.Errorf(codes.Internal, "failed closing downstream: %v", err)
				}
			} else {
				return status.Errorf(codes.Internal, "failed proxying request: %v", err)
			}
		case err, ok := <-downstreamErrChan:
			if !ok {
				downstreamErrChan = nil
				continue
			}

			upstream.SetTrailer(downstream.Trailer())

			if err == io.EOF {
				log.Info("Finish proxying", slog.String("method", fullMethod), slog.Any("worker", worker))
				return nil
			} else {
				log.Error("Failed proxy response", slog.Any("worker", worker), slog.Any("error", err))
				return err
			}
		}
	}
}

func proxyRequest(src grpc.ServerStream, dst grpc.ClientStream) chan error {
	errChan := make(chan error, 1)

	go func() {
		defer close(errChan)

		var msg emptypb.Empty

		for {
			err := src.RecvMsg(&msg)
			if err != nil {
				errChan <- err
				return
			}

			err = dst.SendMsg(&msg)
			if err != nil {
				errChan <- err
				return
			}
		}
	}()

	return errChan
}

func proxyResponse(src grpc.ClientStream, dst grpc.ServerStream) chan error {
	errChan := make(chan error, 1)

	go func() {
		defer close(errChan)

		var msg emptypb.Empty

		// Receive the first message to ensure the connection is established
		// and the header is (or will soon be) available.
		if err := src.RecvMsg(&msg); err != nil {
			errChan <- err
			return
		}

		// Retrieve and forward the header from the client stream
		header, err := src.Header()
		if err != nil {
			errChan <- err
			return
		}
		if err := dst.SendHeader(header); err != nil {
			errChan <- err
			return
		}

		// Send the first message we already received.
		if err := dst.SendMsg(&msg); err != nil {
			errChan <- err
			return
		}

		// Copy the remaining message stream.
		for {
			if err := src.RecvMsg(&msg); err != nil {
				errChan <- err
				return
			}

			if err := dst.SendMsg(&msg); err != nil {
				errChan <- err
				return
			}
		}
	}()

	return errChan
}
