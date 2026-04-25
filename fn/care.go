package fn

import "errors"

func WithCare(fn func() error, err *error) {
	*err = errors.Join(*err, fn())
}

func WithAbandon(fn func() error) {
	_ = fn()
}

func WithPrejudice(fn func() error) {
	if err := fn(); err != nil {
		panic(err)
	}
}
