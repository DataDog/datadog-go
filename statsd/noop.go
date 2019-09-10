package statsd

import "time"

// NoOpClient is a statsd client that does nothing. Can be useful in testing
// situations for library users.
type NoOpClient struct{}

func (n *NoOpClient) Gauge(name string, value float64, tags []string, rate float64) error {
	return nil
}

func (n *NoOpClient) Count(name string, value int64, tags []string, rate float64) error {
	return nil
}

func (n *NoOpClient) Histogram(name string, value float64, tags []string, rate float64) error {
	return nil
}

func (n *NoOpClient) Distribution(name string, value float64, tags []string, rate float64) error {
	return nil
}

func (n *NoOpClient) Decr(name string, tags []string, rate float64) error {
	return nil
}

func (n *NoOpClient) Incr(name string, tags []string, rate float64) error {
	return nil
}

func (n *NoOpClient) Set(name string, value string, tags []string, rate float64) error {
	return nil
}

func (n *NoOpClient) Timing(name string, value time.Duration, tags []string, rate float64) error {
	return nil
}

func (n *NoOpClient) TimeInMilliseconds(name string, value float64, tags []string, rate float64) error {
	return nil
}

func (n *NoOpClient) Event(e *Event) error {
	return nil
}

func (n *NoOpClient) SimpleEvent(title, text string) error {
	return nil
}

func (n *NoOpClient) ServiceCheck(sc *ServiceCheck) error {
	return nil
}

func (n *NoOpClient) SimpleServiceCheck(name string, status ServiceCheckStatus) error {
	return nil
}
