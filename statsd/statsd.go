// Copyright 2013 Ooyala, Inc.

/*
Package statsd provides a Go dogstatsd client. Dogstatsd extends the popular statsd,
adding tags and histograms and pushing upstream to Datadog.

Refer to http://docs.datadoghq.com/guides/dogstatsd/ for information about DogStatsD.

Example Usage:

    // Create the client
    c, err := statsd.New("127.0.0.1:8125")
    if err != nil {
        log.Fatal(err)
    }
    // Prefix every metric with the app name
    c.Namespace = "flubber."
    // Send the EC2 availability zone as a tag with every metric
    c.Tags = append(c.Tags, "us-east-1a")
    err = c.Gauge("request.duration", 1.2, nil, 1)

statsd is based on go-statsd-client.
*/
package statsd

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"
)

/*
OptimalUDPPayloadSize defines the optimal payload size for a UDP datagram, 1432 bytes
is optimal for regular networks with an MTU of 1500 so datagrams don't get
fragmented. It's generally recommended not to fragment UDP datagrams as losing
a single fragment will cause the entire datagram to be lost.
*/
const OptimalUDPPayloadSize = 1432

/*
MaxUDPPayloadSize defines the maximum payload size for a UDP datagram.
Its value comes from the calculation: 65535 bytes Max UDP datagram size -
8byte UDP header - 60byte max IP headers
any number greater than that will see frames being cut out.
*/
const MaxUDPPayloadSize = 65467

/*
DefaultMaxAgentPayloadSize is the default maximum payload size the agent
can receive. This can be adjusted by changing dogstatsd_buffer_size in the
agent configuration file datadog.yaml.
*/
const DefaultMaxAgentPayloadSize = 8192

/*
UnixAddressPrefix holds the prefix to use to enable Unix Domain Socket
traffic instead of UDP.
*/
const UnixAddressPrefix = "unix://"

// Client-side entity ID injection for container tagging
const (
	entityIDEnvName = "DD_ENTITY_ID"
	entityIDTagName = "dd.internal.entity_id"
)

type noClientErr string

// ErrNoClient is returned if statsd reporting methods are invoked on
// a nil client.
const ErrNoClient = noClientErr("statsd client is nil")

func (e noClientErr) Error() string {
	return string(e)
}

// A Client is a handle for sending messages to dogstatsd.  It is safe to
// use one Client from multiple goroutines simultaneously.
type Client struct {
	// Sender handles the underlying networking protocol
	sender *sender
	// Namespace to prepend to all statsd calls
	Namespace string
	// Tags are global tags to be added to every statsd call
	Tags []string
	// skipErrors turns off error passing and allows UDS to emulate UDP behaviour
	SkipErrors bool
	flushTime  time.Duration
	bufferPool *bufferPool
	buffer     *statsdBuffer
	stop       chan struct{}
	sync.Mutex
}

// New returns a pointer to a new Client given an addr in the format "hostname:port" or
// "unix:///path/to/socket".
func New(addr string, options ...Option) (*Client, error) {
	var w statsdWriter
	o, err := resolveOptions(options)
	if err != nil {
		return nil, err
	}

	optimalPayloadSize := OptimalUDPPayloadSize
	if !strings.HasPrefix(addr, UnixAddressPrefix) {
		w, err = newUDPWriter(addr)
	} else {
		optimalPayloadSize = DefaultMaxAgentPayloadSize
		w, err = newUdsWriter(addr[len(UnixAddressPrefix)-1:])
	}
	if err != nil {
		return nil, err
	}

	if o.MaxBytePerPayload == 0 {
		o.MaxBytePerPayload = optimalPayloadSize
	}
	return newWithWriter(w, o)
}

// NewWithWriter creates a new Client with given writer. Writer is a
// io.WriteCloser + SetWriteTimeout(time.Duration) error
func NewWithWriter(w statsdWriter, options ...Option) (*Client, error) {
	o, err := resolveOptions(options)
	if err != nil {
		return nil, err
	}
	return newWithWriter(w, o)
}

func newWithWriter(w statsdWriter, o *Options) (*Client, error) {

	w.SetWriteTimeout(o.WriteTimeoutUDS)

	c := Client{
		Namespace: o.Namespace,
		Tags:      o.Tags,
	}

	// Inject DD_ENTITY_ID as a constant tag if found
	entityID := os.Getenv(entityIDEnvName)
	if entityID != "" {
		entityTag := fmt.Sprintf("%s:%s", entityIDTagName, entityID)
		c.Tags = append(c.Tags, entityTag)
	}

	if o.MaxBytePerPayload == 0 {
		o.MaxBytePerPayload = OptimalUDPPayloadSize
	}

	c.bufferPool = newBufferPool(o.BufferPoolSize, o.MaxBytePerPayload, o.MaxMessagesPerPayload)
	c.buffer = c.bufferPool.borrowBuffer()
	c.sender = newSender(w, o.SenderQueueSize, c.bufferPool)
	c.flushTime = o.BufferFlushInterval
	c.stop = make(chan struct{}, 1)
	go c.watch()

	return &c, nil
}

// NewBuffered returns a Client that buffers its output and sends it in chunks.
// Buflen is the length of the buffer in number of commands.
//
// When addr is empty, the client will default to a UDP client and use the DD_AGENT_HOST
// and (optionally) the DD_DOGSTATSD_PORT environment variables to build the target address.
func NewBuffered(addr string, buflen int) (*Client, error) {
	return New(addr, Buffered(), WithMaxMessagesPerPayload(buflen))
}

// SetWriteTimeout allows the user to set a custom UDS write timeout. Not supported for UDP.
func (c *Client) SetWriteTimeout(d time.Duration) error {
	if c == nil {
		return ErrNoClient
	}
	return c.sender.transport.SetWriteTimeout(d)
}

func (c *Client) watch() {
	ticker := time.NewTicker(c.flushTime)

	for {
		select {
		case <-ticker.C:
			c.Lock()
			c.flushLocked()
			c.Unlock()
		case <-c.stop:
			ticker.Stop()
			return
		}
	}
}

// GetSenderMetrics returns the metrics of the sender
func (c *Client) GetSenderMetrics() SenderMetrics {
	return c.sender.getMetrics()
}

// Flush forces a flush of all the queued dogstatsd payloads
// This method is blocking and will not return untill everything is sent
// trough the network
func (c *Client) Flush() error {
	if c == nil {
		return ErrNoClient
	}
	c.Lock()
	defer c.Unlock()
	c.flushLocked()
	c.sender.flush()
	return nil
}

// flush the current buffer. Lock must be held by caller.
// flushed buffer is sent as
func (c *Client) flushLocked() {
	if len(c.buffer.bytes()) > 0 {
		c.sender.send(c.buffer)
		c.buffer = c.bufferPool.borrowBuffer()
	}
}

func (c *Client) shouldSample(rate float64) bool {
	if rate < 1 && rand.Float64() > rate {
		return true
	}
	return false
}

// Gauge measures the value of a metric at a particular time.
func (c *Client) Gauge(name string, value float64, tags []string, rate float64) error {
	if c == nil {
		return ErrNoClient
	}
	if c.shouldSample(rate) {
		return nil
	}
	c.Lock()
	var err error
	if err = c.buffer.writeGauge(c.Namespace, c.Tags, name, value, tags, rate); err == errBufferFull {
		c.flushLocked()
		err = c.buffer.writeGauge(c.Namespace, c.Tags, name, value, tags, rate)
	}
	c.Unlock()
	return err
}

// Count tracks how many times something happened per second.
func (c *Client) Count(name string, value int64, tags []string, rate float64) error {
	if c == nil {
		return ErrNoClient
	}
	if c.shouldSample(rate) {
		return nil
	}
	c.Lock()
	var err error
	if err = c.buffer.writeCount(c.Namespace, c.Tags, name, value, tags, rate); err == errBufferFull {
		c.flushLocked()
		err = c.buffer.writeCount(c.Namespace, c.Tags, name, value, tags, rate)
	}
	c.Unlock()
	return err
}

// Histogram tracks the statistical distribution of a set of values on each host.
func (c *Client) Histogram(name string, value float64, tags []string, rate float64) error {
	if c == nil {
		return ErrNoClient
	}
	if c.shouldSample(rate) {
		return nil
	}
	c.Lock()
	var err error
	if err = c.buffer.writeHistogram(c.Namespace, c.Tags, name, value, tags, rate); err == errBufferFull {
		c.flushLocked()
		err = c.buffer.writeHistogram(c.Namespace, c.Tags, name, value, tags, rate)
	}
	c.Unlock()
	return err
}

// Distribution tracks the statistical distribution of a set of values across your infrastructure.
func (c *Client) Distribution(name string, value float64, tags []string, rate float64) error {
	if c == nil {
		return ErrNoClient
	}
	if c.shouldSample(rate) {
		return nil
	}
	c.Lock()
	var err error
	if err = c.buffer.writeDistribution(c.Namespace, c.Tags, name, value, tags, rate); err == errBufferFull {
		c.flushLocked()
		err = c.buffer.writeDistribution(c.Namespace, c.Tags, name, value, tags, rate)
	}
	c.Unlock()
	return err
}

// Decr is just Count of -1
func (c *Client) Decr(name string, tags []string, rate float64) error {
	return c.Count(name, -1, tags, rate)
}

// Incr is just Count of 1
func (c *Client) Incr(name string, tags []string, rate float64) error {
	return c.Count(name, 1, tags, rate)
}

// Set counts the number of unique elements in a group.
func (c *Client) Set(name string, value string, tags []string, rate float64) error {
	if c == nil {
		return ErrNoClient
	}
	if c.shouldSample(rate) {
		return nil
	}
	c.Lock()
	var err error
	if err = c.buffer.writeSet(c.Namespace, c.Tags, name, value, tags, rate); err == errBufferFull {
		c.flushLocked()
		err = c.buffer.writeSet(c.Namespace, c.Tags, name, value, tags, rate)
	}
	c.Unlock()
	return err
}

// Timing sends timing information, it is an alias for TimeInMilliseconds
func (c *Client) Timing(name string, value time.Duration, tags []string, rate float64) error {
	return c.TimeInMilliseconds(name, value.Seconds()*1000, tags, rate)
}

// TimeInMilliseconds sends timing information in milliseconds.
// It is flushed by statsd with percentiles, mean and other info (https://github.com/etsy/statsd/blob/master/docs/metric_types.md#timing)
func (c *Client) TimeInMilliseconds(name string, value float64, tags []string, rate float64) error {
	if c == nil {
		return ErrNoClient
	}
	if c.shouldSample(rate) {
		return nil
	}
	c.Lock()
	var err error
	if err = c.buffer.writeTiming(c.Namespace, c.Tags, name, value, tags, rate); err == errBufferFull {
		c.flushLocked()
		err = c.buffer.writeTiming(c.Namespace, c.Tags, name, value, tags, rate)
	}
	c.Unlock()
	return err
}

// Event sends the provided Event.
func (c *Client) Event(e *Event) error {
	if c == nil {
		return ErrNoClient
	}
	c.Lock()
	var err error
	if err = c.buffer.writeEvent(*e, c.Tags); err == errBufferFull {
		c.flushLocked()
		err = c.buffer.writeEvent(*e, c.Tags)
	}
	c.Unlock()
	return err
}

// SimpleEvent sends an event with the provided title and text.
func (c *Client) SimpleEvent(title, text string) error {
	e := NewEvent(title, text)
	return c.Event(e)
}

// ServiceCheck sends the provided ServiceCheck.
func (c *Client) ServiceCheck(sc *ServiceCheck) error {
	if c == nil {
		return ErrNoClient
	}
	c.Lock()
	var err error
	if err = c.buffer.writeServiceCheck(*sc, c.Tags); err == errBufferFull {
		c.flushLocked()
		err = c.buffer.writeServiceCheck(*sc, c.Tags)
	}
	c.Unlock()
	return err
}

// SimpleServiceCheck sends an serviceCheck with the provided name and status.
func (c *Client) SimpleServiceCheck(name string, status ServiceCheckStatus) error {
	sc := NewServiceCheck(name, status)
	return c.ServiceCheck(sc)
}

// Close the client connection.
func (c *Client) Close() error {
	if c == nil {
		return ErrNoClient
	}
	select {
	case c.stop <- struct{}{}:
	default:
	}
	if err := c.Flush(); err != nil {
		return err
	}
	return c.sender.close()
}
