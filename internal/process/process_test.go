package process

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

func TestHealthCheck(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Process Suite")
}

var _ = Describe("Process", func() {
	var (
		ctrl *gomock.Controller
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		DeferCleanup(func() {
			ctrl.Finish()
		})
	})

	Describe("NewProcess", func() {
		It("should create a new process instance", func() {
			p := NewProcess("worker-1", 50051, 9090, "/metrics")
			Expect(p).NotTo(BeNil())
			Expect(p.String()).To(Equal("worker-1"))
			Expect(p.Addr()).To(Equal(fmt.Sprintf("0.0.0.0:%d", 50051)))
			Expect(p.MetricsAddr()).To(Equal(fmt.Sprintf("0.0.0.0:%d%s", 9090, "/metrics")))
		})
	})

	Describe("Run", func() {
		var (
			process      *processImpl
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
			mockExecutor.EXPECT().NewCommand(gomock.Any(), gomock.Any()).Return(mockCommand)
			mockCommand.EXPECT().SetEnv(gomock.Any())

			process = NewProcess("worker-1", 50051, 9090, "/metrics", withExecutor)

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
			mockCommand.EXPECT().ProcessState().Return(nil)

			go func() {
				time.Sleep(100 * time.Millisecond)
				cancel()
			}()

			err := process.Run(ctx)
			Expect(err).NotTo(HaveOccurred())
			Eventually(process.IsRunning()).Should(BeFalse())
		})
	})
})
