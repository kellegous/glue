package metrics

const defaultPath = "/metrics"

type HTTPOptions struct {
	authToken string
	path      string
}

type HTTPOption func(*HTTPOptions)

func WithAuthToken(token string) HTTPOption {
	return func(o *HTTPOptions) {
		o.authToken = token
	}
}

func WithPath(path string) HTTPOption {
	return func(o *HTTPOptions) {
		o.path = path
	}
}
