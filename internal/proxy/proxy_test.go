package proxy

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"testing"
	"time"

	"github.com/bibendi/gruf-relay/internal/codec"
	"github.com/bibendi/gruf-relay/internal/worker"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestProxy(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Proxy Suite")
}

var _ = Describe("Proxy", func() {
	var (
		ctrl             *gomock.Controller
		mockBalancer     *MockBalancer
		proxy            *Proxy
		ctx              context.Context
		cancel           context.CancelFunc
		mockWorker       *worker.MockWorker
		mockServerStream *MockServerStream
		clientConn       *grpc.ClientConn
		pulledClient     *MockPulledClientConn
		lis              *bufconn.Listener
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockBalancer = NewMockBalancer(ctrl)
		proxy = NewProxy(mockBalancer, 2*time.Second)
		ctx, cancel = context.WithCancel(context.Background())
		mockWorker = worker.NewMockWorker(ctrl)
		mockServerStream = NewMockServerStream(ctrl)

		buffer := 1024
		lis = bufconn.Listen(buffer)
		dial := func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}

		encoding.RegisterCodec(codec.Codec())

		grpcServer := grpc.NewServer(
			grpc.UnknownServiceHandler(func(_ interface{}, stream grpc.ServerStream) error {
				log.Println("UnknownServiceHandler called")
				return nil
			}),
		)

		go func() {
			defer GinkgoRecover()
			if err := grpcServer.Serve(lis); err != nil {
				GinkgoT().Errorf("Server exited with error: %v", err)
			}
		}()

		conn, err := grpc.NewClient("passthrough:///bufnet",
			grpc.WithContextDialer(dial),
			grpc.WithTransportCredentials(insecure.NewCredentials()))
		Expect(err).To(BeNil())
		clientConn = conn
		pulledClient = NewMockPulledClientConn(ctrl)
		pulledClient.EXPECT().Conn().Return(clientConn).AnyTimes()
		pulledClient.EXPECT().Return().AnyTimes()

		md := metadata.Pairs("key", "value")
		newCtx := metadata.NewIncomingContext(ctx, md)
		methodCtx := grpc.NewContextWithServerTransportStream(newCtx, &testServerTransportStream{method: "/test.Service/Method"})

		mockServerStream.EXPECT().Context().Return(methodCtx).AnyTimes()

		DeferCleanup(func() {
			if clientConn != nil {
				clientConn.Close()
			}

			if grpcServer != nil {
				grpcServer.GracefulStop()
			}

			if lis != nil {
				lis.Close()
			}

			cancel()
			ctrl.Finish()
		})
	})

	Describe("NewProxy", func() {
		It("should create a new proxy with the given balancer", func() {
			Expect(proxy).NotTo(BeNil())
			Expect(proxy.Balancer).To(Equal(mockBalancer))
		})
	})

	Describe("HandleRequest", func() {
		It("should handle the request", func() {
			mockBalancer.EXPECT().Next().Return(mockWorker).Times(1)
			mockWorker.EXPECT().FetchClientConn(gomock.Any()).Return(pulledClient, nil).Times(1)
			mockServerStream.EXPECT().RecvMsg(gomock.Any()).Return(io.EOF).Times(1)
			mockServerStream.EXPECT().SetTrailer(gomock.Any()).Times(1)

			Expect(proxy.HandleRequest(nil, mockServerStream)).To(BeNil())
		})

		It("Return server unavailable when the balancer returns nil", func() {
			mockBalancer.EXPECT().Next().Return(nil).Times(1)
			err := proxy.HandleRequest(nil, mockServerStream)

			Expect(status.Code(err)).To(Equal(codes.Unavailable))
			Expect(err).ToNot(BeNil())
		})

		It("Return error when can't get client", func() {
			mockBalancer.EXPECT().Next().Return(mockWorker).Times(1)
			mockWorker.EXPECT().FetchClientConn(gomock.Any()).Return(nil, errors.New("Test error")).Times(1)

			err := proxy.HandleRequest(nil, mockServerStream)

			Expect(status.Code(err)).To(Equal(codes.Unavailable))
			Expect(err).ToNot(BeNil())
		})
	})
})

type testServerTransportStream struct {
	grpc.ServerTransportStream
	method string
}

func (t *testServerTransportStream) Method() string {
	return t.method
}
