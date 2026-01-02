package main

import (
	"strings"
	"testing"

	"github.com/DataDog/datadog-go/v5/statsd"
)

func TestStatsDNewWithWriter(t *testing.T) {
	// Create a mock client using NewWithWriter to avoid actual network calls
	writer := &mockWriter{
		data: make([]string, 0),
	}

	client, err := statsd.NewWithWriter(writer,
		statsd.WithTags([]string{"env:prod", "service:myservice"}),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Logf("Error closing client: %v", err)
		}
	}()

	// Send the same metric as in runExample
	err = client.Histogram("my.metrics", 21, []string{"tag1", "tag2:value"}, 1)
	if err != nil {
		t.Fatalf("Failed to send histogram: %v", err)
	}

	// Flush to ensure metrics are written
	err = client.Flush()
	if err != nil {
		t.Fatalf("Failed to flush client: %v", err)
	}

	// Verify that data was written
	if len(writer.data) == 0 {
		t.Fatal("Expected metrics to be written, but got no data")
	}

	// Verify the metric format (DogStatsD format)
	// Expected format: my.metrics:21|h|#env:prod,service:myservice,tag1,tag2:value
	found := false
	for _, metric := range writer.data {
		if containsHistogramMetric(metric, "my.metrics", "21", []string{"env:prod", "service:myservice", "tag1", "tag2:value"}) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected to find histogram metric 'my.metrics' with value 21 and tags, got: %v", writer.data)
	}
}

func TestRunExample(t *testing.T) {
	if err := runExample(); err != nil {
		t.Logf("Error closing client: %v", err)
	}
}

// mockWriter implements io.WriteCloser for testing
type mockWriter struct {
	data   []string
	closed bool
}

func (m *mockWriter) Write(p []byte) (n int, err error) {
	m.data = append(m.data, string(p))
	return len(p), nil
}

func (m *mockWriter) Close() error {
	m.closed = true
	return nil
}

// containsHistogramMetric checks if a metric string contains the expected histogram
func containsHistogramMetric(metric, name, value string, tags []string) bool {
	// Check if metric is non-empty
	if len(metric) == 0 {
		return false
	}

	// Check if it contains the metric name
	if !strings.Contains(metric, name) {
		return false
	}

	// Check if it contains the value
	if !strings.Contains(metric, value) {
		return false
	}

	// Check if it's a histogram metric (contains |h)
	if !strings.Contains(metric, "|h") {
		return false
	}

	// Check if it contains the tags
	for _, tag := range tags {
		if !strings.Contains(metric, tag) {
			return false
		}
	}

	return true
}
