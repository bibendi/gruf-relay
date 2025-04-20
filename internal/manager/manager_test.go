package manager

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bibendi/gruf-relay/internal/config"
	"github.com/bibendi/gruf-relay/internal/process"
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
		It("should create a new manager with the correct number of processes", func() {
			manager := NewManager(workersCfg)
			Expect(manager).NotTo(BeNil())
			Expect(len(manager.Processes)).To(Equal(2))
		})
	})

	Describe("Run", func() {
		It("runs all processes correctly", func() {
			ctx, _ := context.WithTimeout(context.Background(), 1*time.Second)
			process1 := process.NewMockProcess(ctrl)
			process2 := process.NewMockProcess(ctrl)
			process1.EXPECT().Run(gomock.Any()).Return(nil)
			process2.EXPECT().Run(gomock.Any()).Return(nil)

			manager := NewManager(workersCfg)
			manager.Processes = map[string]process.Process{
				"worker-1": process1,
				"worker-2": process2,
			}

			err := manager.Run(ctx)
			Expect(err).To(BeNil())
		})

		It("returns an error if one of the processes fails to start", func() {
			ctx := context.Background()
			process1 := process.NewMockProcess(ctrl)
			process2 := process.NewMockProcess(ctrl)
			expectedError := errors.New("failed to start process")
			process1.EXPECT().Run(gomock.Any()).Return(expectedError)
			process1.EXPECT().String().Return("worker-1").AnyTimes()
			process2.EXPECT().Run(gomock.Any()).Return(nil).AnyTimes()
			process2.EXPECT().String().Return("worker-2").AnyTimes()

			manager := NewManager(workersCfg)
			manager.Processes = map[string]process.Process{
				"worker-1": process1,
				"worker-2": process2,
			}

			err := manager.Run(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(expectedError))
		})
	})
})
