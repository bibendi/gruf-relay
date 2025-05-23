package loadbalance

import (
	"context"
	"testing"
	"time"

	"github.com/bibendi/gruf-relay/internal/worker"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

func TestHealthCheck(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Load Balancer Suite")
}

var _ = Describe("LoadBalance", func() {
	var (
		ctrl *gomock.Controller
		lb   *LoadBalancer
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		DeferCleanup(func() {
			ctrl.Finish()
		})
	})

	JustBeforeEach(func() {
		lb = NewLoadBalancer()
	})

	Describe("NewLoadBalancer", func() {
		It("should create a new random balancer", func() {
			Expect(lb).NotTo(BeNil())
		})
	})

	Describe("Run", func() {
		var (
			ctx    context.Context
			cancel context.CancelFunc
			wrk    *worker.MockWorker
		)

		BeforeEach(func() {
			wrk = worker.NewMockWorker(ctrl)
			wrk.EXPECT().String().Return("worker-foo").AnyTimes()

			ctx, cancel = context.WithCancel(context.Background())
			DeferCleanup(func() {
				cancel()
			})
		})

		It("should stop waiting on context done", func() {
			go func() {
				<-time.After(100 * time.Millisecond)
				cancel()
			}()
			go lb.Run(ctx)
			Eventually(ctx.Done()).Should(BeClosed())
		})

		It("adds and removes worker", func() {
			go lb.Run(ctx)
			lb.AddWorker(wrk)
			time.Sleep(10 * time.Millisecond)
			nextWorker := lb.Next()
			Expect(nextWorker).NotTo(BeNil())
			Expect(nextWorker).To(Equal(wrk))
			lb.RemoveWorker(wrk)
			time.Sleep(10 * time.Millisecond)
			nextWorker = lb.Next()
			Expect(nextWorker).To(BeNil())
		})
	})
})
