package framework

type Option func(*Options)

type Options struct {
	requestHeaders map[string]string
	logEnabled     bool
}

func WithLoggingDisabled() Option {
	return func(opts *Options) {
		opts.logEnabled = false
	}
}

func WithRequestHeaders(headers map[string]string) Option {
	return func(opts *Options) {
		opts.requestHeaders = headers
	}
}

func LogOptions(opts ...Option) *Options {
	options := &Options{
		logEnabled:     true,
		requestHeaders: make(map[string]string),
	}
	for _, opt := range opts {
		opt(options)
	}

	return options
}
