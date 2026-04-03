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

func ForHTTP(mux *http.ServeMux, opts ...HTTPOption) http.Handler {
	var options HTTPOptions
	for _, opt := range opts {
		opt(&options)
	}

	mux.Handle(
		"/metrics",
		maybeRequireAuthToken(options.authToken, promhttp.Handler()))

	return promhttp.InstrumentHandlerDuration(httpDurations,
		promhttp.InstrumentHandlerCounter(httpCounts, mux))
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
