package logging

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Option func(*zap.Config) error

func WithLevel(level zapcore.Level) Option {
	return func(c *zap.Config) error {
		c.Level = zap.NewAtomicLevelAt(level)
		return nil
	}
}

func WithOutputPaths(paths ...string) Option {
	return func(c *zap.Config) error {
		c.OutputPaths = paths
		return nil
	}
}

func Setup(opts ...Option) (*zap.Logger, error) {
	c := zap.NewProductionConfig()
	c.EncoderConfig.EncodeDuration = zapcore.StringDurationEncoder
	c.EncoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder

	for _, opt := range opts {
		if err := opt(&c); err != nil {
			return nil, err
		}
	}

	l, err := c.Build()
	if err != nil {
		return nil, err
	}

	zap.ReplaceGlobals(l)
	return l, nil
}

func MustSetup(opts ...Option) *zap.Logger {
	l, err := Setup(opts...)
	if err != nil {
		panic(err)
	}
	return l
}
