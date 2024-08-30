package response

import (
	"html/template"
	"net/http"
)

type Config struct {
	Header http.Header
	Funcs  template.FuncMap
}

func (c Config) WithOptions(opts []Option) Config {
	for _, opt := range opts {
		c = opt(c)
	}
	return c
}

type Option func(Config) Config

func WithHeader(k, v string) Option {
	return func(c Config) Config {
		if c.Header == nil {
			c.Header = make(http.Header)
		}
		c.Header.Set(k, v)
		return c
	}
}

func WithHeaders(h map[string]string) Option {
	return func(c Config) Config {
		if c.Header == nil {
			c.Header = make(http.Header)
		}
		for k, v := range h {
			c.Header.Set(k, v)
		}
		return c
	}
}

func WithFuncs(f template.FuncMap) Option {
	return func(conf Config) Config {
		conf.Funcs = f
		return conf
	}
}
