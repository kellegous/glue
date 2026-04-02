package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests.",
	}, []string{"code", "method"})

	httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "http_request_duration_seconds",
		Help: "Duration of HTTP requests.",
	}, []string{"code", "method"})
)

func ForHTTP(mux *http.ServeMux) http.Handler {
	mux.Handle("/metrics", promhttp.Handler())
	return promhttp.InstrumentHandlerDuration(httpDuration,
		promhttp.InstrumentHandlerCounter(httpRequestsTotal, mux))
}
