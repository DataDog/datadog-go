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
	"bytes"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Client is the default dogstatsd client.  It always sends out stats to dogstatsd
// as requested.
type Client struct {
	conn net.Conn
	// Namespace to prepend to all statsd calls
	Namespace string
	// Tags are global tags to be added to every statsd call
	Tags []string
	// BufferLength is the length of the buffer in commands.
	bufferLength int
	flushTime    time.Duration
	commands     []string
	stop         bool
	sync.Mutex
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

// NewBuffered returns a Client that buffers its output and sends it in chunks.
// Buflen is the length of the buffer in number of commands.
func NewBuffered(addr string, buflen int) (*Client, error) {
	client, err := New(addr)
	if err != nil {
		return nil, err
	}
	client.bufferLength = buflen
	client.commands = make([]string, 0, buflen)
	client.flushTime = time.Millisecond * 100
	go client.watch()
	return client, nil
}

// format a message from its name, value, tags and rate.  Also adds global
// namespace and tags.
func (c *Client) format(name, value string, tags []string, rate float64) string {
	var buf bytes.Buffer
	if c.Namespace != "" {
		buf.WriteString(c.Namespace)
	}
	buf.WriteString(name)
	buf.WriteString(":")
	buf.WriteString(value)
	if rate < 1 {
		buf.WriteString(`|@`)
		buf.WriteString(strconv.FormatFloat(rate, 'f', -1, 64))
	}

	tags = append(c.Tags, tags...)
	if len(tags) > 0 {
		buf.WriteString("|#")
		buf.WriteString(tags[0])
		for _, tag := range tags[1:] {
			buf.WriteString(",")
			buf.WriteString(tag)
		}
	}
	return buf.String()
}

func (c *Client) watch() {
	for _ = range time.Tick(c.flushTime) {
		if c.stop {
			return
		}
		c.Lock()
		if len(c.commands) > 0 {
			// FIXME: eating error here
			c.flush()
		}
		c.Unlock()
	}
}

func (c *Client) append(cmd string) error {
	c.Lock()
	c.commands = append(c.commands, cmd)
	// if we should flush, lets do it
	if len(c.commands) == c.bufferLength {
		if err := c.flush(); err != nil {
			return err
		}
	}
	c.Unlock()
	return nil
}

// flush the commands in the buffer.  Lock must be held by caller.
func (c *Client) flush() error {
	data := strings.Join(c.commands, "\n")
	_, err := c.conn.Write([]byte(data))
	// clear the slice with a slice op, doesn't realloc
	c.commands = c.commands[:0]
	return err
}

// send handles sampling and sends the message over UDP. It also adds global namespace prefixes and tags.
func (c *Client) send(name, value string, tags []string, rate float64) error {
	if c == nil {
		return nil
	}
	if rate < 1 && rand.Float64() > rate {
		return nil
	}
	data := c.format(name, value, tags, rate)
	// if this client is buffered, then we'll just append this
	if c.bufferLength > 0 {
		return c.append(data)
	}
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
	c.stop = true
	return c.conn.Close()
}
