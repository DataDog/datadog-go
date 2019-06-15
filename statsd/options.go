package statsd

import (
	"math"
	"time"
)

var (
	// DefaultNamespace is the default value for the Namespace option
	DefaultNamespace = ""
	// DefaultTags is the default value for the Tags option
	DefaultTags = []string{}
	// DefaultBuffered is the default value for the Buffered option
	DefaultBuffered = false
	// DefaultMaxBytePerPayload is the default value for the MaxBytePerPayload option
	DefaultMaxBytePerPayload = 0
	// DefaultMaxMessagesPerPayload is the default value for the MaxMessagesPerPayload option
	DefaultMaxMessagesPerPayload = math.MaxInt32
	// DefaultBufferPoolSize is the default value for the DefaultBufferPoolSize option
	DefaultBufferPoolSize = 2048
	// DefaultBufferFlushInterval is the default value for the BufferFlushInterval option
	DefaultBufferFlushInterval = 100 * time.Millisecond
	// DefaultSenderQueueSize is the default value for the DefaultSenderQueueSize option
	DefaultSenderQueueSize = 2048
	// DefaultAsyncUDS is the default value for the AsyncUDS option
	DefaultAsyncUDS = false
	// DefaultWriteTimeoutUDS is the default value for the WriteTimeoutUDS option
	DefaultWriteTimeoutUDS = 1 * time.Millisecond
)

// Options contains the configuration options for a client.
type Options struct {
	// Namespace to prepend to all metrics, events and service checks name.
	Namespace string
	// Tags are global tags to be applied to every metrics, events and service checks.
	Tags []string
	// Buffered allows to pack multiple DogStatsD messages in one payload. Messages will be buffered
	// until the total size of the payload exceeds MaxMessagesPerPayload metrics, events and/or service
	// checks or after 100ms since the payload startedto be built.
	Buffered bool
	// MaxMessagesPerPayload is the maximum number of bytes a single payload will contain.
	// The magic value 0 will set the option to the optimal size for the transport
	// protocol used when creating the client: 1432 for UDP and 8192 for UDS.
	// Note that this option only takes effect when the client is buffered.
	MaxBytePerPayload int
	// MaxMessagesPerPayload is the maximum number of metrics, events and/or service checks a single payload will contain.
	// Note that this option only takes effect when the client is buffered.
	MaxMessagesPerPayload int
	// BufferPoolSize is the size of the pool of buffers in number of buffers.
	BufferPoolSize int
	// BufferFlushInterval is the interval after which the current buffer will get flushed.
	BufferFlushInterval time.Duration
	// SenderQueueSize is the size of the sender queue in number of buffers.
	SenderQueueSize int
	// AsyncUDS allows to switch between async and blocking mode for UDS.
	// Blocking mode allows for error checking but does not guarentee that calls won't block the execution.
	AsyncUDS bool
	// WriteTimeoutUDS is the timeout after which a UDS packet is dropped.
	WriteTimeoutUDS time.Duration
}

func resolveOptions(options []Option) (*Options, error) {
	o := &Options{
		Namespace:             DefaultNamespace,
		Tags:                  DefaultTags,
		Buffered:              DefaultBuffered,
		MaxBytePerPayload:     DefaultMaxBytePerPayload,
		MaxMessagesPerPayload: DefaultMaxMessagesPerPayload,
		BufferPoolSize:        DefaultBufferPoolSize,
		BufferFlushInterval:   DefaultBufferFlushInterval,
		SenderQueueSize:       DefaultSenderQueueSize,
		AsyncUDS:              DefaultAsyncUDS,
		WriteTimeoutUDS:       DefaultWriteTimeoutUDS,
	}

	for _, option := range options {
		err := option(o)
		if err != nil {
			return nil, err
		}
	}

	return o, nil
}

// Option is a client option. Can return an error if validation fails.
type Option func(*Options) error

// WithNamespace sets the Namespace option.
func WithNamespace(namespace string) Option {
	return func(o *Options) error {
		o.Namespace = namespace
		return nil
	}
}

// WithTags sets the Tags option.
func WithTags(tags []string) Option {
	return func(o *Options) error {
		o.Tags = tags
		return nil
	}
}

// Buffered sets the Buffered option.
func Buffered() Option {
	return func(o *Options) error {
		o.Buffered = true
		return nil
	}
}

// WithMaxMessagesPerPayload sets the MaxMessagesPerPayload option.
func WithMaxMessagesPerPayload(maxMessagesPerPayload int) Option {
	return func(o *Options) error {
		o.MaxMessagesPerPayload = maxMessagesPerPayload
		return nil
	}
}

// WithMaxBytePerPayload sets the MaxBytePerPayload option.
func WithMaxBytePerPayload(maxBytePerPayload int) Option {
	return func(o *Options) error {
		o.MaxBytePerPayload = maxBytePerPayload
		return nil
	}
}

// WithBufferPoolSize sets the BufferPoolSize option.
func WithBufferPoolSize(bufferPoolSize int) Option {
	return func(o *Options) error {
		o.BufferPoolSize = bufferPoolSize
		return nil
	}
}

// WithBufferFlushInterval sets the BufferFlushInterval option.
func WithBufferFlushInterval(bufferFlushInterval time.Duration) Option {
	return func(o *Options) error {
		o.BufferFlushInterval = bufferFlushInterval
		return nil
	}
}

// WithSenderQueueSize sets the SenderQueueSize option.
func WithSenderQueueSize(senderQueueSize int) Option {
	return func(o *Options) error {
		o.SenderQueueSize = senderQueueSize
		return nil
	}
}

// WithAsyncUDS sets the AsyncUDS option.
func WithAsyncUDS() Option {
	return func(o *Options) error {
		o.AsyncUDS = true
		return nil
	}
}

// WithWriteTimeoutUDS sets the WriteTimeoutUDS option.
func WithWriteTimeoutUDS(writeTimeoutUDS time.Duration) Option {
	return func(o *Options) error {
		o.WriteTimeoutUDS = writeTimeoutUDS
		return nil
	}
}
