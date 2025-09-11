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
