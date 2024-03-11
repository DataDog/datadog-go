package otel

import (
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/embedded"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/resource"

	"github.com/DataDog/datadog-go/v5/statsd"
)

type meter struct {
	embedded.Meter
	res    *resource.Resource
	scope  instrumentation.Scope
	client *statsd.Client

	cacheInts   cacheWithErr[string, *int64Inst]
	cacheFloats cacheWithErr[string, *float64Inst]
	errHandler  func(error)
}

var _ metric.Meter = (*meter)(nil)

func newMeter(s instrumentation.Scope, res *resource.Resource, client *statsd.Client, errHandler func(error)) *meter {
	return &meter{
		scope:      s,
		res:        res,
		client:     client,
		errHandler: errHandler,
	}
}

func (m *meter) Int64Counter(name string, options ...metric.Int64CounterOption) (metric.Int64Counter, error) {
	cfg := metric.NewInt64CounterConfig(options...)
	return m.cacheInts.Lookup(name, func() (*int64Inst, error) {
		return &int64Inst{
			name:    name,
			unit:    cfg.Unit(),
			desc:    cfg.Description(),
			meter:   m,
			isGauge: false,
		}, nil
	})
}

func (m *meter) Int64UpDownCounter(name string, options ...metric.Int64UpDownCounterOption) (metric.Int64UpDownCounter, error) {
	cfg := metric.NewInt64UpDownCounterConfig(options...)
	return m.cacheInts.Lookup(name, func() (*int64Inst, error) {
		return &int64Inst{
			name:    name,
			unit:    cfg.Unit(),
			desc:    cfg.Description(),
			meter:   m,
			isGauge: true,
		}, nil
	})
}

func (m *meter) Int64Histogram(name string, options ...metric.Int64HistogramOption) (metric.Int64Histogram, error) {
	cfg := metric.NewInt64HistogramConfig(options...)
	return m.cacheInts.Lookup(name, func() (*int64Inst, error) {
		return &int64Inst{
			name:    name,
			unit:    cfg.Unit(),
			desc:    cfg.Description(),
			meter:   m,
			isGauge: false,
		}, nil
	})
}

func (m *meter) Float64Counter(name string, options ...metric.Float64CounterOption) (metric.Float64Counter, error) {
	cfg := metric.NewFloat64CounterConfig(options...)
	return m.cacheFloats.Lookup(name, func() (*float64Inst, error) {
		return &float64Inst{
			name:    name,
			unit:    cfg.Unit(),
			desc:    cfg.Description(),
			meter:   m,
			isGauge: false,
		}, nil
	})
}

func (m *meter) Float64UpDownCounter(name string, options ...metric.Float64UpDownCounterOption) (metric.Float64UpDownCounter, error) {
	cfg := metric.NewFloat64UpDownCounterConfig(options...)
	return m.cacheFloats.Lookup(name, func() (*float64Inst, error) {
		return &float64Inst{
			name:    name,
			unit:    cfg.Unit(),
			desc:    cfg.Description(),
			meter:   m,
			isGauge: true,
		}, nil
	})
}

func (m *meter) Float64Histogram(name string, options ...metric.Float64HistogramOption) (metric.Float64Histogram, error) {
	cfg := metric.NewFloat64HistogramConfig(options...)
	return m.cacheFloats.Lookup(name, func() (*float64Inst, error) {
		return &float64Inst{
			name:    name,
			unit:    cfg.Unit(),
			desc:    cfg.Description(),
			meter:   m,
			isGauge: false,
		}, nil
	})
}

func (m *meter) Int64ObservableCounter(name string, options ...metric.Int64ObservableCounterOption) (metric.Int64ObservableCounter, error) {
	//TODO implement me
	panic("implement me")
}

func (m *meter) Int64ObservableUpDownCounter(name string, options ...metric.Int64ObservableUpDownCounterOption) (metric.Int64ObservableUpDownCounter, error) {
	//TODO implement me
	panic("implement me")
}

func (m *meter) Int64ObservableGauge(name string, options ...metric.Int64ObservableGaugeOption) (metric.Int64ObservableGauge, error) {
	//TODO implement me
	panic("implement me")
}

func (m *meter) Float64ObservableCounter(name string, options ...metric.Float64ObservableCounterOption) (metric.Float64ObservableCounter, error) {
	//TODO implement me
	panic("implement me")
}

func (m *meter) Float64ObservableUpDownCounter(name string, options ...metric.Float64ObservableUpDownCounterOption) (metric.Float64ObservableUpDownCounter, error) {
	//TODO implement me
	panic("implement me")
}

func (m *meter) Float64ObservableGauge(name string, options ...metric.Float64ObservableGaugeOption) (metric.Float64ObservableGauge, error) {
	//TODO implement me
	panic("implement me")
}

func (m *meter) RegisterCallback(f metric.Callback, instruments ...metric.Observable) (metric.Registration, error) {
	//TODO implement me
	panic("implement me")
}
