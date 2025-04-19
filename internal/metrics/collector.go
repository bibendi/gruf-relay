package metrics

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/bibendi/gruf-relay/internal/log"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

type aggregatedCollector struct {
	mu      sync.Mutex
	metrics map[string]*dto.MetricFamily
}

func newAggregatedCollector() *aggregatedCollector {
	return &aggregatedCollector{
		metrics: make(map[string]*dto.MetricFamily),
	}
}

func (c *aggregatedCollector) Collect(ch chan<- prometheus.Metric) {
	c.mu.Lock()
	defer c.mu.Unlock()
	//log.Debug("Collecting metrics", slog.Any("metrics", c.metrics))

	for _, mf := range c.metrics {
		collector, err := c.convertMetricFamilyToCollector(mf)
		if err != nil {
			log.Error("Error converting metric family to collector", slog.Any("error", err))
			continue
		}
		collector.Collect(ch)
	}
}

func (c *aggregatedCollector) Describe(ch chan<- *prometheus.Desc) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, mf := range c.metrics {
		ch <- prometheus.NewDesc(*mf.Name, *mf.Help, nil, nil)
	}
}

func (c *aggregatedCollector) updateMetrics(metricsMap map[string]*dto.MetricFamily) {
	c.mu.Lock()
	defer c.mu.Unlock()
	//log.Debug("Updating metrics map", slog.Any("metrics", metricsMap))
	c.metrics = metricsMap
}

func (c *aggregatedCollector) convertMetricFamilyToCollector(mf *dto.MetricFamily) (prometheus.Collector, error) {
	switch *mf.Type {
	case dto.MetricType_COUNTER:
		return newCounterCollector(mf), nil
	case dto.MetricType_GAUGE:
		return newGaugeCollector(mf), nil
	case dto.MetricType_HISTOGRAM:
		return newHistogramCollector(mf), nil
	// case dto.MetricType_SUMMARY:
	// 	return newSummaryCollector(mf), nil
	// case dto.MetricType_UNTYPED:
	// 	return newUntypedCollector(mf), nil
	default:
		return nil, fmt.Errorf("unsupported metric type: %s", mf.Type.String())
	}
}

func collectGeneric(
	mf *dto.MetricFamily,
	desc *prometheus.Desc,
	labels []string,
	valueFn func(*dto.Metric) float64,
	aggregateFn func(float64, float64) float64,
	valueType prometheus.ValueType,
	ch chan<- prometheus.Metric,
) {
	aggregatedValues := make(map[string]float64)

	for _, metric := range mf.Metric {
		labelValues := make([]string, len(labels))
		for i, labelName := range labels {
			for _, label := range metric.Label {
				if *label.Name == labelName {
					labelValues[i] = *label.Value
					break
				}
			}
		}
		key := strings.Join(labelValues, ",")
		val := valueFn(metric)
		aggregatedValues[key] = aggregateFn(aggregatedValues[key], val)
	}
	for key, val := range aggregatedValues {
		labelValues := strings.Split(key, ",")
		allEmpty := true
		for _, lv := range labelValues {
			if lv != "" {
				allEmpty = false
				break
			}
		}
		ch <- prometheus.MustNewConstMetric(desc, valueType, val, getLabelValues(allEmpty, labelValues)...)
	}
}

func getLabelValues(allEmpty bool, labelValues []string) []string {
	if allEmpty {
		return nil
	}

	return labelValues
}
