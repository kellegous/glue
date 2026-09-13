package zap

import (
	"time"

	"github.com/kellegous/poop"
	"go.uber.org/zap"
)

type RegisterOptions struct {
	app            string
	queueSize      int
	drainTimeout   time.Duration
	retryLimit     uint
	baseRetryDelay time.Duration
	maxRetryDelay  time.Duration
	requestTimeout time.Duration
}

var globalRegisterOptions = RegisterOptions{
	queueSize:      defaultQueueSize,
	drainTimeout:   defaultDrainTimeout,
	retryLimit:     defaultRetryLimit,
	baseRetryDelay: defaultBaseRetryDelay,
	maxRetryDelay:  defaultMaxRetryDelay,
	requestTimeout: defaultRequestTimeout,
}

type RegisterOption func(*RegisterOptions)

func WithApp(app string) RegisterOption {
	return func(o *RegisterOptions) {
		o.app = app
	}
}

func WithQueueSize(queueSize int) RegisterOption {
	return func(o *RegisterOptions) {
		o.queueSize = queueSize
	}
}

func WithDrainTimeout(drainTimeout time.Duration) RegisterOption {
	return func(o *RegisterOptions) {
		o.drainTimeout = drainTimeout
	}
}

func WithRetryLimit(retryLimit uint) RegisterOption {
	return func(o *RegisterOptions) {
		o.retryLimit = retryLimit
	}
}

func WithBaseRetryDelay(baseRetryDelay time.Duration) RegisterOption {
	return func(o *RegisterOptions) {
		o.baseRetryDelay = baseRetryDelay
	}
}

func WithMaxRetryDelay(maxRetryDelay time.Duration) RegisterOption {
	return func(o *RegisterOptions) {
		o.maxRetryDelay = maxRetryDelay
	}
}

func WithRequestTimeout(requestTimeout time.Duration) RegisterOption {
	return func(o *RegisterOptions) {
		o.requestTimeout = requestTimeout
	}
}

func Register(opts ...RegisterOption) error {
	for _, opt := range opts {
		opt(&globalRegisterOptions)
	}

	if err := zap.RegisterSink(httpScheme, newSink); err != nil {
		return poop.Chain(err)
	}
	if err := zap.RegisterSink(httpsScheme, newSink); err != nil {
		return poop.Chain(err)
	}
	return nil
}
