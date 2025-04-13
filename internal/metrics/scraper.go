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
	"github.com/bibendi/gruf-relay/internal/manager"
	"github.com/bibendi/gruf-relay/internal/process"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

type Scraper struct {
	ctx       context.Context
	wg        *sync.WaitGroup
	log       *slog.Logger
	pm        *manager.Manager
	interval  time.Duration
	collector *aggregatedCollector
	client    *http.Client
	server    *http.Server
}

func NewScraper(ctx context.Context, wg *sync.WaitGroup, log *slog.Logger, pm *manager.Manager, cfg *config.Metrics) (*Scraper, error) {
	client := &http.Client{
		Timeout: 10 * time.Second, // Add timeout for http requests
	}

	registry := prometheus.NewRegistry()
	collector := newAggregatedCollector(log)
	if err := registry.Register(collector); err != nil {
		return nil, fmt.Errorf("failed to register aggregated collector: %w", err)
	}
	gatherers := prometheus.Gatherers{
		registry,
		prometheus.DefaultGatherer,
	}

	mux := http.NewServeMux()
	mux.Handle(cfg.Path, promhttp.HandlerFor(gatherers, promhttp.HandlerOpts{}))

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: mux,
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}

	return &Scraper{
		ctx:       ctx,
		wg:        wg,
		log:       log,
		pm:        pm,
		interval:  5 * time.Second,
		client:    client,
		server:    server,
		collector: collector,
	}, nil
}

func (s *Scraper) Start() {
	s.wg.Add(1)
	s.log.Info("Starting metrics scraper")
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	defer s.wg.Done()

	go s.runServer()

	for {
		select {
		case <-s.ctx.Done():
			s.log.Info("Stopping metrics server")
			if err := s.server.Shutdown(context.Background()); err != nil {
				s.log.Error("Failed to shutdown metrics server", slog.Any("err", err))
			}
			s.log.Info("Metrics scraper stopped")
			return
		case <-ticker.C:
			s.scrapeAndAggregate()
		}
	}
}

func (s *Scraper) runServer() {
	if err := s.server.ListenAndServe(); err != nil {
		if err != http.ErrServerClosed {
			s.log.Error("Metrics server failed", slog.Any("err", err))
		}
	}
}

func (s *Scraper) scrapeAndAggregate() {
	s.log.Info("Scraping metrics")
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
				s.log.Error("Error scraping metrics", slog.Any("process", p), slog.Any("error", err))
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
	s.log.Info("Metrics scraped and aggregated")
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

	// Create a new decoder.
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

	//s.log.Debug("Scraped metrics", slog.Any("url", url), slog.Any("metrics", mfList))

	return mfList, nil
}
