package framework

type Option func(*Options)

type Options struct {
	logEnabled  bool
	withContext bool
}

func WithLoggingDisabled() Option {
	return func(opts *Options) {
		opts.logEnabled = false
	}
}

func WithContextDisabled() Option {
	return func(opts *Options) {
		opts.withContext = false
	}
}

func TestOptions(opts ...Option) *Options {
	options := &Options{logEnabled: true, withContext: true}
	for _, opt := range opts {
		opt(options)
	}

	return options
}
