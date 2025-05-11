package server

import (
	"context"
	"testing"
	"time"

	"github.com/bibendi/gruf-relay/internal/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

func TestServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Server Suite")
}

var _ = Describe("Server", func() {
	var (
		ctrl      *gomock.Controller
		mockProxy *MockProxy
		cfg       config.Server
		server    *Server
		ctx       context.Context
		cancel    context.CancelFunc
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockProxy = NewMockProxy(ctrl)
		cfg = config.Server{
			Host: "localhost",
			Port: 6024,
		}
		server = NewServer(cfg, mockProxy)
		ctx, cancel = context.WithCancel(context.Background())

		DeferCleanup(func() {
			ctrl.Finish()
			cancel()
		})
	})

	Describe("NewServer", func() {
		It("should create a new server with the given configuration", func() {
			Expect(server).NotTo(BeNil())
			Expect(server.host).To(Equal(cfg.Host))
			Expect(server.port).To(Equal(cfg.Port))
			Expect(server.proxy).To(Equal(mockProxy))
		})
	})

	Describe("Serve", func() {
		It("should serve gRPC requests until context is cancelled", func() {
			go func() {
				defer GinkgoRecover()
				err := server.Serve(ctx)
				Expect(err).To(BeNil())
			}()

			go func() {
				<-time.After(100 * time.Millisecond)
				cancel()
			}()

			Eventually(ctx.Done()).Should(BeClosed())
		})

		It("should handle listen error", func() {
			cfg.Port = -1 // Provoke error
			server = NewServer(cfg, mockProxy)

			err := server.Serve(ctx)
			Expect(err).To(HaveOccurred())
		})
	})
})
