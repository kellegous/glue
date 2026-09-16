package metrics

import (
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpCounts = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests.",
	}, []string{"code", "method"})

	httpDurations = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "http_request_duration_seconds",
		Help: "Duration of HTTP requests.",
	}, []string{"code", "method"})
)

func ForHTTP(next http.Handler, opts ...HTTPOption) http.Handler {
	options := HTTPOptions{
		path: defaultPath,
	}
	for _, opt := range opts {
		opt(&options)
	}

	toMetrics := maybeRequireAuthToken(options.authToken, promhttp.Handler())

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == options.path {
			toMetrics.ServeHTTP(w, r)
		} else {
			next.ServeHTTP(w, r)
		}
	})

	return promhttp.InstrumentHandlerDuration(httpDurations,
		promhttp.InstrumentHandlerCounter(httpCounts, handler))
}

func maybeRequireAuthToken(token string, next http.Handler) http.Handler {
	if token == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !hasAuthToken(token, r) {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func hasAuthToken(token string, r *http.Request) bool {
	// First check the authorization header
	if auth := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "); auth == token {
		return true
	}

	if r.URL.RawQuery == token {
		return true
	}

	return false
}
