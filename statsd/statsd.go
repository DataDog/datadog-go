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
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"
)

/*
OptimalPayloadSize defines the optimal payload size for a UDP datagram, 1432 bytes
is optimal for regular networks with an MTU of 1500 so datagrams don't get
fragmented. It's generally recommended not to fragment UDP datagrams as losing
a single fragment will cause the entire datagram to be lost.

This can be increased if your network has a greater MTU or you don't mind UDP
datagrams getting fragmented. The practical limit is MaxUDPPayloadSize
*/
const OptimalPayloadSize = 1432

/*
MaxUDPPayloadSize defines the maximum payload size for a UDP datagram.
Its value comes from the calculation: 65535 bytes Max UDP datagram size -
8byte UDP header - 60byte max IP headers
any number greater than that will see frames being cut out.
*/
const MaxUDPPayloadSize = 65467

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
	// Writer handles the underlying networking protocol
	writer statsdWriter
	// Namespace to prepend to all statsd calls
	Namespace string
	// Tags are global tags to be added to every statsd call
	Tags []string
	// skipErrors turns off error passing and allows UDS to emulate UDP behaviour
	SkipErrors bool
	// BufferLength is the length of the buffer in commands.
	bufferLength int
	flushTime    time.Duration
	commands     [][]byte
	buffer       bytes.Buffer
	stop         chan struct{}
	sync.Mutex
}

// New returns a pointer to a new Client given an addr in the format "hostname:port" or
// "unix:///path/to/socket".
func New(addr string, options ...Option) (*Client, error) {
	o, err := resolveOptions(options)
	if err != nil {
		return nil, err
	}

	var w statsdWriter

	if !strings.HasPrefix(addr, UnixAddressPrefix) {
		w, err = newUDPWriter(addr)
	} else {
		w, err = newUdsWriter(addr[len(UnixAddressPrefix)-1:])
	}
	if err != nil {
		return nil, err
	}
	w.SetWriteTimeout(o.WriteTimeoutUDS)

	c := Client{
		Namespace: o.Namespace,
		Tags:      o.Tags,
		writer:    w,
	}

	// Inject DD_ENTITY_ID as a constant tag if found
	entityID := os.Getenv(entityIDEnvName)
	if entityID != "" {
		entityTag := fmt.Sprintf("%s:%s", entityIDTagName, entityID)
		c.Tags = append(c.Tags, entityTag)
	}

	if o.Buffered {
		c.bufferLength = o.MaxMessagesPerPayload
		c.commands = make([][]byte, 0, o.MaxMessagesPerPayload)
		c.flushTime = time.Millisecond * 100
		c.stop = make(chan struct{}, 1)
		go c.watch()
	}

	return &c, nil
}

// NewWithWriter creates a new Client with given writer. Writer is a
// io.WriteCloser + SetWriteTimeout(time.Duration) error
func NewWithWriter(w statsdWriter) (*Client, error) {
	client := &Client{writer: w, SkipErrors: false}

	// Inject DD_ENTITY_ID as a constant tag if found
	entityID := os.Getenv(entityIDEnvName)
	if entityID != "" {
		entityTag := fmt.Sprintf("%s:%s", entityIDTagName, entityID)
		client.Tags = append(client.Tags, entityTag)
	}

	return client, nil
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
	return c.writer.SetWriteTimeout(d)
}

func (c *Client) watch() {
	ticker := time.NewTicker(c.flushTime)

	for {
		select {
		case <-ticker.C:
			c.Lock()
			if len(c.commands) > 0 {
				// FIXME: eating error here
				c.flushLocked()
			}
			c.Unlock()
		case <-c.stop:
			ticker.Stop()
			return
		}
	}
}

func (c *Client) append(cmd []byte) error {
	c.Lock()
	defer c.Unlock()
	c.commands = append(c.commands, cmd)
	// if we should flush, lets do it
	if len(c.commands) == c.bufferLength {
		if err := c.flushLocked(); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) joinMaxSize(cmds [][]byte, sep string, maxSize int) ([][]byte, []int) {
	c.buffer.Reset() //clear buffer

	var frames [][]byte
	var ncmds []int
	sepBytes := []byte(sep)
	sepLen := len(sep)

	elem := 0
	for _, cmd := range cmds {
		needed := len(cmd)

		if elem != 0 {
			needed = needed + sepLen
		}

		if c.buffer.Len()+needed <= maxSize {
			if elem != 0 {
				c.buffer.Write(sepBytes)
			}
			c.buffer.Write(cmd)
			elem++
		} else {
			frames = append(frames, copyAndResetBuffer(&c.buffer))
			ncmds = append(ncmds, elem)
			// if cmd is bigger than maxSize it will get flushed on next loop
			c.buffer.Write(cmd)
			elem = 1
		}
	}

	//add whatever is left! if there's actually something
	if c.buffer.Len() > 0 {
		frames = append(frames, copyAndResetBuffer(&c.buffer))
		ncmds = append(ncmds, elem)
	}

	return frames, ncmds
}

func copyAndResetBuffer(buf *bytes.Buffer) []byte {
	tmpBuf := make([]byte, buf.Len())
	copy(tmpBuf, buf.Bytes())
	buf.Reset()
	return tmpBuf
}

// Flush forces a flush of the pending commands in the buffer
func (c *Client) Flush() error {
	if c == nil {
		return ErrNoClient
	}
	c.Lock()
	defer c.Unlock()
	return c.flushLocked()
}

// flush the commands in the buffer.  Lock must be held by caller.
func (c *Client) flushLocked() error {
	frames, flushable := c.joinMaxSize(c.commands, "\n", OptimalPayloadSize)
	var err error
	cmdsFlushed := 0
	for i, data := range frames {
		_, e := c.writer.Write(data)
		if e != nil {
			err = e
			break
		}
		cmdsFlushed += flushable[i]
	}

	// clear the slice with a slice op, doesn't realloc
	if cmdsFlushed == len(c.commands) {
		c.commands = c.commands[:0]
	} else {
		//this case will cause a future realloc...
		// drop problematic command though (sorry).
		c.commands = c.commands[cmdsFlushed+1:]
	}
	return err
}

func (c *Client) sendMsg(msg []byte) error {
	// return an error if message is bigger than MaxUDPPayloadSize
	if len(msg) > MaxUDPPayloadSize {
		return errors.New("message size exceeds MaxUDPPayloadSize")
	}

	// if this client is buffered, then we'll just append this
	if c.bufferLength > 0 {
		return c.append(msg)
	}

	_, err := c.writer.Write(msg)

	if c.SkipErrors {
		return nil
	}
	return err
}

// Gauge measures the value of a metric at a particular time.
func (c *Client) Gauge(name string, value float64, tags []string, rate float64) error {
	if c == nil {
		return ErrNoClient
	}
	if rate < 1 && rand.Float64() > rate {
		return nil
	}
	buf := make([]byte, 0, 200)
	buf = appendGauge(buf, c.Namespace, c.Tags, name, value, tags, rate)
	return c.sendMsg(buf)
}

// Count tracks how many times something happened per second.
func (c *Client) Count(name string, value int64, tags []string, rate float64) error {
	if c == nil {
		return ErrNoClient
	}
	if rate < 1 && rand.Float64() > rate {
		return nil
	}
	buf := make([]byte, 0, 200)
	buf = appendCount(buf, c.Namespace, c.Tags, name, value, tags, rate)
	return c.sendMsg(buf)
}

// Histogram tracks the statistical distribution of a set of values on each host.
func (c *Client) Histogram(name string, value float64, tags []string, rate float64) error {
	if c == nil {
		return ErrNoClient
	}
	if rate < 1 && rand.Float64() > rate {
		return nil
	}
	buf := make([]byte, 0, 200)
	buf = appendHistogram(buf, c.Namespace, c.Tags, name, value, tags, rate)
	return c.sendMsg(buf)
}

// Distribution tracks the statistical distribution of a set of values across your infrastructure.
func (c *Client) Distribution(name string, value float64, tags []string, rate float64) error {
	if c == nil {
		return ErrNoClient
	}
	if rate < 1 && rand.Float64() > rate {
		return nil
	}
	buf := make([]byte, 0, 200)
	buf = appendDistribution(buf, c.Namespace, c.Tags, name, value, tags, rate)
	return c.sendMsg(buf)
}

// Decr is just Count of -1
func (c *Client) Decr(name string, tags []string, rate float64) error {
	if c == nil {
		return ErrNoClient
	}
	if rate < 1 && rand.Float64() > rate {
		return nil
	}
	buf := make([]byte, 0, 200)
	buf = appendDecrement(buf, c.Namespace, c.Tags, name, tags, rate)
	return c.sendMsg(buf)
}

// Incr is just Count of 1
func (c *Client) Incr(name string, tags []string, rate float64) error {
	if c == nil {
		return ErrNoClient
	}
	if rate < 1 && rand.Float64() > rate {
		return nil
	}
	buf := make([]byte, 0, 200)
	buf = appendIncrement(buf, c.Namespace, c.Tags, name, tags, rate)
	return c.sendMsg(buf)
}

// Set counts the number of unique elements in a group.
func (c *Client) Set(name string, value string, tags []string, rate float64) error {
	if c == nil {
		return ErrNoClient
	}
	if rate < 1 && rand.Float64() > rate {
		return nil
	}
	buf := make([]byte, 0, 200)
	buf = appendSet(buf, c.Namespace, c.Tags, name, value, tags, rate)
	return c.sendMsg(buf)
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
	if rate < 1 && rand.Float64() > rate {
		return nil
	}
	buf := make([]byte, 0, 200)
	buf = appendTiming(buf, c.Namespace, c.Tags, name, value, tags, rate)
	return c.sendMsg(buf)
}

// Event sends the provided Event.
func (c *Client) Event(e *Event) error {
	if c == nil {
		return ErrNoClient
	}
	buf := make([]byte, 0, 200)
	buf = appendEvent(buf, *e, c.Tags)
	return c.sendMsg(buf)
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
	buf := make([]byte, 0, 200)
	buf = appendServiceCheck(buf, *sc, c.Tags)
	return c.sendMsg(buf)
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

	// if this client is buffered, flush before closing the writer
	if c.bufferLength > 0 {
		if err := c.Flush(); err != nil {
			return err
		}
	}

	return c.writer.Close()
}
