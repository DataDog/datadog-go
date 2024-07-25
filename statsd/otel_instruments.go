package statsd

import (
	"context"
	"sync/atomic"

	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/embedded"
)

type int64Inst struct {
	embedded.Int64Counter
	embedded.Int64UpDownCounter
	embedded.Int64Histogram
	embedded.Int64Gauge

	name    string
	meter   *meter
	unit    string
	desc    string
	isGauge bool
}

var _ otelmetric.Int64Counter = (*int64Inst)(nil)
var _ otelmetric.Int64UpDownCounter = (*int64Inst)(nil)
var _ otelmetric.Int64Histogram = (*int64Inst)(nil)
var _ otelmetric.Int64Gauge = (*int64Inst)(nil)

func (i *int64Inst) Add(_ context.Context, incr int64, options ...otelmetric.AddOption) {
	c := otelmetric.NewAddConfig(options)
	tags := attrsToTags(c.Attributes(), i.meter.res.Attributes())

	var err error
	if i.isGauge {
		err = i.meter.client.Gauge(i.name, float64(incr), tags, 1)
	} else {
		err = i.meter.client.Count(i.name, incr, tags, 1)
	}
	if err != nil {
		i.meter.errHandler(err)
	}
}

func (i *int64Inst) Record(_ context.Context, value int64, options ...otelmetric.RecordOption) {
	c := otelmetric.NewRecordConfig(options)
	tags := attrsToTags(c.Attributes(), i.meter.res.Attributes())
	if err := i.meter.client.Histogram(i.name, float64(value), tags, 1); err != nil {
		i.meter.errHandler(err)
	}
}

type float64Inst struct {
	embedded.Float64Counter
	embedded.Float64UpDownCounter
	embedded.Float64Histogram
	embedded.Float64Gauge

	name    string
	meter   *meter
	unit    string
	desc    string
	isGauge bool
}

var _ otelmetric.Float64Counter = (*float64Inst)(nil)
var _ otelmetric.Float64UpDownCounter = (*float64Inst)(nil)
var _ otelmetric.Float64Histogram = (*float64Inst)(nil)
var _ otelmetric.Float64Gauge = (*float64Inst)(nil)

func (i *float64Inst) Add(_ context.Context, incr float64, options ...otelmetric.AddOption) {
	c := otelmetric.NewAddConfig(options)
	tags := attrsToTags(c.Attributes(), i.meter.res.Attributes())

	var err error
	if i.isGauge {
		err = i.meter.client.Gauge(i.name, incr, tags, 1)
	} else {
		atomic.AddUint64(&i.meter.client.telemetry.totalMetricsCount, 1)
		// TODO: Possible to maybe send this as a float instead of casting?
		err = i.meter.client.send(metric{metricType: count, name: i.name, fvalue: incr, tags: tags, rate: 1, globalTags: i.meter.client.tags, namespace: i.meter.client.namespace})
		// If not, we use this.
		//err = i.meter.client.Count(i.name, int64(incr), tags, 1)
	}
	if err != nil {
		i.meter.errHandler(err)
	}
}

func (i *float64Inst) Record(_ context.Context, value float64, options ...otelmetric.RecordOption) {
	c := otelmetric.NewRecordConfig(options)
	tags := attrsToTags(c.Attributes(), i.meter.res.Attributes())
	if err := i.meter.client.Histogram(i.name, value, tags, 1); err != nil {
		i.meter.errHandler(err)
	}
}

type observable[T int64 | float64] struct {
}
