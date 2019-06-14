// Copyright 2013 Ooyala, Inc.

package statsd_test

import (
	"io"
	"net"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/DataDog/datadog-go/statsd"
)

// Client-side entity ID injection for container tagging
const (
	entityIDEnvName = "DD_ENTITY_ID"
	entityIDTagName = "dd.internal.entity_id"
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
	{"", nil, "Gauge", "test.gauge", 1.0, nil, 1.0, "test.gauge:1|g"},
	{"", nil, "Gauge", "test.gauge", 1.0, nil, 0.999999, "test.gauge:1|g|@0.999999"},
	{"", nil, "Gauge", "test.gauge", 1.0, []string{"tagA"}, 1.0, "test.gauge:1|g|#tagA"},
	{"", nil, "Gauge", "test.gauge", 1.0, []string{"tagA", "tagB"}, 1.0, "test.gauge:1|g|#tagA,tagB"},
	{"", nil, "Gauge", "test.gauge", 1.0, []string{"tagA"}, 0.999999, "test.gauge:1|g|@0.999999|#tagA"},
	{"", nil, "Count", "test.count", int64(1), []string{"tagA"}, 1.0, "test.count:1|c|#tagA"},
	{"", nil, "Count", "test.count", int64(-1), []string{"tagA"}, 1.0, "test.count:-1|c|#tagA"},
	{"", nil, "Histogram", "test.histogram", 2.3, []string{"tagA"}, 1.0, "test.histogram:2.3|h|#tagA"},
	{"", nil, "Distribution", "test.distribution", 2.3, []string{"tagA"}, 1.0, "test.distribution:2.3|d|#tagA"},
	{"", nil, "Set", "test.set", "uuid", []string{"tagA"}, 1.0, "test.set:uuid|s|#tagA"},
	{"flubber.", nil, "Set", "test.set", "uuid", []string{"tagA"}, 1.0, "flubber.test.set:uuid|s|#tagA"},
	{"", []string{"tagC"}, "Set", "test.set", "uuid", []string{"tagA"}, 1.0, "test.set:uuid|s|#tagC,tagA"},
	{"", nil, "Count", "test.count", int64(1), []string{"hello\nworld"}, 1.0, "test.count:1|c|#helloworld"},
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

	client.Incr("ic", nil, 1)
	client.Decr("dc", nil, 1)
	client.Count("cc", 1, nil, 1)
	client.Gauge("gg", 10, nil, 1)
	client.Histogram("hh", 1, nil, 1)
	client.Distribution("dd", 1, nil, 1)
	client.Timing("tt", dur, nil, 1)
	client.Set("ss", "ss", nil, 1)

	client.Set("ss", "xx", nil, 1)
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
		`foo.ic:1|c|#dd:2`,
		`foo.dc:-1|c|#dd:2`,
		`foo.cc:1|c|#dd:2`,
		`foo.gg:10|g|#dd:2`,
		`foo.hh:1|h|#dd:2`,
		`foo.dd:1|d|#dd:2`,
		`foo.tt:0.123000|ms|#dd:2`,
		`foo.ss:ss|s|#dd:2`,
		`foo.ss:xx|s|#dd:2`,
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
		`_e{6,5}:title1|text1|p:normal|t:success|#dd:2,tagg`,
		`_e{6,5}:event1|text1|#dd:2`,
	}

	for i, res := range strings.Split(result, "\n") {
		if res != expected[i] {
			t.Errorf("Got `%s`, expected `%s`", res, expected[i])
		}
	}

}

func stringsToBytes(ss []string) [][]byte {
	bs := make([][]byte, len(ss))
	for i, s := range ss {
		bs[i] = []byte(s)
	}
	return bs
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
	}
	for i, f := range tests {
		var err error
		assertNotPanics(t, func() { err = f() })
		if err != statsd.ErrNoClient {
			t.Errorf("Test case %d: expected ErrNoClient, got %#v", i, err)
		}
	}
}

func TestEvents(t *testing.T) {
	matrix := []struct {
		event   *statsd.Event
		encoded string
	}{
		{
			statsd.NewEvent("Hello", "Something happened to my event"),
			`_e{5,30}:Hello|Something happened to my event`,
		}, {
			&statsd.Event{Title: "hi", Text: "okay", AggregationKey: "foo"},
			`_e{2,4}:hi|okay|k:foo`,
		}, {
			&statsd.Event{Title: "hi", Text: "okay", AggregationKey: "foo", AlertType: statsd.Info},
			`_e{2,4}:hi|okay|k:foo|t:info`,
		}, {
			&statsd.Event{Title: "hi", Text: "w/e", AlertType: statsd.Error, Priority: statsd.Normal},
			`_e{2,3}:hi|w/e|p:normal|t:error`,
		}, {
			&statsd.Event{Title: "hi", Text: "uh", Tags: []string{"host:foo", "app:bar"}},
			`_e{2,2}:hi|uh|#host:foo,app:bar`,
		}, {
			&statsd.Event{Title: "hi", Text: "line1\nline2", Tags: []string{"hello\nworld"}},
			`_e{2,12}:hi|line1\nline2|#helloworld`,
		},
	}

	for _, m := range matrix {
		r, err := m.event.Encode()
		if err != nil {
			t.Errorf("Error encoding: %s\n", err)
			continue
		}
		if r != m.encoded {
			t.Errorf("Expected `%s`, got `%s`\n", m.encoded, r)
		}
	}

	e := statsd.NewEvent("", "hi")
	if _, err := e.Encode(); err == nil {
		t.Errorf("Expected error on empty Title.")
	}

	e = statsd.NewEvent("hi", "")
	if _, err := e.Encode(); err == nil {
		t.Errorf("Expected error on empty Text.")
	}

	e = statsd.NewEvent("hello", "world")
	s, err := e.Encode("tag1", "tag2")
	if err != nil {
		t.Error(err)
	}
	expected := "_e{5,5}:hello|world|#tag1,tag2"
	if s != expected {
		t.Errorf("Expected %s, got %s", expected, s)
	}
	if len(e.Tags) != 0 {
		t.Errorf("Modified event in place illegally.")
	}
}

func TestServiceChecks(t *testing.T) {
	matrix := []struct {
		serviceCheck *statsd.ServiceCheck
		encoded      string
	}{
		{
			statsd.NewServiceCheck("DataCatService", statsd.Ok),
			`_sc|DataCatService|0`,
		}, {
			statsd.NewServiceCheck("DataCatService", statsd.Warn),
			`_sc|DataCatService|1`,
		}, {
			statsd.NewServiceCheck("DataCatService", statsd.Critical),
			`_sc|DataCatService|2`,
		}, {
			statsd.NewServiceCheck("DataCatService", statsd.Unknown),
			`_sc|DataCatService|3`,
		}, {
			&statsd.ServiceCheck{Name: "DataCatService", Status: statsd.Ok, Hostname: "DataStation.Cat"},
			`_sc|DataCatService|0|h:DataStation.Cat`,
		}, {
			&statsd.ServiceCheck{Name: "DataCatService", Status: statsd.Ok, Hostname: "DataStation.Cat", Message: "Here goes valuable message"},
			`_sc|DataCatService|0|h:DataStation.Cat|m:Here goes valuable message`,
		}, {
			&statsd.ServiceCheck{Name: "DataCatService", Status: statsd.Ok, Hostname: "DataStation.Cat", Message: "Here are some cyrillic chars: к л м н о п р с т у ф х ц ч ш"},
			`_sc|DataCatService|0|h:DataStation.Cat|m:Here are some cyrillic chars: к л м н о п р с т у ф х ц ч ш`,
		}, {
			&statsd.ServiceCheck{Name: "DataCatService", Status: statsd.Ok, Hostname: "DataStation.Cat", Message: "Here goes valuable message", Tags: []string{"host:foo", "app:bar"}},
			`_sc|DataCatService|0|h:DataStation.Cat|#host:foo,app:bar|m:Here goes valuable message`,
		}, {
			&statsd.ServiceCheck{Name: "DataCatService", Status: statsd.Ok, Hostname: "DataStation.Cat", Message: "Here goes \n that should be escaped", Tags: []string{"host:foo", "app:b\nar"}},
			`_sc|DataCatService|0|h:DataStation.Cat|#host:foo,app:bar|m:Here goes \n that should be escaped`,
		}, {
			&statsd.ServiceCheck{Name: "DataCatService", Status: statsd.Ok, Hostname: "DataStation.Cat", Message: "Here goes m: that should be escaped", Tags: []string{"host:foo", "app:bar"}},
			`_sc|DataCatService|0|h:DataStation.Cat|#host:foo,app:bar|m:Here goes m\: that should be escaped`,
		},
	}

	for _, m := range matrix {
		r, err := m.serviceCheck.Encode()
		if err != nil {
			t.Errorf("Error encoding: %s\n", err)
			continue
		}
		if r != m.encoded {
			t.Errorf("Expected `%s`, got `%s`\n", m.encoded, r)
		}
	}

	sc := statsd.NewServiceCheck("", statsd.Ok)
	if _, err := sc.Encode(); err == nil {
		t.Errorf("Expected error on empty Name.")
	}

	sc = statsd.NewServiceCheck("sc", statsd.ServiceCheckStatus(5))
	if _, err := sc.Encode(); err == nil {
		t.Errorf("Expected error on invalid status value.")
	}

	sc = statsd.NewServiceCheck("hello", statsd.Warn)
	s, err := sc.Encode("tag1", "tag2")
	if err != nil {
		t.Error(err)
	}
	expected := "_sc|hello|1|#tag1,tag2"
	if s != expected {
		t.Errorf("Expected %s, got %s", expected, s)
	}
	if len(sc.Tags) != 0 {
		t.Errorf("Modified serviceCheck in place illegally.")
	}
}

func TestEntityID(t *testing.T) {
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
