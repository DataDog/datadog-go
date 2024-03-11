package statsd

import (
	"fmt"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

func validateInstrumentName(name string) error {
	if len(name) == 0 {
		return fmt.Errorf("%w: %s: is empty", sdkmetric.ErrInstrumentName, name)
	}
	if len(name) > 255 {
		return fmt.Errorf("%w: %s: longer than 255 characters", sdkmetric.ErrInstrumentName, name)
	}
	if !isAlpha([]rune(name)[0]) {
		return fmt.Errorf("%w: %s: must start with a letter", sdkmetric.ErrInstrumentName, name)
	}
	if len(name) == 1 {
		return nil
	}
	for _, c := range name[1:] {
		if !isAlphanumeric(c) && c != '_' && c != '.' && c != '-' && c != '/' {
			return fmt.Errorf("%w: %s: must only contain [A-Za-z0-9_.-/]", sdkmetric.ErrInstrumentName, name)
		}
	}
	return nil
}

func isAlpha(c rune) bool {
	return ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}

func isAlphanumeric(c rune) bool {
	return isAlpha(c) || ('0' <= c && c <= '9')
}
