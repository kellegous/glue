package metrics

type HTTPOptions struct {
	authToken string
}

type HTTPOption func(*HTTPOptions)

func WithAuthToken(token string) HTTPOption {
	return func(o *HTTPOptions) {
		o.authToken = token
	}
}
