package rpc

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestCORForHTTP(t *testing.T) {
	t.Parallel()

	type Expected struct {
		Status   int
		Body     string
		Headers  map[string]string
		CallNext bool
	}

	tests := []struct {
		Name     string
		Opts     []CORSOption
		Method   string
		Expected Expected
	}{
		{
			Name:   "options preflight with defaults",
			Method: http.MethodOptions,
			Expected: Expected{
				Status: http.StatusNoContent,
				Headers: map[string]string{
					"Access-Control-Allow-Origin":  "*",
					"Access-Control-Max-Age":       strconv.Itoa(defaultMaxAge),
					"Access-Control-Allow-Methods": "*",
				},
			},
		},
		{
			Name:   "options with custom origin",
			Opts:   []CORSOption{WithOrigin("https://app.example")},
			Method: http.MethodOptions,
			Expected: Expected{
				Status: http.StatusNoContent,
				Headers: map[string]string{
					"Access-Control-Allow-Origin":  "https://app.example",
					"Access-Control-Max-Age":       strconv.Itoa(defaultMaxAge),
					"Access-Control-Allow-Methods": "*",
				},
			},
		},
		{
			Name:   "options with custom methods",
			Opts:   []CORSOption{WithMethods([]string{"GET", "POST"})},
			Method: http.MethodOptions,
			Expected: Expected{
				Status: http.StatusNoContent,
				Headers: map[string]string{
					"Access-Control-Allow-Origin":  "*",
					"Access-Control-Max-Age":       strconv.Itoa(defaultMaxAge),
					"Access-Control-Allow-Methods": "GET, POST",
				},
			},
		},
		{
			Name: "options with custom headers and max age",
			Opts: []CORSOption{
				WithHeaders([]string{"Authorization", "Content-Type"}),
				WithMaxAge(3600),
			},
			Method: http.MethodOptions,
			Expected: Expected{
				Status: http.StatusNoContent,
				Headers: map[string]string{
					"Access-Control-Allow-Origin":  "*",
					"Access-Control-Max-Age":       "3600",
					"Access-Control-Allow-Methods": "*",
					"Access-Control-Allow-Headers": "Authorization, Content-Type",
				},
			},
		},
		{
			Name:   "non-options request passes through",
			Method: http.MethodGet,
			Expected: Expected{
				Status:   http.StatusTeapot,
				Body:     "ok",
				CallNext: true,
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()

			cors := NewCORS(tt.Opts...)
			nextCalled := false
			handler := cors.ForHTTP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusTeapot)
				_, _ = w.Write([]byte("ok"))
			}))

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tt.Method, "/", nil)
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.Expected.Status {
				t.Fatalf("status: got %d, want %d", rec.Code, tt.Expected.Status)
			}
			if got := rec.Body.String(); got != tt.Expected.Body {
				t.Fatalf("body: got %q, want %q", got, tt.Expected.Body)
			}
			if nextCalled != tt.Expected.CallNext {
				t.Fatalf("callNext: got %v, want %v", nextCalled, tt.Expected.CallNext)
			}
			for key, want := range tt.Expected.Headers {
				if got := rec.Header().Get(key); got != want {
					t.Fatalf("header %q: got %q, want %q", key, got, want)
				}
			}
			if tt.Method == http.MethodOptions && len(tt.Opts) == 0 {
				if got := rec.Header().Get("Access-Control-Allow-Headers"); got != "" {
					t.Fatalf("unexpected Access-Control-Allow-Headers: %q", got)
				}
			}
		})
	}
}

func TestCORSWrapUnary(t *testing.T) {
	t.Parallel()

	handlerErr := errors.New("handler failed")

	type Expected struct {
		Origin string
		Header bool
		Err    error
	}

	tests := []struct {
		Name     string
		Opts     []CORSOption
		NextErr  error
		Expected Expected
	}{
		{
			Name: "default origin on success",
			Expected: Expected{
				Origin: "*",
				Header: true,
			},
		},
		{
			Name: "custom origin on success",
			Opts: []CORSOption{WithOrigin("https://app.example")},
			Expected: Expected{
				Origin: "https://app.example",
				Header: true,
			},
		},
		{
			Name:    "error skips header",
			NextErr: handlerErr,
			Expected: Expected{
				Err: handlerErr,
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()

			cors := NewCORS(tt.Opts...)
			wrapped := cors.WrapUnary(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
				if tt.NextErr != nil {
					return nil, tt.NextErr
				}
				return connect.NewResponse(&struct{}{}), nil
			})

			res, err := wrapped(context.Background(), connect.NewRequest(&struct{}{}))
			if !errors.Is(err, tt.Expected.Err) {
				t.Fatalf("error: got %v, want %v", err, tt.Expected.Err)
			}
			if tt.Expected.Header {
				if res == nil {
					t.Fatal("expected response")
				}
				if got := res.Header().Get("Access-Control-Allow-Origin"); got != tt.Expected.Origin {
					t.Fatalf("Access-Control-Allow-Origin: got %q, want %q", got, tt.Expected.Origin)
				}
				return
			}
			if res != nil {
				t.Fatalf("expected nil response, got %#v", res)
			}
		})
	}
}

func TestCORSWrapStreamingClient(t *testing.T) {
	t.Parallel()

	type Expected struct {
		Procedure string
	}

	tests := []struct {
		Name     string
		Spec     connect.Spec
		Expected Expected
	}{
		{
			Name: "passes through client connection",
			Spec: connect.Spec{Procedure: "/test.v1.Service/Stream"},
			Expected: Expected{
				Procedure: "/test.v1.Service/Stream",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()

			cors := NewCORS()
			wantConn := &stubStreamingClientConn{}
			called := false

			wrapped := cors.WrapStreamingClient(func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
				called = true
				if spec.Procedure != tt.Expected.Procedure {
					t.Fatalf("spec.Procedure: got %q, want %q", spec.Procedure, tt.Expected.Procedure)
				}
				return wantConn
			})

			gotConn := wrapped(context.Background(), tt.Spec)
			if !called {
				t.Fatal("expected next streaming client func to be called")
			}
			if gotConn != wantConn {
				t.Fatalf("conn: got %v, want %v", gotConn, wantConn)
			}
		})
	}
}

func TestCORSWrapStreamingHandler(t *testing.T) {
	t.Parallel()

	const path = "/test.v1.Service/Stream"

	type Expected struct {
		Origin string
		Header bool
		Err    bool
	}

	tests := []struct {
		Name     string
		Opts     []CORSOption
		Handler  func(context.Context, *connect.Request[emptypb.Empty], *connect.ServerStream[emptypb.Empty]) error
		Expected Expected
	}{
		{
			Name: "default origin on success",
			Handler: func(context.Context, *connect.Request[emptypb.Empty], *connect.ServerStream[emptypb.Empty]) error {
				return nil
			},
			Expected: Expected{
				Origin: "*",
				Header: true,
			},
		},
		{
			Name: "custom origin on success",
			Opts: []CORSOption{WithOrigin("https://app.example")},
			Handler: func(context.Context, *connect.Request[emptypb.Empty], *connect.ServerStream[emptypb.Empty]) error {
				return nil
			},
			Expected: Expected{
				Origin: "https://app.example",
				Header: true,
			},
		},
		{
			Name: "stream with messages completes",
			Handler: func(_ context.Context, _ *connect.Request[emptypb.Empty], stream *connect.ServerStream[emptypb.Empty]) error {
				return stream.Send(&emptypb.Empty{})
			},
		},
		{
			Name: "error skips header",
			Handler: func(context.Context, *connect.Request[emptypb.Empty], *connect.ServerStream[emptypb.Empty]) error {
				return connect.NewError(connect.CodeInternal, errors.New("stream failed"))
			},
			Expected: Expected{
				Err: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()

			cors := NewCORS(tt.Opts...)
			handler := connect.NewServerStreamHandler(
				path,
				tt.Handler,
				connect.WithInterceptors(cors),
			)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handler.ServeHTTP(w, r)
			}))
			t.Cleanup(server.Close)

			client := connect.NewClient[emptypb.Empty, emptypb.Empty](server.Client(), server.URL+path)
			stream, err := client.CallServerStream(context.Background(), connect.NewRequest(&emptypb.Empty{}))
			if err != nil {
				t.Fatalf("CallServerStream: %v", err)
			}

			for stream.Receive() {
			}
			if gotErr := stream.Err() != nil; gotErr != tt.Expected.Err {
				t.Fatalf("stream error: got %v, wantErr %v", stream.Err(), tt.Expected.Err)
			}

			if tt.Expected.Header {
				if got := stream.ResponseHeader().Get("Access-Control-Allow-Origin"); got != tt.Expected.Origin {
					t.Fatalf("Access-Control-Allow-Origin: got %q, want %q", got, tt.Expected.Origin)
				}
				return
			}

			if got := stream.ResponseHeader().Get("Access-Control-Allow-Origin"); got != "" {
				t.Fatalf("unexpected Access-Control-Allow-Origin: %q", got)
			}
		})
	}
}

type stubStreamingClientConn struct {
	connect.StreamingClientConn
}
