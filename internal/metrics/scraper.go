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
	log "github.com/bibendi/gruf-relay/internal/logger"
	"github.com/bibendi/gruf-relay/internal/manager"
	"github.com/bibendi/gruf-relay/internal/process"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

type Scraper struct {
	pm        *manager.Manager
	interval  time.Duration
	collector *aggregatedCollector
	client    *http.Client
}

func NewScraper(pm *manager.Manager) *Scraper {
	client := &http.Client{
		Timeout: 10 * time.Second, // Add timeout for http requests
	}

	return &Scraper{
		pm:        pm,
		interval:  5 * time.Second,
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

	server, err := newServer(ctx, s.collector)
	if err != nil {
		return err
	}

	go func() {
		if err := server.ListenAndServe(); err != nil {
			if err != http.ErrServerClosed {
				log.Error("Metrics server failed", slog.Any("error", err))
				errChan <- err
			}
		}
	}()

	for {
		select {
		case <-errChan:
			log.Error("Error scraping metrics", slog.Any("error", err))
			return err
		case <-ctx.Done():
			log.Info("Stopping metrics server")
			err := server.Shutdown(context.Background())
			if err != nil {
				log.Error("Failed to shutdown metrics server", slog.Any("error", err))
			}
			log.Info("Metrics scraper stopped")
			return err
		case <-ticker.C:
			s.scrapeAndAggregate()
		}
	}
}

func newServer(ctx context.Context, collector *aggregatedCollector) (*http.Server, error) {
	registry := prometheus.NewRegistry()
	if err := registry.Register(collector); err != nil {
		return nil, fmt.Errorf("failed to register aggregated collector: %w", err)
	}

	gatherers := prometheus.Gatherers{
		registry,
		prometheus.DefaultGatherer,
	}

	mux := http.NewServeMux()
	cfg := config.AppConfig.Metrics
	mux.Handle(cfg.Path, promhttp.HandlerFor(gatherers, promhttp.HandlerOpts{}))

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
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

	for _, p := range s.pm.Processes {
		if !p.IsRunning() {
			continue
		}

		wg.Add(1)
		go func(p process.Process) {
			defer wg.Done()

			mfList, err := s.scrapeMetrics("http://" + p.MetricsAddr())
			if err != nil {
				log.Error("Error scraping metrics", slog.Any("process", p), slog.Any("error", err))
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
		}(p)
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

	// Create a new decoder
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
