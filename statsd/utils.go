package statsd

import (
	"unsafe"

	"github.com/DataDog/datadog-go/v5/statsd/fastrand"
)

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

func joinTags(tags []string) string {
	const tagSeparator = ','
	switch len(tags) {
	case 0:
		return ""
	case 1:
		return tags[0]
	}

	n := len(tags) - 1
	for _, tag := range tags {
		n += len(tag)
	}
	b := make([]byte, 0, n)
	b = append(b, tags[0]...)
	for _, s := range tags[1:] {
		b = append(append(b, tagSeparator), s...)
	}
	return *(*string)(unsafe.Pointer(&b))
}
