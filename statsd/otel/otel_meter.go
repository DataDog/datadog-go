package otel

import (
	otelmetric "go.opentelemetry.io/otel/metric"
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

var _ otelmetric.Meter = (*meter)(nil)

func newMeter(s instrumentation.Scope, res *resource.Resource, client *statsd.Client, errHandler func(error)) *meter {
	return &meter{
		scope:      s,
		res:        res,
		client:     client,
		errHandler: errHandler,
	}
}

func (m *meter) Int64Counter(name string, options ...otelmetric.Int64CounterOption) (otelmetric.Int64Counter, error) {
	cfg := otelmetric.NewInt64CounterConfig(options...)
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

func (m *meter) Int64UpDownCounter(name string, options ...otelmetric.Int64UpDownCounterOption) (otelmetric.Int64UpDownCounter, error) {
	cfg := otelmetric.NewInt64UpDownCounterConfig(options...)
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

func (m *meter) Int64Histogram(name string, options ...otelmetric.Int64HistogramOption) (otelmetric.Int64Histogram, error) {
	cfg := otelmetric.NewInt64HistogramConfig(options...)
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

func (m *meter) Float64Counter(name string, options ...otelmetric.Float64CounterOption) (otelmetric.Float64Counter, error) {
	cfg := otelmetric.NewFloat64CounterConfig(options...)
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

func (m *meter) Float64UpDownCounter(name string, options ...otelmetric.Float64UpDownCounterOption) (otelmetric.Float64UpDownCounter, error) {
	cfg := otelmetric.NewFloat64UpDownCounterConfig(options...)
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

func (m *meter) Float64Histogram(name string, options ...otelmetric.Float64HistogramOption) (otelmetric.Float64Histogram, error) {
	cfg := otelmetric.NewFloat64HistogramConfig(options...)
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

func (m *meter) Int64ObservableCounter(name string, options ...otelmetric.Int64ObservableCounterOption) (otelmetric.Int64ObservableCounter, error) {
	//TODO implement me
	panic("implement me")
}

func (m *meter) Int64ObservableUpDownCounter(name string, options ...otelmetric.Int64ObservableUpDownCounterOption) (otelmetric.Int64ObservableUpDownCounter, error) {
	//TODO implement me
	panic("implement me")
}

func (m *meter) Int64ObservableGauge(name string, options ...otelmetric.Int64ObservableGaugeOption) (otelmetric.Int64ObservableGauge, error) {
	//TODO implement me
	panic("implement me")
}

func (m *meter) Float64ObservableCounter(name string, options ...otelmetric.Float64ObservableCounterOption) (otelmetric.Float64ObservableCounter, error) {
	//TODO implement me
	panic("implement me")
}

func (m *meter) Float64ObservableUpDownCounter(name string, options ...otelmetric.Float64ObservableUpDownCounterOption) (otelmetric.Float64ObservableUpDownCounter, error) {
	//TODO implement me
	panic("implement me")
}

func (m *meter) Float64ObservableGauge(name string, options ...otelmetric.Float64ObservableGaugeOption) (otelmetric.Float64ObservableGauge, error) {
	//TODO implement me
	panic("implement me")
}

func (m *meter) RegisterCallback(f otelmetric.Callback, instruments ...otelmetric.Observable) (otelmetric.Registration, error) {
	//TODO implement me
	panic("implement me")
}
