package framework

type Option func(*Options)

type Options struct {
	logEnabled bool
}

func WithLoggingDisabled() Option {
	return func(opts *Options) {
		opts.logEnabled = false
	}
}

func LogOptions(opts ...Option) *Options {
	options := &Options{
		logEnabled: true,
	}
	for _, opt := range opts {
		opt(options)
	}

	return options
}
