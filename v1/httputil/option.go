package httputil

type Config struct {
	MaxMem int64
}

func (c Config) WithOptions(opts []Option) Config {
	for _, opt := range opts {
		c = opt(c)
	}
	return c
}

type Option func(Config) Config

// The maximum amount of memory that is permitted to buffer input data
// per request before it is offloaded to temporary disk storage. This is
// mainly only relevant to multipart file data.
func MaximumMemory(v int64) Option {
	return func(c Config) Config {
		c.MaxMem = v
		return c
	}
}
