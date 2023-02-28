package statsd

import "github.com/DataDog/datadog-go/v5/statsd/fastrand"

func shouldSample(rate float64) bool {
	return rate >= 1 || fastrand.Float64() <= rate
}

func copySlice(src []string) []string {
	if src == nil {
		return nil
	}

	c := make([]string, len(src))
	copy(c, src)
	return c
}
