package statsd

import (
	"fmt"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

func warnRepeatedObservableCallbacks(handler ErrorHandler, id sdkmetric.Instrument) {
	handler(fmt.Errorf("Repeated observable instrument creation with callbacks. Ignoring new callbacks. Use meter.RegisterCallback and Registration.Unregister to manage callbacks. Instrument{Name: %q, Description: %q, Kind: %q, Unit: %q}",
		id.Name, id.Description, "InstrumentKind"+id.Kind.String(), id.Unit,
	))
}
