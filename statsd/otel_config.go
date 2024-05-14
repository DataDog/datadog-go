package statsd

import (
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/sdk/resource"
)

type Config struct {
	client                     *Client
	logger                     *slog.Logger
	res                        *resource.Resource
	observerCollectionInterval time.Duration
	observerCollectionTimeout  time.Duration
}

// OTELOption applies a configuration option to the MeterProvider.
type OTELOption func(Config) Config

const (
	defaultObserverCollectionInterval = time.Second * 10
	defaultObserverCollectionTimeout  = time.Second * 2
)

func newConfig(options ...OTELOption) Config {
	cfg := Config{
		observerCollectionInterval: defaultObserverCollectionInterval,
		observerCollectionTimeout:  defaultObserverCollectionTimeout,
	}

	for _, option := range options {
		cfg = option(cfg)
	}

	return cfg
}

func WithClient(client *Client) OTELOption {
	return func(cfg Config) Config {
		cfg.client = client
		return cfg
	}
}

func WithObserverCollectionInterval(interval time.Duration) OTELOption {
	return func(cfg Config) Config {
		cfg.observerCollectionInterval = interval
		return cfg
	}
}

func WithObserverCollectionTimeout(timeout time.Duration) OTELOption {
	return func(cfg Config) Config {
		cfg.observerCollectionTimeout = timeout
		return cfg
	}
}

func WithLogger(logger *slog.Logger) OTELOption {
	return func(cfg Config) Config {
		cfg.logger = logger
		return cfg
	}
}

func WithResource(res *resource.Resource) OTELOption {
	return func(cfg Config) Config {
		cfg.res = res
		return cfg
	}
}
