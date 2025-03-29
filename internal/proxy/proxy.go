package proxy

import (
	"context"
	"io"
	"log"
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
	log.Printf("Hangle gRPC request method name: %s", fullMethod)

	timeoutCtx, cancel := context.WithTimeout(ctx, 1000*time.Second)
	defer cancel()

	md, _ := metadata.FromIncomingContext(ctx)
	outCtx := metadata.NewOutgoingContext(timeoutCtx, md.Copy())
	log.Printf("Metadata: %+v\n", md)

	process := p.Balancer.Next()
	if process == nil {
		return status.Error(codes.Unavailable, "server unavailable")
	}

	downstreamCtx, downstreamCancel := context.WithCancel(outCtx)
	defer downstreamCancel()

	// TODO: Should the process establish gRPC connection by itself?
	downstream, err := grpc.NewClientStream(downstreamCtx, downstreamDescForProxying, process.Client, fullMethod)
	if err != nil {
		return status.Errorf(codes.Unavailable, "failed creating downstream: %v", err)
	}

	log.Printf("Start proxying %s to %s", fullMethod, process.Name)

	upstreamErrChan := proxyRequest(upstream, downstream)
	downstreamErrChan := proxyResponse(downstream, upstream)

	for {
		select {
		case upstreamErr, ok := <-upstreamErrChan:
			if !ok {
				upstreamErr = nil
				continue
			}

			if upstreamErr == io.EOF {
				downstream.CloseSend()
			} else {
				return status.Errorf(codes.Internal, "failed proxying request: %v", upstreamErr)
			}
		case downstreamErr, ok := <-downstreamErrChan:
			if !ok {
				downstreamErr = nil
				continue
			}

			upstream.SetTrailer(downstream.Trailer())

			if downstreamErr == io.EOF {
				log.Printf("Finish proxying %s to %s", fullMethod, process.Name)
				return nil
			} else {
				log.Printf("failed proxying response: %v", downstreamErr)
				return downstreamErr
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
