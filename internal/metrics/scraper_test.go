package metrics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bibendi/gruf-relay/internal/config"
	"github.com/bibendi/gruf-relay/internal/process"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	dto "github.com/prometheus/client_model/go"
	"go.uber.org/mock/gomock"
)

func TestScraper(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Scraper Suite")
}

var _ = Describe("Scraper", func() {
	var (
		ctrl    *gomock.Controller
		pm      *MockManager
		cfg     config.Metrics
		scraper *Scraper
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		pm = NewMockManager(ctrl)
		cfg = config.Metrics{
			Interval: time.Second,
			Port:     8080,
			Path:     "/metrics",
			Enabled:  true,
		}

		DeferCleanup(func() {
			ctrl.Finish()
		})
	})

	Describe("NewScraper", func() {
		It("should create a new scraper with the given configuration", func() {
			scraper = NewScraper(cfg, pm)
			Expect(scraper).NotTo(BeNil())
			Expect(scraper.pm).To(Equal(pm))
			Expect(scraper.interval).To(Equal(cfg.Interval))
			Expect(scraper.port).To(Equal(cfg.Port))
			Expect(scraper.path).To(Equal(cfg.Path))
			Expect(scraper.client).NotTo(BeNil())
			Expect(scraper.collector).NotTo(BeNil())
		})

		It("should create a http client with timeout", func() {
			scraper = NewScraper(cfg, pm)
			Expect(scraper.client.Timeout).To(Equal(10 * time.Second))
		})
	})

	Describe("Serve", func() {
		var (
			ctx    context.Context
			cancel context.CancelFunc
		)
		BeforeEach(func() {
			scraper = NewScraper(cfg, pm)
			ctx, cancel = context.WithCancel(context.Background())
			DeferCleanup(func() {
				cancel()
			})
		})

		It("should serve metrics until context is cancelled", func() {
			go func() {
				<-time.After(100 * time.Millisecond)
				cancel()
			}()
			go scraper.Serve(ctx)
			Eventually(ctx.Done()).Should(BeClosed())
		})

		It("should handle server listen error", func() {
			cfg.Port = -1 // Provoke an error
			scraper = NewScraper(cfg, pm)
			ctx, cancel = context.WithCancel(context.Background())
			defer cancel()
			err := scraper.Serve(ctx)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("scrapeAndAggregate", func() {
		var (
			worker1, worker2 *process.MockProcess
			ts               *httptest.Server
		)

		BeforeEach(func() {
			scraper = NewScraper(cfg, pm)
			worker1 = process.NewMockProcess(ctrl)
			worker2 = process.NewMockProcess(ctrl)

			ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				_, _ = w.Write([]byte(`# HELP test_metric Test metric
# TYPE test_metric counter
test_metric{label="value"} 10
`))
			}))

			pm.EXPECT().GetWorkers().Return(map[string]process.Process{
				"worker1": worker1,
				"worker2": worker2,
			}).AnyTimes()
			worker1.EXPECT().IsRunning().Return(true).AnyTimes()
			worker2.EXPECT().IsRunning().Return(true).AnyTimes()
			worker1.EXPECT().MetricsAddr().Return(ts.URL[7:]).AnyTimes()
			worker2.EXPECT().MetricsAddr().Return(ts.URL[7:]).AnyTimes()
		})

		AfterEach(func() {
			ts.Close()
			scraper.collector.metrics = make(map[string]*dto.MetricFamily)
		})

		It("Should scrap all workers", func() {
			scraper.scrapeAndAggregate()
			Expect(len(scraper.collector.metrics)).To(Equal(1))
		})
	})

	Describe("scrapeMetrics", func() {
		It("Should return err when request failed", func() {
			scraper = NewScraper(cfg, pm)
			_, err := scraper.scrapeMetrics("http://invalid-url")
			Expect(err).To(HaveOccurred())
		})

		It("Should return err when status code is not ok", func() {
			scraper = NewScraper(cfg, pm)
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer ts.Close()
			_, err := scraper.scrapeMetrics(ts.URL)
			Expect(err).To(HaveOccurred())
		})

		It("Should return metrics", func() {
			scraper = NewScraper(cfg, pm)
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				_, _ = w.Write([]byte(`# HELP test_metric Test metric
# TYPE test_metric counter
test_metric{label="value"} 10
`))
			}))
			defer ts.Close()

			metrics, err := scraper.scrapeMetrics(ts.URL)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(metrics)).To(Equal(1))
			Expect(*metrics[0].Name).To(Equal("test_metric"))
		})
	})
})
