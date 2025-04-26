package worker

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

func TestHealthCheck(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Worker Suite")
}

var _ = Describe("Worker", func() {
	var (
		ctrl *gomock.Controller
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		DeferCleanup(func() {
			ctrl.Finish()
		})
	})

	Describe("NewWorker", func() {
		It("should create a new worker instance", func() {
			w := NewWorker("worker-1", 50051, 9090, "/metrics")
			Expect(w).NotTo(BeNil())
			Expect(w.String()).To(Equal("worker-1"))
			Expect(w.Addr()).To(Equal(fmt.Sprintf("0.0.0.0:%d", 50051)))
			Expect(w.MetricsAddr()).To(Equal(fmt.Sprintf("0.0.0.0:%d%s", 9090, "/metrics")))
		})
	})

	Describe("Run", func() {
		var (
			worker       *workerImpl
			ctx          context.Context
			cancel       context.CancelFunc
			mockExecutor *MockCommandExecutor
			mockCommand  *MockCommand
		)

		BeforeEach(func() {
			ctx, cancel = context.WithCancel(context.Background())

			mockExecutor = NewMockCommandExecutor(ctrl)
			withExecutor := WithExecutor(mockExecutor)
			mockCommand = NewMockCommand(ctrl)
			mockExecutor.EXPECT().NewCommand(gomock.Any(), gomock.Any()).Return(mockCommand).AnyTimes()
			mockCommand.EXPECT().SetEnv(gomock.Any()).AnyTimes()

			worker = NewWorker("worker-1", 50051, 9090, "/metrics", withExecutor)

			DeferCleanup(func() {
				cancel()
			})
		})

		It("should start and shutdown on context done with clean exit", func() {
			wCtx, wCancel := context.WithCancel(context.Background())
			mockCommand.EXPECT().Start().Return(nil)
			mockCommand.EXPECT().Wait().DoAndReturn(func() error {
				<-wCtx.Done()
				return nil
			})
			mockCommand.EXPECT().Stop().DoAndReturn(func() error {
				wCancel()
				return nil
			})
			mockCommand.EXPECT().ProcessState().Return(nil).AnyTimes()

			go func() {
				time.Sleep(100 * time.Millisecond)
				cancel()
			}()

			err := worker.Run(ctx)
			Expect(err).NotTo(HaveOccurred())
			Eventually(worker.IsRunning()).Should(BeFalse())
		})

		It("should restart if the worker exits with an error", func() {
			firstRun := true
			mockCommand.EXPECT().Start().Return(nil).Times(2)
			mockCommand.EXPECT().Wait().DoAndReturn(func() error {
				if firstRun {
					firstRun = false
					return errors.New("test error")
				}
				go func() {
					<-ctx.Done()
				}()
				return nil
			}).Times(2)
			mockCommand.EXPECT().ProcessState().Return(nil).AnyTimes()

			go func() {
				time.Sleep(1500 * time.Millisecond)
				cancel()
			}()

			err := worker.Run(ctx)
			Expect(err).NotTo(HaveOccurred())
			Eventually(worker.IsRunning()).Should(BeFalse())
		})
	})
})
