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
)

type Client struct {
	conn net.Conn
	// Namespace to prepend to all statsd calls
	Namespace string
	// Global tags to be added to every statsd call
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
