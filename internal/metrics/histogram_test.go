package metrics

import (
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
)

func TestHistogramCollector(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Histogram Collector Suite")
}

var _ = Describe("HistogramCollector", func() {
	var (
		mf        *dto.MetricFamily
		collector *histogramCollector
		registry  *prometheus.Registry
	)

	BeforeEach(func() {
		registry = prometheus.NewRegistry()

		mf = &dto.MetricFamily{
			Name: &[]string{"test_histogram"}[0],
			Help: &[]string{"Test histogram metric"}[0],
			Type: dto.MetricType_HISTOGRAM.Enum(),
			Metric: []*dto.Metric{
				{
					Label: []*dto.LabelPair{
						{
							Name:  &[]string{"label1"}[0],
							Value: &[]string{"value1"}[0],
						},
					},
					Histogram: &dto.Histogram{
						SampleCount: &[]uint64{100}[0],
						SampleSum:   &[]float64{10.5}[0],
						Bucket: []*dto.Bucket{
							{
								CumulativeCount: &[]uint64{10}[0],
								UpperBound:      &[]float64{1.0}[0],
							},
							{
								CumulativeCount: &[]uint64{50}[0],
								UpperBound:      &[]float64{5.0}[0],
							},
							{
								CumulativeCount: &[]uint64{100}[0],
								UpperBound:      &[]float64{10.0}[0],
							},
						},
					},
				},
			},
		}

		collector = newHistogramCollector(mf)
	})

	Describe("newHistogramCollector", func() {
		It("should create a new histogram collector", func() {
			Expect(collector).NotTo(BeNil())
			Expect(collector.desc).NotTo(BeNil())
			Expect(collector.desc.String()).To(ContainSubstring("test_histogram"))
			Expect(collector.labels).To(Equal([]string{"label1"}))
		})
	})

	Describe("Describe", func() {
		It("should send the descriptor to the provided channel", func() {
			descChan := make(chan *prometheus.Desc, 1)
			collector.Describe(descChan)
			Eventually(descChan).Should(Receive(Equal(collector.desc)))
			close(descChan)
		})
	})

	Describe("Collect", func() {
		It("should send the metric to the provided channel", func() {
			Expect(registry.Register(collector)).To(Succeed())
			err := testutil.GatherAndCompare(
				registry,
				strings.NewReader(`
					# HELP test_histogram Test histogram metric
					# TYPE test_histogram histogram
					test_histogram_bucket{label1="value1", le="1"} 10
					test_histogram_bucket{label1="value1", le="5"} 50
					test_histogram_bucket{label1="value1", le="10"} 100
					test_histogram_sum{label1="value1"} 10.5
					test_histogram_count{label1="value1"} 100
				`),
				"test_histogram",
			)
			Expect(err).To(BeNil())
		})

		Context("when metric family has multiple histograms", func() {
			BeforeEach(func() {
				mf = &dto.MetricFamily{
					Name: &[]string{"test_histogram"}[0],
					Help: &[]string{"Test histogram metric"}[0],
					Type: dto.MetricType_HISTOGRAM.Enum(),
					Metric: []*dto.Metric{
						{
							Label: []*dto.LabelPair{
								{
									Name:  &[]string{"label1"}[0],
									Value: &[]string{"value1"}[0],
								},
							},
							Histogram: &dto.Histogram{
								SampleCount: &[]uint64{100}[0],
								SampleSum:   &[]float64{10.5}[0],
								Bucket: []*dto.Bucket{
									{
										CumulativeCount: &[]uint64{10}[0],
										UpperBound:      &[]float64{1.0}[0],
									},
									{
										CumulativeCount: &[]uint64{50}[0],
										UpperBound:      &[]float64{5.0}[0],
									},
									{
										CumulativeCount: &[]uint64{100}[0],
										UpperBound:      &[]float64{10.0}[0],
									},
								},
							},
						},
						{
							Label: []*dto.LabelPair{
								{
									Name:  &[]string{"label1"}[0],
									Value: &[]string{"value2"}[0],
								},
							},
							Histogram: &dto.Histogram{
								SampleCount: &[]uint64{200}[0],
								SampleSum:   &[]float64{20.5}[0],
								Bucket: []*dto.Bucket{
									{
										CumulativeCount: &[]uint64{20}[0],
										UpperBound:      &[]float64{1.0}[0],
									},
									{
										CumulativeCount: &[]uint64{100}[0],
										UpperBound:      &[]float64{5.0}[0],
									},
									{
										CumulativeCount: &[]uint64{200}[0],
										UpperBound:      &[]float64{10.0}[0],
									},
								},
							},
						},
					},
				}
				collector = newHistogramCollector(mf)
			})

			It("collects all metric values", func() {
				Expect(registry.Register(collector)).To(Succeed())
				err := testutil.GatherAndCompare(
					registry,
					strings.NewReader(`
						# HELP test_histogram Test histogram metric
						# TYPE test_histogram histogram
						test_histogram_bucket{label1="value1", le="1"} 10
						test_histogram_bucket{label1="value1", le="5"} 50
						test_histogram_bucket{label1="value1", le="10"} 100
						test_histogram_sum{label1="value1"} 10.5
						test_histogram_count{label1="value1"} 100
						test_histogram_bucket{label1="value2", le="1"} 20
						test_histogram_bucket{label1="value2", le="5"} 100
						test_histogram_bucket{label1="value2", le="10"} 200
						test_histogram_sum{label1="value2"} 20.5
						test_histogram_count{label1="value2"} 200
					`),
					"test_histogram",
				)
				Expect(err).To(BeNil())
			})
		})

		Context("when metric family has same labels", func() {
			BeforeEach(func() {
				mf = &dto.MetricFamily{
					Name: &[]string{"test_histogram"}[0],
					Help: &[]string{"Test histogram metric"}[0],
					Type: dto.MetricType_HISTOGRAM.Enum(),
					Metric: []*dto.Metric{
						{
							Label: []*dto.LabelPair{
								{
									Name:  &[]string{"label1"}[0],
									Value: &[]string{"value1"}[0],
								},
							},
							Histogram: &dto.Histogram{
								SampleCount: &[]uint64{100}[0],
								SampleSum:   &[]float64{10.5}[0],
								Bucket: []*dto.Bucket{
									{
										CumulativeCount: &[]uint64{10}[0],
										UpperBound:      &[]float64{1.0}[0],
									},
									{
										CumulativeCount: &[]uint64{50}[0],
										UpperBound:      &[]float64{5.0}[0],
									},
									{
										CumulativeCount: &[]uint64{100}[0],
										UpperBound:      &[]float64{10.0}[0],
									},
								},
							},
						},
						{
							Label: []*dto.LabelPair{
								{
									Name:  &[]string{"label1"}[0],
									Value: &[]string{"value1"}[0],
								},
							},
							Histogram: &dto.Histogram{
								SampleCount: &[]uint64{200}[0],
								SampleSum:   &[]float64{20.5}[0],
								Bucket: []*dto.Bucket{
									{
										CumulativeCount: &[]uint64{20}[0],
										UpperBound:      &[]float64{1.0}[0],
									},
									{
										CumulativeCount: &[]uint64{100}[0],
										UpperBound:      &[]float64{5.0}[0],
									},
									{
										CumulativeCount: &[]uint64{200}[0],
										UpperBound:      &[]float64{10.0}[0],
									},
								},
							},
						},
					},
				}
				collector = newHistogramCollector(mf)
			})

			It("collects and aggregates same labels", func() {
				Expect(registry.Register(collector)).To(Succeed())
				err := testutil.GatherAndCompare(
					registry,
					strings.NewReader(`
						# HELP test_histogram Test histogram metric
						# TYPE test_histogram histogram
						test_histogram_bucket{label1="value1", le="1"} 30
						test_histogram_bucket{label1="value1", le="5"} 150
						test_histogram_bucket{label1="value1", le="10"} 300
						test_histogram_sum{label1="value1"} 31
						test_histogram_count{label1="value1"} 300
					`),
					"test_histogram",
				)
				Expect(err).To(BeNil())
			})
		})
	})
})
