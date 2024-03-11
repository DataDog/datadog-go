package otel

import (
	"log/slog"

	"go.opentelemetry.io/otel/sdk/resource"

	"github.com/DataDog/datadog-go/v5/statsd"
)

type Config struct {
	client     *statsd.Client
	logger     *slog.Logger
	res        *resource.Resource
	errHandler func(error)
}

// OTELOption applies a configuration option to the MeterProvider.
type OTELOption func(Config) Config

func newConfig(options ...OTELOption) Config {
	cfg := Config{}

	for _, option := range options {
		cfg = option(cfg)
	}

	return cfg
}

func WithClient(client *statsd.Client) OTELOption {
	return func(cfg Config) Config {
		cfg.client = client
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

func WithErrorHandler(f func(error)) OTELOption {
	return func(cfg Config) Config {
		cfg.errHandler = f
		return cfg
	}
}
