package metrics

import (
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

type gaugeCollector struct {
	desc   *prometheus.Desc
	mf     *dto.MetricFamily
	labels []string
	log    *slog.Logger
}

// newGaugeCollector - конструктор для gaugeCollector.
func newGaugeCollector(log *slog.Logger, mf *dto.MetricFamily) prometheus.Collector {
	// Get the label names from the MetricFamily
	var labelNames []string
	if len(mf.Metric) > 0 {
		for _, label := range mf.Metric[0].Label {
			labelNames = append(labelNames, *label.Name)
		}
	}

	return &gaugeCollector{
		desc: prometheus.NewDesc(
			*mf.Name,
			*mf.Help,
			labelNames,
			nil,
		),
		mf:     mf,
		labels: labelNames,
		log:    log,
	}
}

func (g *gaugeCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- g.desc
}

func (g *gaugeCollector) Collect(ch chan<- prometheus.Metric) {
	collectGeneric(
		g.mf,
		g.desc,
		g.labels,
		func(metric *dto.Metric) float64 { return *metric.Gauge.Value },
		func(current, new float64) float64 { return new },
		prometheus.CounterValue,
		ch,
	)
}
