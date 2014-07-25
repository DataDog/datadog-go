// Copyright 2013 Ooyala, Inc.

/*
Package dogstatsd provides a Go DogStatsD client. DogStatsD extends StatsD - adding tags and
histograms. Refer to http://docs.datadoghq.com/guides/dogstatsd/ for information about DogStatsD.

Example Usage:
		// Create the client
		c, err := dogstatsd.New("127.0.0.1:8125")
		if err != nil {
			log.Fatal(err)
		}
		// Prefix every metric with the app name
		c.Namespace = "flubber."
		// Send the EC2 availability zone as a tag with every metric
		append(c.Tags, "us-east-1a")
		err = c.Gauge("request.duration", 1.2, nil, 1)

dogstatsd is based on go-statsd-client.
*/
package dogstatsd

import (
	"fmt"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"
)

// StatsdClient is an interface for statsd client implementations to follow.
type StatsdClient interface {
	Gauge(name string, value float64, tags []string, rate float64) error
	Count(name string, value int64, tags []string, rate float64) error
	Histogram(name string, value float64, tags []string, rate float64) error
	Set(name, value string, tags []string, rate float64) error
	Close() error
}

type BufferingClient struct {
	conn net.Conn
	// Namespace to prepend to all statsd calls
	Namespace string
	// Tags are global tags to be added to every statsd call
	Tags []string
	// BufferLength is the length of the buffer in commands.
	BufferLength int
	flushTime    time.Duration
	commands     []string
	stop         bool
	sync.Mutex
}

// NewBufferingClient returns a buffering client with a default FlushTime of 10msec.
func NewBufferingClient(addr string, bufferLength int) (*BufferingClient, error) {
	conn, err := net.Dial("udp", addr)
	if err != nil {
		return nil, err
	}
	client := &BufferingClient{
		conn:         conn,
		commands:     make([]string, 0, bufferLength),
		BufferLength: bufferLength,
		// flushTime represents the longest delay a message will stay in the buffer
		flushTime: time.Millisecond * 100,
	}
	go client.flush()
	return client, nil
}

// Gauge measures the value of a metric at a particular time
func (b *BufferingClient) Gauge(name string, value float64, tags []string, rate float64) error {
	stat := fmt.Sprintf("%f|g", value)
	return b.append(name, stat, tags, rate)
}

// Count tracks how many times something happened per second
func (b *BufferingClient) Count(name string, value int64, tags []string, rate float64) error {
	stat := fmt.Sprintf("%d|c", value)
	return b.append(name, stat, tags, rate)
}

// Histogram tracks the statistical distribution of a set of values
func (b *BufferingClient) Histogram(name string, value float64, tags []string, rate float64) error {
	stat := fmt.Sprintf("%f|h", value)
	return b.append(name, stat, tags, rate)
}

// Sets count the number of unique elements in a group
func (b *BufferingClient) Set(name, value string, tags []string, rate float64) error {
	stat := fmt.Sprintf("%s|s", value)
	return b.append(name, stat, tags, rate)
}

func (b *BufferingClient) Close() error {
	if b == nil {
		return nil
	}
	b.stop = true
	return b.conn.Close()
}

func (b *BufferingClient) flush() {
	for _ = range time.Tick(b.flushTime) {
		if b.stop {
			return
		}
		b.Lock()
		if len(b.commands) > 0 {
			// FIXME: eating error here
			b.send()
		}
		b.Unlock()
	}
}

// sends what is in the buffer over the socket.  clients calling send must have
// the lock.  resets the command buffer to empty
func (b *BufferingClient) send() error {
	data := strings.Join(b.commands, "\n")
	_, err := b.conn.Write([]byte(data))
	b.commands = make([]string, 0, b.BufferLength)
	return err
}

// append appends a command to the internal command buffer.  If this fills the
// buffer, then a send is done and fullsend is set to true, signalling the flush
// routine to not flush.  Rate limiting is done on append.
func (b *BufferingClient) append(name, value string, tags []string, rate float64) error {
	if b == nil {
		return nil
	}
	if rate < 1 {
		if rand.Float64() < rate {
			value = fmt.Sprintf("%s|@%f", value, rate)
		} else {
			return nil
		}
	}

	if b.Namespace != "" {
		name = b.Namespace + name
	}

	tags = append(b.Tags, tags...)
	if len(tags) > 0 {
		value = fmt.Sprintf("%s|#%s", value, strings.Join(tags, ","))
	}

	data := fmt.Sprintf("%s:%s", name, value)
	b.Lock()
	b.commands = append(b.commands, data)
	// if we should flush, lets do it
	if len(b.commands) == b.BufferLength {
		if err := b.send(); err != nil {
			return err
		}
	}
	b.Unlock()
	return nil
}

// Client is the default dogstatsd client.  It always sends out stats to dogstatsd
// as requested.
type Client struct {
	conn net.Conn
	// Namespace to prepend to all statsd calls
	Namespace string
	// Tags are global tags to be added to every statsd call
	Tags []string
}

// New returns a pointer to a new Client and an error.
// addr must have the format "hostname:port"
func New(addr string) (*Client, error) {
	conn, err := net.Dial("udp", addr)
	if err != nil {
		return nil, err
	}
	client := &Client{conn: conn}
	return client, nil
}

// send handles sampling and sends the message over UDP. It also adds global namespace prefixes and tags.
func (c *Client) send(name string, value string, tags []string, rate float64) error {
	if c == nil {
		return nil
	}
	if rate < 1 {
		if rand.Float64() < rate {
			value = fmt.Sprintf("%s|@%f", value, rate)
		} else {
			return nil
		}
	}

	if c.Namespace != "" {
		name = fmt.Sprintf("%s%s", c.Namespace, name)
	}

	tags = append(c.Tags, tags...)
	if len(tags) > 0 {
		value = fmt.Sprintf("%s|#%s", value, strings.Join(tags, ","))
	}

	data := fmt.Sprintf("%s:%s", name, value)
	_, err := c.conn.Write([]byte(data))
	return err
}

// Gauges measure the value of a metric at a particular time
func (c *Client) Gauge(name string, value float64, tags []string, rate float64) error {
	stat := fmt.Sprintf("%f|g", value)
	return c.send(name, stat, tags, rate)
}

// Counters track how many times something happened per second
func (c *Client) Count(name string, value int64, tags []string, rate float64) error {
	stat := fmt.Sprintf("%d|c", value)
	return c.send(name, stat, tags, rate)
}

// Histograms track the statistical distribution of a set of values
func (c *Client) Histogram(name string, value float64, tags []string, rate float64) error {
	stat := fmt.Sprintf("%f|h", value)
	return c.send(name, stat, tags, rate)
}

// Sets count the number of unique elements in a group
func (c *Client) Set(name string, value string, tags []string, rate float64) error {
	stat := fmt.Sprintf("%s|s", value)
	return c.send(name, stat, tags, rate)
}

// Close the client connection
func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	return c.conn.Close()
}
