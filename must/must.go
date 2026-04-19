package must

// Do panics if the function returns an error. This is primarily intended to
// be used in defer statements, for example:
//
//	defer must.Do(r.Close)
func Do(fn func() error) {
	if err := fn(); err != nil {
		panic(err)
	}
}

// BeOK panics if the error is not nil.
func BeOK(err error) {
	if err != nil {
		panic(err)
	}
}

// GetValue returns the value if the error is nil. Otherwise, it panics.
func GetValue[T any](value T, err error) T {
	BeOK(err)
	return value
}
