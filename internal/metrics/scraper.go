//go:generate mockgen -source=scraper.go -destination=scraper_mock.go -package=metrics
package metrics

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/bibendi/gruf-relay/internal/config"
	"github.com/bibendi/gruf-relay/internal/log"
	"github.com/bibendi/gruf-relay/internal/worker"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

type Manager interface {
	GetWorkers() map[string]worker.Worker
}

type Scraper struct {
	m         Manager
	interval  time.Duration
	port      int
	path      string
	collector *aggregatedCollector
	client    *http.Client
}

func NewScraper(cfg config.Metrics, m Manager) *Scraper {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	return &Scraper{
		m:         m,
		interval:  cfg.Interval,
		port:      cfg.Port,
		path:      cfg.Path,
		client:    client,
		collector: newAggregatedCollector(),
	}
}

func (s *Scraper) Serve(ctx context.Context) error {
	log.Info("Starting metrics scraper")
	ticker := time.NewTicker(s.interval)
	errChan := make(chan error, 1)
	defer ticker.Stop()
	defer close(errChan)

	server, err := newServer(ctx, s.port, s.path, s.collector)
	if err != nil {
		return err
	}

	go func() {
		if err := server.ListenAndServe(); err != nil {
			if err != http.ErrServerClosed {
				log.Error("Failed to serve the metrics server", slog.Any("error", err))
				errChan <- err
			}
		}
	}()

	for {
		select {
		case err := <-errChan:
			log.Error("Error scraping metrics", slog.Any("error", err))
			return err
		case <-ctx.Done():
			log.Info("Stopping metrics server")
			err := server.Shutdown(context.Background())
			if err != nil {
				log.Error("Failed to shutdown metrics server", slog.Any("error", err))
			}
			return nil
		case <-ticker.C:
			s.scrapeAndAggregate()
		}
	}
}

func newServer(ctx context.Context, port int, path string, collector *aggregatedCollector) (*http.Server, error) {
	registry := prometheus.NewRegistry()
	if err := registry.Register(collector); err != nil {
		return nil, fmt.Errorf("failed to register aggregated collector: %w", err)
	}

	gatherers := prometheus.Gatherers{
		registry,
		prometheus.DefaultGatherer,
	}

	mux := http.NewServeMux()
	mux.Handle(path, promhttp.HandlerFor(gatherers, promhttp.HandlerOpts{}))

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}

	return server, nil
}

func (s *Scraper) scrapeAndAggregate() {
	log.Info("Scraping metrics")
	var wg sync.WaitGroup
	var mapMu sync.Mutex

	metricsMap := make(map[string]*dto.MetricFamily)

	for name, w := range s.m.GetWorkers() {
		if !w.IsRunning() {
			continue
		}

		wg.Add(1)
		go func(w worker.Worker) {
			defer wg.Done()

			mfList, err := s.scrapeMetrics("http://" + w.MetricsAddr())
			if err != nil {
				log.Error("Error scraping metrics", slog.String("worker", name), slog.Any("error", err))
				return
			}

			mapMu.Lock()
			for _, mf := range mfList {
				if existingMF, ok := metricsMap[*mf.Name]; ok {
					existingMF.Metric = append(existingMF.Metric, mf.Metric...)
				} else {
					metricsMap[*mf.Name] = mf
				}
			}
			mapMu.Unlock()
		}(w)
	}

	wg.Wait()
	s.collector.updateMetrics(metricsMap)
	log.Info("Metrics scraped and aggregated")
}

func (s *Scraper) scrapeMetrics(url string) ([]*dto.MetricFamily, error) {
	resp, err := s.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch metrics from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch metrics from %s, status code: %d", url, resp.StatusCode)
	}

	decoder := expfmt.NewDecoder(resp.Body, expfmt.FmtText)

	var mfList []*dto.MetricFamily
	for {
		var mf dto.MetricFamily
		err := decoder.Decode(&mf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to decode metric: %w", err)
		}

		mfList = append(mfList, &mf)
	}

	//log.Debug("Scraped metrics", slog.Any("url", url), slog.Any("metrics", mfList))

	return mfList, nil
}
