package rest

import (
	"log/slog"

	"github.com/bww/go-metrics/v1"
	"github.com/bww/go-router/v2"
)

type Config struct {
	Default router.Handler
	Logger  *slog.Logger
	Metrics *metrics.Metrics
	Verbose bool
	Debug   bool
}

func (c Config) WithOptions(opts []Option) (Config, error) {
	var err error
	for _, opt := range opts {
		c, err = opt(c)
		if err != nil {
			return c, err
		}
	}
	return c, nil
}

type Option func(s Config) (Config, error)

func WithDefault(r router.Handler) Option {
	return func(c Config) (Config, error) {
		c.Default = r
		return c, nil
	}
}

func WithVerbose(on bool) Option {
	return func(c Config) (Config, error) {
		c.Verbose = on
		return c, nil
	}
}

func WithDebug(on bool) Option {
	return func(c Config) (Config, error) {
		c.Debug = on
		return c, nil
	}
}

func WithLogger(l *slog.Logger) Option {
	return func(c Config) (Config, error) {
		c.Logger = l
		return c, nil
	}
}

func WithMetrics(m *metrics.Metrics) Option {
	return func(c Config) (Config, error) {
		c.Metrics = m
		return c, nil
	}
}
