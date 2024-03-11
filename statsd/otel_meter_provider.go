package statsd

import (
	"log/slog"
	"sync/atomic"
	"time"

	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/embedded"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/resource"
)

type MeterProvider struct {
	embedded.MeterProvider

	client *Client
	logger *slog.Logger
	res    *resource.Resource

	errChan    chan error
	errHandler func(error)

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
		client:     cfg.client,
		logger:     cfg.logger,
		errHandler: cfg.client.errorHandler,
		errChan:    make(chan error, 10),
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
		return newMeter(s, mp.res, mp.client, mp.handleError)
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
			mp.errHandler(err)
		}
	}
}
