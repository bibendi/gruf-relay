package metrics

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
)

var _ = Describe("AggregatedCollector", func() {
	var (
		collector *aggregatedCollector
		registry  *prometheus.Registry
		metrics   map[string]*dto.MetricFamily
	)

	BeforeEach(func() {
		registry = prometheus.NewRegistry()
		collector = newAggregatedCollector()
		metrics = make(map[string]*dto.MetricFamily)
	})

	Describe("newAggregatedCollector", func() {
		It("should create a new aggregated collector", func() {
			Expect(collector).NotTo(BeNil())
			Expect(collector.metrics).To(BeEmpty())
		})
	})

	Describe("Describe", func() {
		It("should send the descriptors of all metric families to the provided channel", func() {
			metrics["test_counter"] = &dto.MetricFamily{
				Name: &[]string{"test_counter"}[0],
				Help: &[]string{"Test counter metric"}[0],
				Type: dto.MetricType_COUNTER.Enum(),
			}
			metrics["test_gauge"] = &dto.MetricFamily{
				Name: &[]string{"test_gauge"}[0],
				Help: &[]string{"Test gauge metric"}[0],
				Type: dto.MetricType_GAUGE.Enum(),
			}
			collector.updateMetrics(metrics)

			descChan := make(chan *prometheus.Desc, 2)
			collector.Describe(descChan)

			var desc1, desc2 *prometheus.Desc
			Eventually(descChan).Should(Receive(&desc1))
			Eventually(descChan).Should(Receive(&desc2))

			Expect([]*prometheus.Desc{desc1, desc2}).To(ContainElements(
				Equal(prometheus.NewDesc("test_counter", "Test counter metric", nil, nil)),
				Equal(prometheus.NewDesc("test_gauge", "Test gauge metric", nil, nil)),
			))

			close(descChan)
		})
	})

	Describe("Collect", func() {
		Context("with different metric types", func() {
			BeforeEach(func() {
				metrics["test_counter"] = &dto.MetricFamily{
					Name: &[]string{"test_counter"}[0],
					Help: &[]string{"Test counter metric"}[0],
					Type: dto.MetricType_COUNTER.Enum(),
					Metric: []*dto.Metric{
						{
							Label:   []*dto.LabelPair{{Name: &[]string{"label1"}[0], Value: &[]string{"value1"}[0]}},
							Counter: &dto.Counter{Value: &[]float64{10.0}[0]},
						},
					},
				}
				metrics["test_gauge"] = &dto.MetricFamily{
					Name: &[]string{"test_gauge"}[0],
					Help: &[]string{"Test gauge metric"}[0],
					Type: dto.MetricType_GAUGE.Enum(),
					Metric: []*dto.Metric{
						{
							Label: []*dto.LabelPair{{Name: &[]string{"label2"}[0], Value: &[]string{"value2"}[0]}},
							Gauge: &dto.Gauge{Value: &[]float64{20.0}[0]},
						},
					},
				}
				metrics["test_histogram"] = &dto.MetricFamily{
					Name: &[]string{"test_histogram"}[0],
					Help: &[]string{"Test histogram metric"}[0],
					Type: dto.MetricType_HISTOGRAM.Enum(),
					Metric: []*dto.Metric{
						{
							Label: []*dto.LabelPair{{Name: &[]string{"label3"}[0], Value: &[]string{"value3"}[0]}},
							Histogram: &dto.Histogram{
								SampleCount: &[]uint64{100}[0],
								SampleSum:   &[]float64{10.5}[0],
								Bucket: []*dto.Bucket{
									{CumulativeCount: &[]uint64{10}[0], UpperBound: &[]float64{1.0}[0]},
									{CumulativeCount: &[]uint64{50}[0], UpperBound: &[]float64{5.0}[0]},
									{CumulativeCount: &[]uint64{100}[0], UpperBound: &[]float64{10.0}[0]},
								},
							},
						},
					},
				}
				collector.updateMetrics(metrics)
			})

			It("should collect metrics of all types", func() {
				Expect(registry.Register(collector)).To(Succeed())
				err := testutil.GatherAndCompare(
					registry,
					strings.NewReader(`
						# HELP test_counter Test counter metric
						# TYPE test_counter counter
						test_counter{label1="value1"} 10
						# HELP test_gauge Test gauge metric
						# TYPE test_gauge gauge
						test_gauge{label2="value2"} 20
						# HELP test_histogram Test histogram metric
						# TYPE test_histogram histogram
						test_histogram_bucket{label3="value3",le="1"} 10
						test_histogram_bucket{label3="value3",le="5"} 50
						test_histogram_bucket{label3="value3",le="10"} 100
						test_histogram_sum{label3="value3"} 10.5
						test_histogram_count{label3="value3"} 100
					`),
					"test_counter", "test_gauge", "test_histogram",
				)
				Expect(err).To(BeNil())
			})
		})

		Context("when convertMetricFamilyToCollector returns an error", func() {
			BeforeEach(func() {
				metrics["test_unsupported"] = &dto.MetricFamily{
					Name: &[]string{"test_unsupported"}[0],
					Help: &[]string{"Test unsupported metric"}[0],
					//Using invalid type
					Type: &[]dto.MetricType{100}[0],
				}
				collector.updateMetrics(metrics)
			})
			It("should not panic and continue collecting other metrics", func() {
				Expect(func() {
					metricChan := make(chan prometheus.Metric, 1)
					collector.Collect(metricChan)
					close(metricChan)
				}).ShouldNot(Panic())
			})
		})
	})
})
