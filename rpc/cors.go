package rpc

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"connectrpc.com/connect"
)

const defaultMaxAge = 86400

type CORSOption func(*CORS)

// WithOrigin sets the origin header for the CORS response.
func WithOrigin(origin string) CORSOption {
	return func(c *CORS) {
		c.origin = origin
	}
}

// WithMethods sets the methods header for the CORS response.
func WithMethods(methods []string) CORSOption {
	return func(c *CORS) {
		c.methods = methods
	}
}

// WithHeaders sets the headers header for the CORS response.
func WithHeaders(headers []string) CORSOption {
	return func(c *CORS) {
		c.headers = headers
	}
}

// WithMaxAge sets the max age header for the CORS response.
func WithMaxAge(maxAge int) CORSOption {
	return func(c *CORS) {
		c.maxAge = maxAge
	}
}

type CORS struct {
	origin  string
	methods []string
	headers []string
	maxAge  int
}

var _ connect.Interceptor = (*CORS)(nil)

func (c *CORS) originHeader() string {
	if c.origin == "" {
		return "*"
	}
	return c.origin
}

func (c *CORS) optionsHeaders() map[string]string {
	methodsHeader := "*"
	if m := c.methods; len(m) > 0 {
		methodsHeader = strings.Join(m, ", ")
	}
	h := map[string]string{
		"Access-Control-Allow-Origin":  c.originHeader(),
		"Access-Control-Max-Age":       strconv.Itoa(c.maxAge),
		"Access-Control-Allow-Methods": methodsHeader,
	}

	if headers := c.headers; len(headers) > 0 {
		h["Access-Control-Allow-Headers"] = strings.Join(headers, ", ")
	}

	return h
}

// ForHTTP returns a http.Handler that sets the CORS headers for the preflight
// options requests.
func (c *CORS) ForHTTP(next http.Handler) http.Handler {
	pfHeaders := c.optionsHeaders()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			h := w.Header()
			for k, v := range pfHeaders {
				h.Set(k, v)
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (c *CORS) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		res, err := next(ctx, req)
		if err != nil {
			return res, err
		}
		h := res.Header()
		h.Set("Access-Control-Allow-Origin", c.originHeader())
		return res, err
	}
}

func (c *CORS) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		return next(ctx, spec)
	}
}

func (c *CORS) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		err := next(ctx, conn)
		if err != nil {
			return err
		}
		h := conn.ResponseHeader()
		h.Set("Access-Control-Allow-Origin", c.originHeader())
		return err
	}
}

func NewCORS(opts ...CORSOption) *CORS {
	c := &CORS{
		maxAge: defaultMaxAge,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}
