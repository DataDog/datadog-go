package otel

import (
	"context"

	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/embedded"
)

type int64Inst struct {
	embedded.Int64Counter
	embedded.Int64UpDownCounter
	embedded.Int64Histogram

	name    string
	meter   *meter
	unit    string
	desc    string
	isGauge bool
}

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

	name    string
	meter   *meter
	unit    string
	desc    string
	isGauge bool
}

func (i *float64Inst) Add(_ context.Context, incr float64, options ...otelmetric.AddOption) {
	c := otelmetric.NewAddConfig(options)
	tags := attrsToTags(c.Attributes(), i.meter.res.Attributes())

	var err error
	if i.isGauge {
		err = i.meter.client.Gauge(i.name, incr, tags, 1)
	} else {
		err = i.meter.client.Count(i.name, int64(incr), tags, 1)
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
