package probes

import (
	"context"
	"fmt"
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
		host      = "localhost"
		port      = 8080
		pb        *Probes
		ctx       context.Context
		cancel    context.CancelFunc
		cfg       config.Probes
		isStarted *atomic.Value
		m         *MockManager
		hc        *MockHealthChecker
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		isStarted = &atomic.Value{}
		isStarted.Store(true)
		cfg = config.Probes{Port: port}
		m = NewMockManager(ctrl)
		m.EXPECT().GetWorkerNames().Return([]string{"worker-a"}).AnyTimes()
		hc = NewMockHealthChecker(ctrl)

		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)

		DeferCleanup(func() {
			cancel()
			ctrl.Finish()
		})
	})

	JustBeforeEach(func() {
		pb = NewProbes(cfg, isStarted, m, hc)
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
			startupURL := fmt.Sprintf("http://%s:%d/startup", host, port)
			go pb.Serve(ctx)

			// Test success case when app is started
			err := waitForProbe(startupURL, 3*time.Second)
			Expect(err).NotTo(HaveOccurred())

			// Test failure case when app is not started
			isStarted.Store(false)
			err = waitForProbe(startupURL, 100*time.Millisecond)
			Expect(err).To(HaveOccurred())
		})

		It("responds on /readiness request", func() {
			readinessURL := fmt.Sprintf("http://%s:%d/readiness", host, port)
			go pb.Serve(ctx)

			hc.EXPECT().GetServerState("worker-a").Return(connectivity.Ready)
			err := waitForProbe(readinessURL, 3*time.Second)
			Expect(err).NotTo(HaveOccurred())

			downStates := []connectivity.State{
				connectivity.TransientFailure,
				connectivity.Shutdown,
			}
			for _, state := range downStates {
				hc.EXPECT().GetServerState("worker-a").Return(state)
				err = waitForProbe(readinessURL, 100*time.Millisecond)
				Expect(err).To(HaveOccurred())
			}
		})

		It("responds on /liveness request", func() {
			livenessURL := fmt.Sprintf("http://%s:%d/liveness", host, port)
			go pb.Serve(ctx)

			hc.EXPECT().GetServerState("worker-a").Return(connectivity.Ready)
			err := waitForProbe(livenessURL, 3*time.Second)
			Expect(err).NotTo(HaveOccurred())

			hc.EXPECT().GetServerState("worker-a").Return(connectivity.TransientFailure)
			err = waitForProbe(livenessURL, 100*time.Millisecond)
			Expect(err).NotTo(HaveOccurred())

			hc.EXPECT().GetServerState("worker-a").Return(connectivity.Shutdown)
			err = waitForProbe(livenessURL, 100*time.Millisecond)
			Expect(err).To(HaveOccurred())
		})
	})
})

func waitForProbe(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			lastErr = fmt.Errorf("status: %d", resp.StatusCode)
		} else {
			lastErr = err
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("probe %s not ready: %v", url, lastErr)
}
