package statsd

import (
	"context"
	"errors"
	"time"

	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/embedded"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

type meter struct {
	embedded.Meter
	cfg    Config
	scope  instrumentation.Scope
	client *Client

	cacheInts                  *cacheWithErr[instID, *int64Inst]
	cacheFloats                *cacheWithErr[instID, *float64Inst]
	int64Observables           *cacheWithErr[instID, int64Observable]
	float64Observables         *cacheWithErr[instID, float64Observable]
	errHandler                 ErrorHandler
	observerCollectionInterval time.Duration
	callbacks                  []func(context.Context) error
}

var _ otelmetric.Meter = (*meter)(nil)

func newMeter(s instrumentation.Scope, cfg Config, errHandler func(error)) *meter {
	var int64Insts cacheWithErr[instID, *int64Inst]
	var float64Insts cacheWithErr[instID, *float64Inst]
	var int64ObservableInsts cacheWithErr[instID, int64Observable]
	var float64ObservableInsts cacheWithErr[instID, float64Observable]

	m := &meter{
		scope:      s,
		client:     cfg.client,
		cfg:        cfg,
		errHandler: errHandler,

		cacheInts:          &int64Insts,
		cacheFloats:        &float64Insts,
		int64Observables:   &int64ObservableInsts,
		float64Observables: &float64ObservableInsts,
	}

	cfg.client.collectMeter(cfg.observerCollectionInterval, m.collect)

	return m
}

func (m *meter) Int64Counter(name string, options ...otelmetric.Int64CounterOption) (otelmetric.Int64Counter, error) {
	cfg := otelmetric.NewInt64CounterConfig(options...)
	id := instID{
		Name:        name,
		Description: cfg.Description(),
		Unit:        cfg.Unit(),
		Kind:        sdkmetric.InstrumentKindCounter,
	}
	return m.cacheInts.Lookup(id, func() (*int64Inst, error) {
		return &int64Inst{
			instID: id,
			meter:  m,
		}, validateInstrumentName(name)
	})
}

func (m *meter) Int64UpDownCounter(name string, options ...otelmetric.Int64UpDownCounterOption) (otelmetric.Int64UpDownCounter, error) {
	cfg := otelmetric.NewInt64UpDownCounterConfig(options...)
	id := instID{
		Name:        name,
		Description: cfg.Description(),
		Unit:        cfg.Unit(),
		Kind:        sdkmetric.InstrumentKindUpDownCounter,
	}
	return m.cacheInts.Lookup(id, func() (*int64Inst, error) {
		return &int64Inst{
			instID: id,
			meter:  m,
		}, validateInstrumentName(name)
	})
}

func (m *meter) Int64Histogram(name string, options ...otelmetric.Int64HistogramOption) (otelmetric.Int64Histogram, error) {
	cfg := otelmetric.NewInt64HistogramConfig(options...)
	id := instID{
		Name:        name,
		Description: cfg.Description(),
		Unit:        cfg.Unit(),
		Kind:        sdkmetric.InstrumentKindHistogram,
	}
	return m.cacheInts.Lookup(id, func() (*int64Inst, error) {
		return &int64Inst{
			instID: id,
			meter:  m,
		}, validateInstrumentName(name)
	})
}

var (
	errPrecisionLoss = errors.New("warning: float counters are converted to int and will lose precision")
)

func checkPrecisionLoss(errHandler ErrorHandler, unit string) {
	if unit != "ms" && unit != "s" {
		errHandler(errPrecisionLoss)
	}
}

func (m *meter) Float64Counter(name string, options ...otelmetric.Float64CounterOption) (otelmetric.Float64Counter, error) {
	cfg := otelmetric.NewFloat64CounterConfig(options...)
	unit := cfg.Unit()
	checkPrecisionLoss(m.errHandler, unit)
	id := instID{
		Name:        name,
		Description: cfg.Description(),
		Unit:        unit,
		Kind:        sdkmetric.InstrumentKindCounter,
	}
	return m.cacheFloats.Lookup(id, func() (*float64Inst, error) {
		return &float64Inst{
			instID: id,
			meter:  m,
		}, validateInstrumentName(name)
	})
}

func (m *meter) Float64UpDownCounter(name string, options ...otelmetric.Float64UpDownCounterOption) (otelmetric.Float64UpDownCounter, error) {
	cfg := otelmetric.NewFloat64UpDownCounterConfig(options...)
	unit := cfg.Unit()
	checkPrecisionLoss(m.errHandler, unit)
	id := instID{
		Name:        name,
		Description: cfg.Description(),
		Unit:        unit,
		Kind:        sdkmetric.InstrumentKindUpDownCounter,
	}
	return m.cacheFloats.Lookup(id, func() (*float64Inst, error) {
		return &float64Inst{
			instID: id,
			meter:  m,
		}, validateInstrumentName(name)
	})
}

func (m *meter) Float64Histogram(name string, options ...otelmetric.Float64HistogramOption) (otelmetric.Float64Histogram, error) {
	cfg := otelmetric.NewFloat64HistogramConfig(options...)
	id := instID{
		Name:        name,
		Description: cfg.Description(),
		Unit:        cfg.Unit(),
		Kind:        sdkmetric.InstrumentKindHistogram,
	}
	return m.cacheFloats.Lookup(id, func() (*float64Inst, error) {
		return &float64Inst{
			instID: id,
			meter:  m,
		}, validateInstrumentName(name)
	})
}

func (m *meter) Int64ObservableCounter(name string, options ...otelmetric.Int64ObservableCounterOption) (otelmetric.Int64ObservableCounter, error) {
	cfg := otelmetric.NewInt64ObservableCounterConfig(options...)
	id := sdkmetric.Instrument{
		Name:        name,
		Description: cfg.Description(),
		Unit:        cfg.Unit(),
		Kind:        sdkmetric.InstrumentKindObservableCounter,
		Scope:       m.scope,
	}
	return m.int64ObservableInstrument(id, cfg.Callbacks())
}

func (m *meter) Int64ObservableUpDownCounter(name string, options ...otelmetric.Int64ObservableUpDownCounterOption) (otelmetric.Int64ObservableUpDownCounter, error) {
	cfg := otelmetric.NewInt64ObservableUpDownCounterConfig(options...)
	id := sdkmetric.Instrument{
		Name:        name,
		Description: cfg.Description(),
		Unit:        cfg.Unit(),
		Kind:        sdkmetric.InstrumentKindObservableUpDownCounter,
		Scope:       m.scope,
	}
	return m.int64ObservableInstrument(id, cfg.Callbacks())
}

func (m *meter) Int64ObservableGauge(name string, options ...otelmetric.Int64ObservableGaugeOption) (otelmetric.Int64ObservableGauge, error) {
	cfg := otelmetric.NewInt64ObservableGaugeConfig(options...)
	id := sdkmetric.Instrument{
		Name:        name,
		Description: cfg.Description(),
		Unit:        cfg.Unit(),
		Kind:        sdkmetric.InstrumentKindObservableGauge,
		Scope:       m.scope,
	}
	return m.int64ObservableInstrument(id, cfg.Callbacks())
}

func (m *meter) Float64ObservableCounter(name string, options ...otelmetric.Float64ObservableCounterOption) (otelmetric.Float64ObservableCounter, error) {
	cfg := otelmetric.NewFloat64ObservableCounterConfig(options...)
	unit := cfg.Unit()
	checkPrecisionLoss(m.errHandler, unit)
	id := sdkmetric.Instrument{
		Name:        name,
		Description: cfg.Description(),
		Unit:        unit,
		Kind:        sdkmetric.InstrumentKindObservableCounter,
		Scope:       m.scope,
	}
	return m.float64ObservableInstrument(id, cfg.Callbacks())
}

func (m *meter) Float64ObservableUpDownCounter(name string, options ...otelmetric.Float64ObservableUpDownCounterOption) (otelmetric.Float64ObservableUpDownCounter, error) {
	cfg := otelmetric.NewFloat64ObservableUpDownCounterConfig(options...)
	unit := cfg.Unit()
	checkPrecisionLoss(m.errHandler, unit)
	id := sdkmetric.Instrument{
		Name:        name,
		Description: cfg.Description(),
		Unit:        unit,
		Kind:        sdkmetric.InstrumentKindObservableUpDownCounter,
		Scope:       m.scope,
	}
	return m.float64ObservableInstrument(id, cfg.Callbacks())
}

func (m *meter) Float64ObservableGauge(name string, options ...otelmetric.Float64ObservableGaugeOption) (otelmetric.Float64ObservableGauge, error) {
	cfg := otelmetric.NewFloat64ObservableGaugeConfig(options...)
	id := sdkmetric.Instrument{
		Name:        name,
		Description: cfg.Description(),
		Unit:        cfg.Unit(),
		Kind:        sdkmetric.InstrumentKindObservableGauge,
		Scope:       m.scope,
	}
	return m.float64ObservableInstrument(id, cfg.Callbacks())
}

func (m *meter) RegisterCallback(f otelmetric.Callback, instruments ...otelmetric.Observable) (otelmetric.Registration, error) {
	//TODO implement me
	panic("implement me")
}

func (m *meter) collect() []metric {
	ctx, cancel := context.WithTimeout(context.Background(), m.cfg.observerCollectionTimeout)
	defer cancel()

	for _, callback := range m.callbacks {
		if err := callback(ctx); err != nil {
			m.errHandler(err)
		}
	}

	// Currently opting to send to the workers inside the callbacks
	return nil
}
