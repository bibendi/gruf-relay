package probes

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bibendi/gruf-relay/internal/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/connectivity"
)

func TestHealthCheck(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Probes Suite")
}

var _ = Describe("Probes", func() {
	var (
		ctrl      *gomock.Controller
		pb        *Probes
		ctx       context.Context
		cancel    context.CancelFunc
		cfg       config.Probes
		isStarted *atomic.Value
		pm        *MockManager
		hc        *MockHealthChecker
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		isStarted = &atomic.Value{}
		isStarted.Store(true)
		cfg = config.Probes{Port: 6014}
		pm = NewMockManager(ctrl)
		pm.EXPECT().GetWorkerNames().Return([]string{"worker-a"}).AnyTimes()
		hc = NewMockHealthChecker(ctrl)

		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)

		DeferCleanup(func() {
			cancel()
			ctrl.Finish()
		})
	})

	JustBeforeEach(func() {
		pb = NewProbes(cfg, isStarted, pm, hc)
	})

	Describe("NewProbes", func() {
		It("should create a new health checker", func() {
			Expect(pb).NotTo(BeNil())
		})
	})

	Describe("Serve", func() {
		It("should stop serving on context done", func() {
			go func() {
				<-time.After(100 * time.Millisecond)
				cancel()
			}()
			go pb.Serve(ctx)
			Eventually(ctx.Done()).Should(BeClosed())
		})

		Context("when serve with error", func() {
			BeforeEach(func() {
				cfg = config.Probes{Port: 99999999}
			})

			It("returns error when cannot serve", func() {
				err := pb.Serve(ctx)
				Expect(err).To(HaveOccurred())
			})
		})

		It("responds on /startup request", func() {
			go pb.Serve(ctx)
			time.Sleep(100 * time.Millisecond)

			// Test success case when app is started
			resp, err := http.Get("http://localhost:6014/startup")
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			resp.Body.Close()

			// Test failure case when app is not started
			isStarted.Store(false)
			resp, err = http.Get("http://localhost:6014/startup")
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusServiceUnavailable))
			resp.Body.Close()
		})

		It("responds on /readiness request", func() {
			go pb.Serve(ctx)
			time.Sleep(100 * time.Millisecond)

			hc.EXPECT().GetServerState("worker-a").Return(connectivity.Ready)
			resp, err := http.Get("http://localhost:6014/readiness")
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			resp.Body.Close()

			downStates := []connectivity.State{
				connectivity.TransientFailure,
				connectivity.Shutdown,
			}
			for _, state := range downStates {
				hc.EXPECT().GetServerState("worker-a").Return(state)
				resp, err := http.Get("http://localhost:6014/readiness")
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusServiceUnavailable))
				resp.Body.Close()
			}
		})

		It("responds on /liveness request", func() {
			go pb.Serve(ctx)
			time.Sleep(100 * time.Millisecond)

			okStates := []connectivity.State{
				connectivity.Ready,
				connectivity.TransientFailure,
			}
			for _, state := range okStates {
				hc.EXPECT().GetServerState("worker-a").Return(state)
				resp, err := http.Get("http://localhost:6014/liveness")
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				resp.Body.Close()
			}

			hc.EXPECT().GetServerState("worker-a").Return(connectivity.Shutdown)
			resp, err := http.Get("http://localhost:6014/liveness")
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusServiceUnavailable))
			resp.Body.Close()
		})
	})
})
