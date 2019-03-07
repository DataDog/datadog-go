package statsd

import "time"

var (
	// DefaultNamespace is the default value for the Namespace option
	DefaultNamespace = ""
	// DefaultTags is the default value for the Tags option
	DefaultTags = []string{}
	// DefaultBuffered is the default value for the Buffered option
	DefaultBuffered = false
	// DefaultMaxMessagesPerPayload is the default value for the MaxMessagesPerPayload option
	DefaultMaxMessagesPerPayload = 16
	// DefaultBlockingUDS is the default value for the BlockingUDS option
	DefaultBlockingUDS = false
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
	// MaxMessagesPerPayload is the maximum number of metrics, events and/or service checks a single payload will contain.
	// Note that this option only takes effect when the client is buffered.
	MaxMessagesPerPayload int
	// BlockingUDS allows to switch between async and blocking mode for UDS.
	// Blocking mode allows for error checking but does not guarentee that calls won't block the execution.
	BlockingUDS bool
	// WriteTimeoutUDS is the timeout after which a UDS packet is dropped.
	WriteTimeoutUDS time.Duration
}

func resolveOptions(options []Option) *Options {
	o := &Options{
		Namespace:             DefaultNamespace,
		Tags:                  DefaultTags,
		Buffered:              DefaultBuffered,
		MaxMessagesPerPayload: DefaultMaxMessagesPerPayload,
		BlockingUDS:           DefaultBlockingUDS,
		WriteTimeoutUDS:       DefaultWriteTimeoutUDS,
	}

	for _, option := range options {
		option(o)
	}

	return o
}

// Option is a client option.
type Option func(*Options)

// Namespace sets the Namespace option.
func Namespace(namespace string) Option {
	return func(o *Options) {
		o.Namespace = namespace
	}
}

// Tags sets the Tags option.
func Tags(tags []string) Option {
	return func(o *Options) {
		o.Tags = tags
	}
}

// Buffered sets the Buffered option.
func Buffered(buffered bool) Option {
	return func(o *Options) {
		o.Buffered = buffered
	}
}

// MaxMessagesPerPayload sets the MaxMessagesPerPayload option.
func MaxMessagesPerPayload(maxMessagesPerPayload int) Option {
	return func(o *Options) {
		o.MaxMessagesPerPayload = maxMessagesPerPayload
	}
}

// BlockingUDS sets the BlockingUDS option.
func BlockingUDS(blockingUDS bool) Option {
	return func(o *Options) {
		o.BlockingUDS = blockingUDS
	}
}

// WriteTimeoutUDS sets the WriteTimeoutUDS option.
func WriteTimeoutUDS(writeTimeoutUDS time.Duration) Option {
	return func(o *Options) {
		o.WriteTimeoutUDS = writeTimeoutUDS
	}
}
