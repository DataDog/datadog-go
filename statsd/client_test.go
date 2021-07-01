// Copyright 2013 Ooyala, Inc.

package statsd_test

import (
	"io"
	"net"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DataDog/datadog-go/statsd"
	"github.com/stretchr/testify/assert"
)

var dogstatsdTests = []struct {
	GlobalNamespace string
	GlobalTags      []string
	Method          string
	Metric          string
	Value           interface{}
	Tags            []string
	Rate            float64
	Expected        string
}{
	{"", nil, "Gauge", "test.gauge", 1.0, nil, 1.0, "test.gauge:1|g\n"},
	{"", nil, "Gauge", "test.gauge", 1.0, nil, 0.999999, "test.gauge:1|g|@0.999999\n"},
	{"", nil, "Gauge", "test.gauge", 1.0, []string{"tagA"}, 1.0, "test.gauge:1|g|#tagA\n"},
	{"", nil, "Gauge", "test.gauge", 1.0, []string{"tagA", "tagB"}, 1.0, "test.gauge:1|g|#tagA,tagB\n"},
	{"", nil, "Gauge", "test.gauge", 1.0, []string{"tagA"}, 0.999999, "test.gauge:1|g|@0.999999|#tagA\n"},
	{"", nil, "Count", "test.count", int64(1), []string{"tagA"}, 1.0, "test.count:1|c|#tagA\n"},
	{"", nil, "Count", "test.count", int64(-1), []string{"tagA"}, 1.0, "test.count:-1|c|#tagA\n"},
	{"", nil, "Histogram", "test.histogram", 2.3, []string{"tagA"}, 1.0, "test.histogram:2.3|h|#tagA\n"},
	{"", nil, "Distribution", "test.distribution", 2.3, []string{"tagA"}, 1.0, "test.distribution:2.3|d|#tagA\n"},
	{"", nil, "Set", "test.set", "uuid", []string{"tagA"}, 1.0, "test.set:uuid|s|#tagA\n"},
	{"flubber.", nil, "Set", "test.set", "uuid", []string{"tagA"}, 1.0, "flubber.test.set:uuid|s|#tagA\n"},
	{"", []string{"tagC"}, "Set", "test.set", "uuid", []string{"tagA"}, 1.0, "test.set:uuid|s|#tagC,tagA\n"},
	{"", nil, "Count", "test.count", int64(1), []string{"hello\nworld"}, 1.0, "test.count:1|c|#helloworld\n"},
}

func assertNotPanics(t *testing.T, f func()) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatal(r)
		}
	}()
	f()
}

func TestClientUDP(t *testing.T) {
	addr := "localhost:1201"
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		t.Fatal(err)
	}

	server, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	client, err := statsd.New(addr)
	if err != nil {
		t.Fatal(err)
	}

	clientTest(t, server, client)
}

type statsdWriterWrapper struct {
	io.WriteCloser
}

func (statsdWriterWrapper) SetWriteTimeout(time.Duration) error {
	return nil
}

func TestClientWithConn(t *testing.T) {
	server, conn, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	client, err := statsd.NewWithWriter(statsdWriterWrapper{conn})
	if err != nil {
		t.Fatal(err)
	}

	clientTest(t, server, client)
}

func clientTest(t *testing.T, server io.Reader, client *statsd.Client) {
	for _, tt := range dogstatsdTests {
		client.Namespace = tt.GlobalNamespace
		client.Tags = tt.GlobalTags
		method := reflect.ValueOf(client).MethodByName(tt.Method)
		e := method.Call([]reflect.Value{
			reflect.ValueOf(tt.Metric),
			reflect.ValueOf(tt.Value),
			reflect.ValueOf(tt.Tags),
			reflect.ValueOf(tt.Rate)})[0]
		errInter := e.Interface()
		if errInter != nil {
			t.Fatal(errInter.(error))
		}

		bytes := make([]byte, 1024)
		n, err := server.Read(bytes)
		if err != nil {
			t.Fatal(err)
		}
		message := bytes[:n]
		if string(message) != tt.Expected {
			t.Errorf("Expected: %s. Actual: %s", tt.Expected, string(message))
		}
	}
}

func TestBufferedClient(t *testing.T) {
	addr := "localhost:1201"
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		t.Fatal(err)
	}

	server, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	bufferLength := 9
	client, err := statsd.NewBuffered(addr, bufferLength)
	if err != nil {
		t.Fatal(err)
	}

	client.Namespace = "foo."
	client.Tags = []string{"dd:2"}

	dur, _ := time.ParseDuration("123us")

	client.Incr("ab", nil, 1)
	client.Decr("ab", nil, 1)
	client.Count("ab", 1, nil, 1)
	client.Gauge("ab", 10, nil, 1)
	client.Histogram("ab", 1, nil, 1)
	client.Distribution("ab", 1, nil, 1)
	client.Timing("ab", dur, nil, 1)
	client.Set("ab", "ss", nil, 1)

	client.Set("ab", "xx", nil, 1)
	client.Flush()
	if err != nil {
		t.Errorf("Error sending: %s", err)
	}

	buffer := make([]byte, 4096)
	n, err := io.ReadAtLeast(server, buffer, 1)
	result := string(buffer[:n])

	if err != nil {
		t.Error(err)
	}

	expected := []string{
		`foo.ab:1|c|#dd:2`,
		`foo.ab:-1|c|#dd:2`,
		`foo.ab:1|c|#dd:2`,
		`foo.ab:10|g|#dd:2`,
		`foo.ab:1|h|#dd:2`,
		`foo.ab:1|d|#dd:2`,
		`foo.ab:0.123000|ms|#dd:2`,
		`foo.ab:ss|s|#dd:2`,
		`foo.ab:xx|s|#dd:2`,
		``,
	}

	for i, res := range strings.Split(result, "\n") {
		if res != expected[i] {
			t.Errorf("Got `%s`, expected `%s`", res, expected[i])
		}
	}

	client.Event(&statsd.Event{Title: "title1", Text: "text1", Priority: statsd.Normal, AlertType: statsd.Success, Tags: []string{"tagg"}})
	client.SimpleEvent("event1", "text1")
	err = client.Flush()

	if err != nil {
		t.Errorf("Error sending: %s", err)
	}

	buffer = make([]byte, 1024)
	n, err = io.ReadAtLeast(server, buffer, 1)
	result = string(buffer[:n])

	if err != nil {
		t.Error(err)
	}

	if n == 0 {
		t.Errorf("Read 0 bytes but expected more.")
	}

	expected = []string{
		"_e{6,5}:title1|text1|p:normal|t:success|#dd:2,tagg",
		"_e{6,5}:event1|text1|#dd:2",
		"",
	}

	arr := strings.Split(result, "\n")
	_ = arr
	for i, res := range strings.Split(result, "\n") {
		if res != expected[i] {
			t.Errorf("Got `%s`, expected `%s`", res, expected[i])
		}
	}

}

func TestNilError(t *testing.T) {
	var c *statsd.Client
	tests := []func() error{
		func() error { return c.SetWriteTimeout(0) },
		func() error { return c.Flush() },
		func() error { return c.Close() },
		func() error { return c.Count("", 0, nil, 1) },
		func() error { return c.Incr("", nil, 1) },
		func() error { return c.Decr("", nil, 1) },
		func() error { return c.Histogram("", 0, nil, 1) },
		func() error { return c.Distribution("", 0, nil, 1) },
		func() error { return c.Gauge("", 0, nil, 1) },
		func() error { return c.Set("", "", nil, 1) },
		func() error { return c.Timing("", time.Second, nil, 1) },
		func() error { return c.TimeInMilliseconds("", 1, nil, 1) },
		func() error { return c.Event(statsd.NewEvent("", "")) },
		func() error { return c.SimpleEvent("", "") },
		func() error { return c.ServiceCheck(statsd.NewServiceCheck("", statsd.Ok)) },
		func() error { return c.SimpleServiceCheck("", statsd.Ok) },
		func() error {
			_, err := statsd.CloneWithExtraOptions(nil, statsd.WithChannelMode())
			return err
		},
	}
	for i, f := range tests {
		var err error
		assertNotPanics(t, func() { err = f() })
		if err != statsd.ErrNoClient {
			t.Errorf("Test case %d: expected ErrNoClient, got %#v", i, err)
		}
	}
}

func TestEntityID(t *testing.T) {
	entityIDEnvName := "DD_ENTITY_ID"
	initialValue, initiallySet := os.LookupEnv(entityIDEnvName)
	if initiallySet {
		defer os.Setenv(entityIDEnvName, initialValue)
	} else {
		defer os.Unsetenv(entityIDEnvName)
	}

	// Set to a valid value
	os.Setenv(entityIDEnvName, "testing")
	client, err := statsd.New("localhost:8125")
	if err != nil {
		t.Fatal(err)
	}
	if len(client.Tags) != 1 {
		t.Errorf("Expecting one tag, got %d", len(client.Tags))
	}
	if client.Tags[0] != "dd.internal.entity_id:testing" {
		t.Errorf("Bad tag value, got %s", client.Tags[0])
	}

	// Set to empty string
	os.Setenv(entityIDEnvName, "")
	client, err = statsd.New("localhost:8125")
	if err != nil {
		t.Fatal(err)
	}
	if len(client.Tags) != 0 {
		t.Errorf("Expecting empty default tags, got %v", client.Tags)
	}

	// Unset
	os.Unsetenv(entityIDEnvName)
	client, err = statsd.New("localhost:8125")
	if err != nil {
		t.Fatal(err)
	}
	if len(client.Tags) != 0 {
		t.Errorf("Expecting empty default tags, got %v", client.Tags)
	}
}

var (
	ddEnvName     = "DD_ENV"
	ddServiceName = "DD_SERVICE"
	ddVersionName = "DD_VERSION"
)

func TestDDEnvServiceVersionSet(t *testing.T) {
	for _, tt := range []struct {
		DDEnv     string
		DDService string
		DDVersion string
		Expected  []string
	}{
		{"", "", "", []string{}},
		{"prod", "", "", []string{"env:prod"}},
		{"prod", "dog", "", []string{"env:prod", "service:dog"}},
		{"prod", "dog", "abc123", []string{"env:prod", "service:dog", "version:abc123"}},
	} {
		for _, t := range []string{ddEnvName, ddServiceName, ddVersionName} {
			initialValue, initiallySet := os.LookupEnv(t)
			if initiallySet {
				defer os.Setenv(t, initialValue)
			} else {
				defer os.Unsetenv(t)
			}
		}
		os.Setenv(ddEnvName, tt.DDEnv)
		os.Setenv(ddServiceName, tt.DDService)
		os.Setenv(ddVersionName, tt.DDVersion)
		client, err := statsd.New("localhost:8125")
		if err != nil {
			t.Fatal(err)
		}
		// Keep the ordering of global tags consistent.
		sort.Strings(client.Tags)
		assert.Equal(t, tt.Expected, client.Tags)
	}
}

func TestDDEnvServiceVersionTagsEmitted(t *testing.T) {
	for _, t := range []string{ddEnvName, ddServiceName, ddVersionName} {
		initialValue, initiallySet := os.LookupEnv(t)
		if initiallySet {
			defer os.Setenv(t, initialValue)
		} else {
			defer os.Unsetenv(t)
		}
	}
	os.Setenv(ddEnvName, "prod")
	os.Setenv(ddServiceName, "dog")
	os.Setenv(ddVersionName, "abc123")
	addr := "localhost:1201"
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		t.Fatal(err)
	}
	server, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	for _, tt := range []struct {
		Tags       []string
		GlobalTags []string
		Expected   string
	}{
		{nil, nil, "test.count:100|c|#env:prod,service:dog,version:abc123\n"},
		{[]string{"env:staging", "service:cat", "custom_tag"}, nil, "test.count:100|c|#env:prod,service:dog,version:abc123,env:staging,service:cat,custom_tag\n"},
		{nil, []string{"version:def456", "custom_tag_two"}, "test.count:100|c|#custom_tag_two,env:prod,service:dog,version:abc123,version:def456\n"},
		{[]string{"env:staging", "service:cat", "custom_tag"}, []string{"version:def456", "custom_tag_two"}, "test.count:100|c|#custom_tag_two,env:prod,service:dog,version:abc123,version:def456,env:staging,service:cat,custom_tag\n"},
	} {
		client, err := statsd.New(addr, statsd.WithTags(tt.GlobalTags))
		if err != nil {
			t.Fatal(err)
		}
		// Keep the ordering of global tags consistent.
		sort.Strings(client.Tags)
		client.Count("test.count", 100, tt.Tags, 1.0)
		err = client.Flush()
		if err != nil {
			t.Errorf("Error sending: %s", err)
		}
		buffer := make([]byte, 1024)
		n, err := io.ReadAtLeast(server, buffer, 1)
		if err != nil {
			t.Errorf("ReadAtLeast: %s", err)
		}
		result := string(buffer[:n])
		if result != tt.Expected {
			t.Errorf("Flushed metric incorrect; expected %s but got %s", tt.Expected, result)
		}
	}
}

func TestClosePanic(t *testing.T) {
	c, err := statsd.New("localhost:8125")
	assert.NoError(t, err)
	c.Close()
	c.Close()
}

func TestCloseRace(t *testing.T) {
	for i := 0; i < 100; i++ {
		c, err := statsd.New("localhost:8125")
		assert.NoError(t, err)
		start := make(chan struct{})
		var wg sync.WaitGroup
		for j := 0; j < 100; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				c.Close()
			}()
		}
		close(start)
		wg.Wait()
	}
}
