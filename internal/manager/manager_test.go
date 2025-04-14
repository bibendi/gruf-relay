// internal/manager/manager_test.go
package manager

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/bibendi/gruf-relay/internal/config"
	"github.com/bibendi/gruf-relay/internal/process"
	mock_process "github.com/bibendi/gruf-relay/internal/process/mock"
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
		ctrl    *gomock.Controller
		ctx     context.Context
		wg      *sync.WaitGroup
		cfg     *config.Workers
		manager *Manager
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		ctx = context.Background()
		wg = &sync.WaitGroup{}
		cfg = &config.Workers{
			Count:       2,
			StartPort:   9000,
			MetricsPath: "/metrics",
		}
		manager = NewManager(ctx, wg, cfg)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("NewManager", func() {
		It("should create a new manager with the correct number of processes", func() {
			Expect(manager).NotTo(BeNil())
			Expect(len(manager.Processes)).To(Equal(cfg.Count))
		})
	})

	Describe("StartAll", func() {
		It("starts all processes correctly", func() {
			process1 := mock_process.NewMockProcess(ctrl)
			process2 := mock_process.NewMockProcess(ctrl)
			process1.EXPECT().Start().Return(nil)
			process2.EXPECT().Start().Return(nil)

			manager.Processes = map[string]process.Process{
				"worker-1": process1,
				"worker-2": process2,
			}

			err := manager.StartAll()
			Expect(err).To(BeNil())
		})

		It("returns an error if one of the processes fails to start", func() {
			process1 := mock_process.NewMockProcess(ctrl)
			process2 := mock_process.NewMockProcess(ctrl)
			expectedError := errors.New("failed to start process")
			process1.EXPECT().Start().Return(expectedError)
			process2.EXPECT().Start().Return(nil).AnyTimes()

			manager.Processes = map[string]process.Process{
				"worker-1": process1,
				"worker-2": process2,
			}

			err := manager.StartAll()
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(expectedError))
		})
	})
})
