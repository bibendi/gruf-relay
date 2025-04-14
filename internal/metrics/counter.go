package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

type counterCollector struct {
	desc   *prometheus.Desc
	mf     *dto.MetricFamily
	labels []string
}

func newCounterCollector(mf *dto.MetricFamily) *counterCollector {
	var labelNames []string
	if len(mf.Metric) > 0 {
		for _, label := range mf.Metric[0].Label {
			labelNames = append(labelNames, *label.Name)
		}
	}

	return &counterCollector{
		desc: prometheus.NewDesc(
			*mf.Name,
			*mf.Help,
			labelNames,
			nil,
		),
		mf:     mf,
		labels: labelNames,
	}
}

func (c *counterCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.desc
}

func (c *counterCollector) Collect(ch chan<- prometheus.Metric) {
	collectGeneric(
		c.mf,
		c.desc,
		c.labels,
		func(metric *dto.Metric) float64 { return *metric.Counter.Value },
		func(current, new float64) float64 { return current + new },
		prometheus.CounterValue,
		ch,
	)
}
