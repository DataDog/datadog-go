package statsd

import (
	"log/slog"

	"go.opentelemetry.io/otel/sdk/resource"
)

type Config struct {
	client *Client
	logger *slog.Logger
	res    *resource.Resource
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

func WithClient(client *Client) OTELOption {
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
