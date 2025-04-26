package healthcheck

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bibendi/gruf-relay/internal/config"
	"github.com/bibendi/gruf-relay/internal/worker"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/connectivity"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func TestHealthCheck(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "HealthCheck Suite")
}

var _ = Describe("HealthCheck", func() {
	var (
		ctrl *gomock.Controller
		cfg  config.HealthCheck
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		cfg = config.HealthCheck{
			Interval: 4 * time.Second,
		}

		DeferCleanup(func() {
			ctrl.Finish()
		})
	})

	Describe("NewChecker", func() {
		It("should create a new health checker", func() {
			workers := map[string]worker.Worker{
				"worker-1": worker.NewMockWorker(ctrl),
				"worker-2": worker.NewMockWorker(ctrl),
			}
			lb := NewMockBalancer(ctrl)
			checker := NewChecker(cfg, workers, lb, nil)
			Expect(checker).NotTo(BeNil())
		})
	})

	Describe("Run", func() {
		It("runs health checking", func() {
			ctx, _ := context.WithTimeout(context.Background(), 1*time.Second)
			workers := map[string]worker.Worker{
				"worker-1": worker.NewMockWorker(ctrl),
				"worker-2": worker.NewMockWorker(ctrl),
			}
			lb := NewMockBalancer(ctrl)
			checker := NewChecker(cfg, workers, lb, nil)
			Expect(func() {
				checker.Run(ctx)
			}).NotTo(Panic())
		})
	})

	Describe("checkAll", func() {
		var (
			lb            *MockBalancer
			workerA       *worker.MockWorker
			workers       map[string]worker.Worker
			checker       *Checker
			healthcheckFn HealthCheckFunc
		)

		BeforeEach(func() {
			workerA = worker.NewMockWorker(ctrl)
			workers = map[string]worker.Worker{"worker-a": workerA}
			workerA.EXPECT().String().Return("worker-a").AnyTimes()
			lb = NewMockBalancer(ctrl)
			healthcheckFn = func(ctx context.Context, w worker.Worker) (healthpb.HealthCheckResponse_ServingStatus, error) {
				return healthpb.HealthCheckResponse_SERVING, nil
			}
		})

		JustBeforeEach(func() {
			checker = NewChecker(cfg, workers, lb, healthcheckFn)
		})

		It("updates state to ready when worker is healthy", func() {
			workerA.EXPECT().IsRunning().Return(true)
			lb.EXPECT().AddWorker(workerA)
			checker.checkAll()
			Expect(checker.GetServerState(workerA.String())).To(Equal(connectivity.Ready))
		})

		It("updates state to shoutdown when worker is not running", func() {
			workerA.EXPECT().IsRunning().Return(false)
			lb.EXPECT().RemoveWorker(workerA)
			checker.checkAll()
			Expect(checker.GetServerState(workerA.String())).To(Equal(connectivity.Shutdown))
		})

		Context("when grpc error", func() {
			BeforeEach(func() {
				healthcheckFn = func(ctx context.Context, w worker.Worker) (healthpb.HealthCheckResponse_ServingStatus, error) {
					return healthpb.HealthCheckResponse_NOT_SERVING, fmt.Errorf("no connection")
				}
			})

			It("updates state to transient failure when no connection", func() {
				workerA.EXPECT().IsRunning().Return(true)
				lb.EXPECT().RemoveWorker(workerA)

				checker.checkAll()
				Expect(checker.GetServerState(workerA.String()).String()).To(Equal(connectivity.TransientFailure.String()))
			})
		})
	})
})
