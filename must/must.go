package must

// Do panics if the function returns an error. This is primarily intended to
// be used in defer statements, for example:
//
//	defer must.Do(r.Close)
//
// Deprecated: this helper is deprecated.
func Do(fn func() error) {
	if err := fn(); err != nil {
		panic(err)
	}
}

// BeOK panics if the error is not nil.
//
// Deprecated: this helper is deprecated.
func BeOK(err error) {
	if err != nil {
		panic(err)
	}
}

// GetValue returns the value if the error is nil. Otherwise, it panics.
//
// Deprecated: this helper is deprecated.
func GetValue[T any](value T, err error) T {
	BeOK(err)
	return value
}
