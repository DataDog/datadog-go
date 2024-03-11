package statsd

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/embedded"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

type int64Inst struct {
	embedded.Int64Counter
	embedded.Int64UpDownCounter
	embedded.Int64Histogram
	embedded.Int64Gauge

	meter *meter
	instID
}

var _ otelmetric.Int64Counter = (*int64Inst)(nil)
var _ otelmetric.Int64UpDownCounter = (*int64Inst)(nil)
var _ otelmetric.Int64Histogram = (*int64Inst)(nil)
var _ otelmetric.Int64Gauge = (*int64Inst)(nil)

func (i *int64Inst) Add(_ context.Context, incr int64, options ...otelmetric.AddOption) {
	c := otelmetric.NewAddConfig(options)
	tags := attrsToTags(c.Attributes(), i.meter.cfg.res.Attributes())

	var err error
	if i.Kind == sdkmetric.InstrumentKindUpDownCounter {
		err = i.meter.client.Gauge(i.Name, float64(incr), tags, 1)
	} else {
		err = i.meter.client.Count(i.Name, incr, tags, 1)
	}
	if err != nil {
		i.meter.errHandler(err)
	}
}

func (i *int64Inst) Record(_ context.Context, value int64, options ...otelmetric.RecordOption) {
	c := otelmetric.NewRecordConfig(options)
	tags := attrsToTags(c.Attributes(), i.meter.cfg.res.Attributes())
	if err := i.meter.client.Histogram(i.Name, float64(value), tags, 1); err != nil {
		i.meter.errHandler(err)
	}
}

type float64Inst struct {
	embedded.Float64Counter
	embedded.Float64UpDownCounter
	embedded.Float64Histogram
	embedded.Float64Gauge

	meter *meter
	instID
}

var _ otelmetric.Float64Counter = (*float64Inst)(nil)
var _ otelmetric.Float64UpDownCounter = (*float64Inst)(nil)
var _ otelmetric.Float64Histogram = (*float64Inst)(nil)
var _ otelmetric.Float64Gauge = (*float64Inst)(nil)

func (i *float64Inst) Add(_ context.Context, incr float64, options ...otelmetric.AddOption) {
	c := otelmetric.NewAddConfig(options)
	tags := attrsToTags(c.Attributes(), i.meter.cfg.res.Attributes())

	var err error
	if i.Kind == sdkmetric.InstrumentKindUpDownCounter {
		err = i.meter.client.Gauge(i.Name, incr, tags, 1)
	} else {
		atomic.AddUint64(&i.meter.client.telemetry.totalMetricsCount, 1)
		// TODO: Possible to maybe send this as a float instead of casting?
		err = i.meter.client.send(metric{metricType: count, name: i.Name, fvalue: incr, tags: tags, rate: 1, globalTags: i.meter.client.tags, namespace: i.meter.client.namespace})
		// If not, we use this.
		//err = i.meter.client.Count(i.name, int64(incr), tags, 1)
	}
	if err != nil {
		i.meter.errHandler(err)
	}
}

func (i *float64Inst) Record(_ context.Context, value float64, options ...otelmetric.RecordOption) {
	c := otelmetric.NewRecordConfig(options)
	tags := attrsToTags(c.Attributes(), i.meter.cfg.res.Attributes())
	if err := i.meter.client.Histogram(i.Name, value, tags, 1); err != nil {
		i.meter.errHandler(err)
	}
}

var (
	errInvalidObserverKind = errors.New("unknown observer instrument kind")
)

type int64Observable struct {
	otelmetric.Int64Observable
	meter *meter
	kind  sdkmetric.InstrumentKind
	name  string
	desc  string
	unit  string

	embedded.Int64Observer
	embedded.Int64ObservableCounter
	embedded.Int64ObservableUpDownCounter
	embedded.Int64ObservableGauge
}

func newInt64Observable(m *meter, kind sdkmetric.InstrumentKind, name, desc, u string) int64Observable {
	return int64Observable{
		meter: m,
		kind:  kind,
		name:  name,
		desc:  desc,
		unit:  u,
	}
}

func (o int64Observable) Observe(val int64, opts ...otelmetric.ObserveOption) {
	c := otelmetric.NewObserveConfig(opts)
	tags := attrsToTags(c.Attributes(), o.meter.cfg.res.Attributes())
	m := metric{tags: tags, name: o.name, ivalue: val, rate: 1, globalTags: o.meter.client.tags, namespace: o.meter.client.namespace, timestamp: time.Now().Unix()}
	switch o.kind {
	case sdkmetric.InstrumentKindObservableCounter, sdkmetric.InstrumentKindObservableUpDownCounter:
		m.metricType = count
	case sdkmetric.InstrumentKindObservableGauge:
		m.metricType = gauge
	default:
		o.meter.errHandler(errInvalidObserverKind)
		return
	}
	if err := o.meter.client.send(m); err != nil {
		o.meter.errHandler(err)
	}
}

type float64Observable struct {
	otelmetric.Float64Observable
	meter *meter
	kind  sdkmetric.InstrumentKind
	name  string
	desc  string
	unit  string

	embedded.Float64Observer
	embedded.Float64ObservableCounter
	embedded.Float64ObservableUpDownCounter
	embedded.Float64ObservableGauge
}

func newFloat64Observable(m *meter, kind sdkmetric.InstrumentKind, name, desc, u string) float64Observable {
	return float64Observable{
		meter: m,
		kind:  kind,
		name:  name,
		desc:  desc,
		unit:  u,
	}
}

func (o float64Observable) Observe(val float64, opts ...otelmetric.ObserveOption) {
	c := otelmetric.NewObserveConfig(opts)
	tags := attrsToTags(c.Attributes(), o.meter.cfg.res.Attributes())
	m := metric{tags: tags, name: o.name, fvalue: val, rate: 1, globalTags: o.meter.client.tags, namespace: o.meter.client.namespace, timestamp: time.Now().Unix()}
	switch o.kind {
	case sdkmetric.InstrumentKindObservableCounter, sdkmetric.InstrumentKindObservableUpDownCounter:
		m.metricType = count
	case sdkmetric.InstrumentKindObservableGauge:
		m.metricType = gauge
	default:
		o.meter.errHandler(errInvalidObserverKind)
		return
	}
	if err := o.meter.client.send(m); err != nil {
		o.meter.errHandler(err)
	}
}

// instID are the identifying properties of a instrument.
type instID struct {
	// Name is the name of the stream.
	Name string
	// Description is the description of the stream.
	Description string
	// Kind defines the functional group of the instrument.
	Kind sdkmetric.InstrumentKind
	// Unit is the unit of the stream.
	Unit string
	// Number is the number type of the stream.
	Number string
}

func (m *meter) int64ObservableInstrument(id sdkmetric.Instrument, callbacks []otelmetric.Int64Callback) (int64Observable, error) {
	key := instID{
		Name:        id.Name,
		Description: id.Description,
		Unit:        id.Unit,
		Kind:        id.Kind,
	}
	if m.int64Observables.HasKey(key) && len(callbacks) > 0 {
		warnRepeatedObservableCallbacks(m.errHandler, id)
	}
	return m.int64Observables.Lookup(key, func() (int64Observable, error) {
		inst := newInt64Observable(m, id.Kind, id.Name, id.Description, id.Unit)

		for _, callback := range callbacks {
			m.callbacks = append(m.callbacks, func(ctx context.Context) error {
				return callback(ctx, inst)
			})
		}

		return inst, validateInstrumentName(id.Name)
	})
}

func (m *meter) float64ObservableInstrument(id sdkmetric.Instrument, callbacks []otelmetric.Float64Callback) (float64Observable, error) {
	key := instID{
		Name:        id.Name,
		Description: id.Description,
		Unit:        id.Unit,
		Kind:        id.Kind,
	}
	if m.float64Observables.HasKey(key) && len(callbacks) > 0 {
		warnRepeatedObservableCallbacks(m.errHandler, id)
	}
	return m.float64Observables.Lookup(key, func() (float64Observable, error) {
		inst := newFloat64Observable(m, id.Kind, id.Name, id.Description, id.Unit)

		for _, callback := range callbacks {
			m.callbacks = append(m.callbacks, func(ctx context.Context) error {
				return callback(ctx, inst)
			})
		}

		return inst, validateInstrumentName(id.Name)
	})
}
