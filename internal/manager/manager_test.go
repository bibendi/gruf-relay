package manager

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bibendi/gruf-relay/internal/config"
	"github.com/bibendi/gruf-relay/internal/worker"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

func TestManager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Manager Suite")
}

var _ = Describe("Manager", func() {
	var (
		ctrl       *gomock.Controller
		workersCfg config.Workers
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		workersCfg = config.Workers{
			Count:       2,
			StartPort:   9000,
			MetricsPath: "/metrics",
		}

		DeferCleanup(func() {
			ctrl.Finish()
		})
	})

	Describe("NewManager", func() {
		It("should create a new manager with the correct number of workers", func() {
			manager := NewManager(workersCfg)
			Expect(manager).NotTo(BeNil())
			Expect(len(manager.GetWorkers())).To(Equal(2))
		})
	})

	Describe("Run", func() {
		It("runs all workers correctly", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			worker1 := worker.NewMockWorker(ctrl)
			worker2 := worker.NewMockWorker(ctrl)
			worker1.EXPECT().Run(gomock.Any()).Return(nil)
			worker2.EXPECT().Run(gomock.Any()).Return(nil)

			manager := NewManager(workersCfg)
			manager.workers = map[string]worker.Worker{
				"worker-1": worker1,
				"worker-2": worker2,
			}

			err := manager.Run(ctx)
			Expect(err).To(BeNil())
		})

		It("returns an error if one of the workers fails to start", func() {
			ctx := context.Background()
			worker1 := worker.NewMockWorker(ctrl)
			worker2 := worker.NewMockWorker(ctrl)
			expectedError := errors.New("failed to start process")
			worker1.EXPECT().Run(gomock.Any()).Return(expectedError)
			worker1.EXPECT().String().Return("worker-1").AnyTimes()
			worker2.EXPECT().Run(gomock.Any()).Return(nil).AnyTimes()
			worker2.EXPECT().String().Return("worker-2").AnyTimes()

			manager := NewManager(workersCfg)
			manager.workers = map[string]worker.Worker{
				"worker-1": worker1,
				"worker-2": worker2,
			}

			err := manager.Run(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(expectedError))
		})
	})
})
