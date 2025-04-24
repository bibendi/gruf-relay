package metrics

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
)

var _ = Describe("GaugeCollector", func() {
	var (
		mf        *dto.MetricFamily
		collector *gaugeCollector
		registry  *prometheus.Registry
	)

	BeforeEach(func() {
		registry = prometheus.NewRegistry()

		mf = &dto.MetricFamily{
			Name: &[]string{"test_gauge"}[0],
			Help: &[]string{"Test gauge metric"}[0],
			Type: dto.MetricType_GAUGE.Enum(),
			Metric: []*dto.Metric{
				{
					Label: []*dto.LabelPair{
						{
							Name:  &[]string{"label1"}[0],
							Value: &[]string{"value1"}[0],
						},
					},
					Gauge: &dto.Gauge{
						Value: &[]float64{10.0}[0],
					},
				},
			},
		}

		collector = newGaugeCollector(mf)
	})

	Describe("newGaugeCollector", func() {
		It("should create a new gauge collector", func() {
			Expect(collector).NotTo(BeNil())
			Expect(collector.desc).NotTo(BeNil())
			Expect(collector.desc.String()).To(ContainSubstring("test_gauge"))
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
					# HELP test_gauge Test gauge metric
					# TYPE test_gauge gauge
					test_gauge{label1="value1"} 10
				`),
				"test_gauge",
			)
			Expect(err).To(BeNil())
		})

		Context("when metric family has multiple metrics", func() {
			BeforeEach(func() {
				mf = &dto.MetricFamily{
					Name: &[]string{"test_gauge"}[0],
					Help: &[]string{"Test gauge metric"}[0],
					Type: dto.MetricType_GAUGE.Enum(),
					Metric: []*dto.Metric{
						{
							Label: []*dto.LabelPair{
								{
									Name:  &[]string{"label1"}[0],
									Value: &[]string{"value1"}[0],
								},
							},
							Gauge: &dto.Gauge{
								Value: &[]float64{10.0}[0],
							},
						},
						{
							Label: []*dto.LabelPair{
								{
									Name:  &[]string{"label1"}[0],
									Value: &[]string{"value2"}[0],
								},
							},
							Gauge: &[]dto.Gauge{{
								Value: &[]float64{20.0}[0],
							}}[0],
						},
					},
				}
				collector = newGaugeCollector(mf)
			})

			It("collects all metric values", func() {
				Expect(registry.Register(collector)).To(Succeed())
				err := testutil.GatherAndCompare(
					registry,
					strings.NewReader(`
						# HELP test_gauge Test gauge metric
						# TYPE test_gauge gauge
						test_gauge{label1="value1"} 10
						test_gauge{label1="value2"} 20
					`),
					"test_gauge",
				)
				Expect(err).To(BeNil())
			})
		})
	})
})
