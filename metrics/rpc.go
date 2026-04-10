package metrics

import (
	"context"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	rpcSourceHeader  = "X-Rpc-Source"
	rpcSourceUnknown = "unknown"
)

var (
	rpcCounts = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rpc_requests_total",
			Help: "Total number of RPC requests",
		},
		[]string{"method", "code", "source"},
	)

	rpcDurations = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rpc_request_duration_seconds",
			Help:    "RPC request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "source"},
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
		source := sourceFrom(req.Header())

		rpcCounts.WithLabelValues(method, code, source).Inc()
		rpcDurations.WithLabelValues(method, source).Observe(took.Seconds())

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
		source := sourceFrom(conn.RequestHeader())

		rpcCounts.WithLabelValues(method, code, source).Inc()
		rpcDurations.WithLabelValues(method, source).Observe(took.Seconds())

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

func sourceFrom(header http.Header) string {
	if src := header.Get(rpcSourceHeader); src != "" {
		return src
	}
	return rpcSourceUnknown
}

func addSource(src string, header http.Header) {
	header.Set(rpcSourceHeader, src)
}

type rpcSourceInterceptor struct {
	source string
}

// WithSource returns a client interceptor that sets X-Rpc-Source on every RPC.
func WithSource(source string) connect.Interceptor {
	return &rpcSourceInterceptor{source: source}
}

var _ connect.Interceptor = (*rpcSourceInterceptor)(nil)

func (i *rpcSourceInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		addSource(i.source, req.Header())
		return next(ctx, req)
	}
}

func (i *rpcSourceInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		conn := next(ctx, spec)
		addSource(i.source, conn.RequestHeader())
		return conn
	}
}

func (i *rpcSourceInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}
