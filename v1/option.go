package rest

import (
	"github.com/bww/go-metrics/v1"
	"github.com/sirupsen/logrus"
)

type Option func(s *Service) (*Service, error)

func WithVerbose(on bool) Option {
	return func(s *Service) (*Service, error) {
		s.verbose = on
		return s, nil
	}
}

func WithDebug(on bool) Option {
	return func(s *Service) (*Service, error) {
		s.debug = on
		return s, nil
	}
}

func WithLogger(l *logrus.Logger) Option {
	return func(s *Service) (*Service, error) {
		s.log = l
		return s, nil
	}
}

func WithMetrics(m *metrics.Metrics) Option {
	return func(s *Service) (*Service, error) {
		s.metrics = m
		return s, nil
	}
}

func WithHandlers(v ...Handler) Option {
	return func(s *Service) (*Service, error) {
		s.pline = &Pipeline{v}
		return s, nil
	}
}
