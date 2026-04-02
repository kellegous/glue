package metrics

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	rpcCounts = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rpc_requests_total",
			Help: "Total number of RPC requests",
		},
		[]string{"method", "code"},
	)

	rpcDurations = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rpc_request_duration_seconds",
			Help:    "RPC request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method"},
	)
)

func ForRPC() connect.Interceptor {
	return rpcMetricsInterceptor{}
}

type rpcMetricsInterceptor struct{}

func (rpcMetricsInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		at := time.Now()
		res, err := next(ctx, req)
		took := time.Since(at)

		method := req.Spec().Procedure
		code := statusFromError(err)

		rpcCounts.WithLabelValues(method, code).Inc()
		rpcDurations.WithLabelValues(method).Observe(took.Seconds())

		return res, err
	}
}

func (rpcMetricsInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		at := time.Now()
		err := next(ctx, conn)
		took := time.Since(at)

		method := conn.Spec().Procedure
		code := statusFromError(err)

		rpcCounts.WithLabelValues(method, code).Inc()
		rpcDurations.WithLabelValues(method).Observe(took.Seconds())

		return err
	}
}

func (rpcMetricsInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func statusFromError(err error) string {
	if err == nil {
		return "ok"
	}
	return connect.CodeOf(err).String()
}
