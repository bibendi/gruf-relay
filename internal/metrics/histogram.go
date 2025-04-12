package metrics

import (
	"log/slog"
	"math"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

type histogramCollector struct {
	desc   *prometheus.Desc
	mf     *dto.MetricFamily
	labels []string
	log    *slog.Logger
}

func newHistogramCollector(log *slog.Logger, mf *dto.MetricFamily) *histogramCollector {
	var labelNames []string
	if len(mf.Metric) > 0 {
		for _, label := range mf.Metric[0].Label {
			labelNames = append(labelNames, *label.Name)
		}
	}

	return &histogramCollector{
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

func (h *histogramCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- h.desc
}

// Collect - собирает метрики Histogram.
func (h *histogramCollector) Collect(ch chan<- prometheus.Metric) {
	aggregatedHistograms := h.aggregateHistograms()
	h.sendHistogramsToChannel(aggregatedHistograms, ch)
}

// aggregateHistograms агрегирует гистограммы по лейблам.
func (h *histogramCollector) aggregateHistograms() map[string]*dto.Histogram {
	aggregatedHistograms := make(map[string]*dto.Histogram)

	for _, metric := range h.mf.Metric {
		key := h.getLabelKey(metric)

		hist, ok := aggregatedHistograms[key]
		if !ok {
			hist = &dto.Histogram{
				SampleCount: new(uint64),
				SampleSum:   new(float64),
				Bucket:      make([]*dto.Bucket, 0),
			}
			aggregatedHistograms[key] = hist
		}

		h.aggregateHistogramData(metric, hist)
	}

	return aggregatedHistograms
}

func (h *histogramCollector) getLabelKey(metric *dto.Metric) string {
	labelValues := make([]string, len(h.labels))
	for i, labelName := range h.labels {
		for _, label := range metric.Label {
			if *label.Name == labelName {
				labelValues[i] = *label.Value
				break
			}
		}
	}
	return strings.Join(labelValues, ",")
}

func (h *histogramCollector) aggregateHistogramData(metric *dto.Metric, hist *dto.Histogram) {
	if metric.Histogram == nil {
		return
	}

	if metric.Histogram.SampleCount != nil {
		*hist.SampleCount += *metric.Histogram.SampleCount
	}
	if metric.Histogram.SampleSum != nil {
		*hist.SampleSum += *metric.Histogram.SampleSum
	}

	for _, bucket := range metric.Histogram.Bucket {
		h.aggregateBucket(bucket, hist)
	}
}

func (h *histogramCollector) aggregateBucket(bucket *dto.Bucket, hist *dto.Histogram) {
	upperBound := *bucket.UpperBound

	found := false
	for _, aggBucket := range hist.Bucket {
		if math.Abs(*aggBucket.UpperBound-upperBound) < 1e-10 {
			*aggBucket.CumulativeCount += *bucket.CumulativeCount
			found = true
			break
		}
	}

	if !found {
		newBucket := &dto.Bucket{
			CumulativeCount: new(uint64),
			UpperBound:      new(float64),
		}
		*newBucket.UpperBound = upperBound
		*newBucket.CumulativeCount = *bucket.CumulativeCount
		hist.Bucket = append(hist.Bucket, newBucket)
	}
}

func (h *histogramCollector) sendHistogramsToChannel(aggregatedHistograms map[string]*dto.Histogram, ch chan<- prometheus.Metric) {
	for key, hist := range aggregatedHistograms {
		labelValues := strings.Split(key, ",")

		buckets := make(map[float64]uint64)
		for _, bucket := range hist.Bucket {
			buckets[*bucket.UpperBound] = *bucket.CumulativeCount
		}

		ch <- prometheus.MustNewConstHistogram(
			h.desc,
			*hist.SampleCount,
			*hist.SampleSum,
			buckets,
			h.prepareLabelValues(labelValues)...,
		)
	}
}

func (h *histogramCollector) prepareLabelValues(labelValues []string) []string {
	allEmpty := true
	for _, lv := range labelValues {
		if lv != "" {
			allEmpty = false
			break
		}
	}

	if allEmpty {
		return nil
	}
	return labelValues
}
