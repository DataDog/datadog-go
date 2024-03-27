package statsd

import (
	"context"
	"errors"

	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/embedded"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

type int64Inst struct {
	embedded.Int64Counter
	embedded.Int64UpDownCounter
	embedded.Int64Histogram

	meter *meter
	instID
}

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

	meter *meter
	instID
}

func (i *float64Inst) Add(_ context.Context, incr float64, options ...otelmetric.AddOption) {
	c := otelmetric.NewAddConfig(options)
	tags := attrsToTags(c.Attributes(), i.meter.cfg.res.Attributes())

	var err error
	if i.Kind == sdkmetric.InstrumentKindUpDownCounter {
		err = i.meter.client.Gauge(i.Name, incr, tags, 1)
	} else {
		err = i.meter.client.Count(i.Name, int64(incr), tags, 1)
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
	instID

	embedded.Int64Observer
	embedded.Int64ObservableCounter
	embedded.Int64ObservableUpDownCounter
	embedded.Int64ObservableGauge
}

func newInt64Observable(m *meter, id instID) int64Observable {
	return int64Observable{
		meter:  m,
		instID: id,
	}
}

func (o int64Observable) Observe(val int64, opts ...otelmetric.ObserveOption) {
	c := otelmetric.NewObserveConfig(opts)
	tags := attrsToTags(c.Attributes(), o.meter.cfg.res.Attributes())
	var err error
	switch {
	case o.Unit == "s":
		val *= 1000
		fallthrough
	case o.Unit == "ms":
		err = o.meter.client.TimeInMilliseconds(o.Name, float64(val), tags, 1)
	case o.Kind == sdkmetric.InstrumentKindObservableCounter || o.Kind == sdkmetric.InstrumentKindObservableUpDownCounter:
		err = o.meter.client.Count(o.Name, val, tags, 1)
	case o.Kind == sdkmetric.InstrumentKindObservableGauge:
		err = o.meter.client.Gauge(o.Name, float64(val), tags, 1)
	default:
		err = errInvalidObserverKind
	}
	if err != nil {
		o.meter.errHandler(err)
		return
	}
}

type float64Observable struct {
	otelmetric.Float64Observable
	meter *meter
	instID

	embedded.Float64Observer
	embedded.Float64ObservableCounter
	embedded.Float64ObservableUpDownCounter
	embedded.Float64ObservableGauge
}

func newFloat64Observable(m *meter, id instID) float64Observable {
	return float64Observable{
		meter:  m,
		instID: id,
	}
}

func (o float64Observable) Observe(val float64, opts ...otelmetric.ObserveOption) {
	c := otelmetric.NewObserveConfig(opts)
	tags := attrsToTags(c.Attributes(), o.meter.cfg.res.Attributes())
	var err error
	switch {
	case o.Unit == "s":
		val *= 1000
		fallthrough
	case o.Unit == "ms":
		err = o.meter.client.TimeInMilliseconds(o.Name, val, tags, 1)
	case o.Kind == sdkmetric.InstrumentKindObservableCounter || o.Kind == sdkmetric.InstrumentKindObservableUpDownCounter:
		err = o.meter.client.Count(o.Name, int64(val), tags, 1)
	case o.Kind == sdkmetric.InstrumentKindObservableGauge:
		err = o.meter.client.Gauge(o.Name, val, tags, 1)
	default:
		err = errInvalidObserverKind
	}
	if err != nil {
		o.meter.errHandler(err)
		return
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
		inst := newInt64Observable(m, key)

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
		inst := newFloat64Observable(m, key)

		for _, callback := range callbacks {
			m.callbacks = append(m.callbacks, func(ctx context.Context) error {
				return callback(ctx, inst)
			})
		}

		return inst, validateInstrumentName(id.Name)
	})
}
