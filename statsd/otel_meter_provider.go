package statsd

import (
	"log/slog"
	"sync/atomic"
	"time"

	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/embedded"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/sdk/instrumentation"
)

type MeterProvider struct {
	embedded.MeterProvider

	errorHandler ErrorHandler
	logger       *slog.Logger

	errChan chan error
	cfg     Config

	stopped atomic.Bool
	meters  cache[instrumentation.Scope, *meter]
}

var _ otelmetric.MeterProvider = (*MeterProvider)(nil)

func NewMeterProvider(options ...OTELOption) (*MeterProvider, error) {
	cfg := newConfig(options...)

	if cfg.client == nil {
		return nil, ErrNoClient
	}

	mp := &MeterProvider{
		errorHandler: cfg.client.errorHandler,
		logger:       cfg.logger,
		cfg:          cfg,
		errChan:      make(chan error, 10),
	}

	go mp.processErrors()

	return mp, nil
}

func (mp *MeterProvider) Meter(name string, opts ...otelmetric.MeterOption) otelmetric.Meter {
	if name == "" {
		mp.logger.Warn("Invalid Meter name.", "name", name)
	}

	if mp.stopped.Load() {
		return noop.Meter{}
	}

	c := otelmetric.NewMeterConfig(opts...)

	s := instrumentation.Scope{
		Name:      name,
		Version:   c.InstrumentationVersion(),
		SchemaURL: c.SchemaURL(),
	}

	return mp.meters.Lookup(s, func() *meter {
		return newMeter(s, mp.cfg, mp.handleError)
	})
}

func (mp *MeterProvider) handleError(err error) {
	select {
	case mp.errChan <- err:
	default:
		mp.logger.Error("dropped error", err.Error())
	}
}

func (mp *MeterProvider) processErrors() {
	for {
		if mp.stopped.Load() {
			return
		}
		select {
		case <-time.After(time.Second):
		case err := <-mp.errChan:
			mp.errorHandler(err)
		}
	}
}
