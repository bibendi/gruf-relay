package healthcheck

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bibendi/gruf-relay/internal/config"
	"github.com/bibendi/gruf-relay/internal/process"
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
			processes := map[string]process.Process{
				"worker-1": process.NewMockProcess(ctrl),
				"worker-2": process.NewMockProcess(ctrl),
			}
			lb := NewMockBalancer(ctrl)
			checker := NewChecker(cfg, processes, lb, nil)
			Expect(checker).NotTo(BeNil())
		})
	})

	Describe("Run", func() {
		It("runs health checking", func() {
			ctx, _ := context.WithTimeout(context.Background(), 1*time.Second)
			processes := map[string]process.Process{
				"worker-1": process.NewMockProcess(ctrl),
				"worker-2": process.NewMockProcess(ctrl),
			}
			lb := NewMockBalancer(ctrl)
			checker := NewChecker(cfg, processes, lb, nil)
			Expect(func() {
				checker.Run(ctx)
			}).NotTo(Panic())
		})
	})

	Describe("checkAll", func() {
		var (
			lb            *MockBalancer
			processA      *process.MockProcess
			processes     map[string]process.Process
			checker       *Checker
			healthcheckFn HealthCheckFunc
		)

		BeforeEach(func() {
			processA = process.NewMockProcess(ctrl)
			processes = map[string]process.Process{"worker-a": processA}
			processA.EXPECT().String().Return("worker-a").AnyTimes()
			lb = NewMockBalancer(ctrl)
			healthcheckFn = func(ctx context.Context, p process.Process) (healthpb.HealthCheckResponse_ServingStatus, error) {
				return healthpb.HealthCheckResponse_SERVING, nil
			}
		})

		JustBeforeEach(func() {
			checker = NewChecker(cfg, processes, lb, healthcheckFn)
		})

		It("updates state to ready when worker is healthy", func() {
			processA.EXPECT().IsRunning().Return(true)
			lb.EXPECT().AddProcess(processA)
			checker.checkAll()
			Expect(checker.GetServerState(processA.String())).To(Equal(connectivity.Ready))
		})

		It("updates state to shoutdown when worker is not running", func() {
			processA.EXPECT().IsRunning().Return(false)
			lb.EXPECT().RemoveProcess(processA)
			checker.checkAll()
			Expect(checker.GetServerState(processA.String())).To(Equal(connectivity.Shutdown))
		})

		Context("when grpc error", func() {
			BeforeEach(func() {
				healthcheckFn = func(ctx context.Context, p process.Process) (healthpb.HealthCheckResponse_ServingStatus, error) {
					return healthpb.HealthCheckResponse_NOT_SERVING, fmt.Errorf("no connection")
				}
			})

			It("updates state to transient failure when no connection", func() {
				processA.EXPECT().IsRunning().Return(true)
				lb.EXPECT().RemoveProcess(processA)

				checker.checkAll()
				Expect(checker.GetServerState(processA.String()).String()).To(Equal(connectivity.TransientFailure.String()))
			})
		})
	})
})
