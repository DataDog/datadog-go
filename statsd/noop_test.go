package statsd

import (
	"testing"
	"time"
)

func TestNoOpClient(t *testing.T) {
	c := NoOpClient{}
	tags := []string{"a:b"}

	if c.Gauge("asd", 123.4, tags, 56.0) != nil {
		t.Error("Gauge output not nil")
	}

	if c.Count("asd", 1234, tags, 56.0) != nil {
		t.Error("Count output not nil")
	}

	if c.Histogram("asd", 12.34, tags, 56.0) != nil {
		t.Error("Histogram output not nil")
	}

	if c.Distribution("asd", 1.234, tags, 56.0) != nil {
		t.Error("Distribution output not nil")
	}

	if c.Decr("asd", tags, 56.0) != nil {
		t.Error("Decr output not nil")
	}

	if c.Incr("asd", tags, 56.0) != nil {
		t.Error("Incr output not nil")
	}

	if c.Set("asd", "asd", tags, 56.0) != nil {
		t.Error("Set output not nil")
	}

	if c.Timing("asd", time.Second, tags, 56.0) != nil {
		t.Error("Timing output not nil")
	}

	if c.TimeInMilliseconds("asd", 1234.5, tags, 56.0) != nil {
		t.Error("TimeInMilliseconds output not nil")
	}

	if c.Event(nil) != nil {
		t.Error("Event output not nil")
	}

	if c.SimpleEvent("asd", "zxc") != nil {
		t.Error("SimpleEvent output not nil")
	}

	if c.ServiceCheck(nil) != nil {
		t.Error("ServiceCheck output not nil")
	}

	if c.SimpleServiceCheck("asd", Ok) != nil {
		t.Error("SimpleServiceCheck output not nil")
	}
}
