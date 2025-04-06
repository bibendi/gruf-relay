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

	"github.com/bibendi/gruf-relay/internal/process"
)

var (
	downstreamDescForProxying = &grpc.StreamDesc{
		ServerStreams: true,
		ClientStreams: true,
	}
)

type Balancer interface {
	Next() *process.Process
}

type Proxy struct {
	Balancer Balancer
}

func NewProxy(balancer Balancer) *Proxy {
	return &Proxy{
		Balancer: balancer,
	}
}

func (p *Proxy) HandleRequest(srv any, upstream grpc.ServerStream) error {
	ctx := upstream.Context()

	fullMethod, ok := grpc.Method(ctx)
	if !ok {
		return status.Error(codes.Internal, "method unknown")
	}
	slog.Info("Handle gRPC request", slog.String("method", fullMethod))

	timeoutCtx, cancel := context.WithTimeout(ctx, 1000*time.Second)
	defer cancel()

	md, _ := metadata.FromIncomingContext(ctx)
	outCtx := metadata.NewOutgoingContext(timeoutCtx, md.Copy())
	slog.Debug("Request metadata", slog.Any("metadata", md))

	process := p.Balancer.Next()
	slog.Debug("Selected server", slog.Any("server", process))
	if process == nil {
		return status.Error(codes.Unavailable, "server unavailable")
	}

	downstreamCtx, downstreamCancel := context.WithCancel(outCtx)
	defer downstreamCancel()

	client, err := process.GetClient()
	if err != nil {
		return status.Errorf(codes.Unavailable, "failed getting grpc client: %v", err)
	}
	downstream, err := grpc.NewClientStream(downstreamCtx, downstreamDescForProxying, client, fullMethod)
	if err != nil {
		return status.Errorf(codes.Unavailable, "failed creating downstream: %v", err)
	}

	slog.Info("Proxying request", slog.String("method", fullMethod), slog.Any("server", process))

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
				downstream.CloseSend()
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
				slog.Info("Finish proxying", slog.String("method", fullMethod), slog.Any("server", process))
				return nil
			} else {
				slog.Error("Failed proxy response", slog.Any("server", process), slog.Any("error", err))
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
