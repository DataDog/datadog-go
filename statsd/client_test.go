// Copyright 2013 Ooyala, Inc.

package statsd

import (
	"io"
	"net"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

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

	client, err := New(addr)
	if err != nil {
		t.Fatal(err)
	}

	clientTest(t, server, client)
}

func TestClientWithConn(t *testing.T) {
	server, conn, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	client, err := NewWithWriter(conn)
	if err != nil {
		t.Fatal(err)
	}

	clientTest(t, server, client)
}

func clientTest(t *testing.T, server io.Reader, client *Client) {
	for _, tt := range dogstatsdTests {
		client.namespace = tt.GlobalNamespace
		client.tags = tt.GlobalTags
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
	client, err := NewBuffered(addr, bufferLength)
	if err != nil {
		t.Fatal(err)
	}

	client.namespace = "foo."
	client.tags = []string{"dd:2"}

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

	client.Event(&Event{Title: "title1", Text: "text1", Priority: Normal, AlertType: Success, Tags: []string{"tagg"}})
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

func TestClosePanic(t *testing.T) {
	c, err := New("localhost:8125")
	assert.NoError(t, err)
	c.Close()
	c.Close()
}

func TestCloseRace(t *testing.T) {
	for i := 0; i < 100; i++ {
		c, err := New("localhost:8125")
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
