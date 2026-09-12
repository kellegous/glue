package zap

import (
	"context"
	"errors"
	"io"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/kellegous/glue/fn"
	"github.com/kellegous/glue/logging/yarder"
	"github.com/kellegous/glue/logging/yarder/yarder_connect"
)

type recordingYarderClient struct {
	lck      sync.Mutex
	requests []*yarder.LogReq
	errors   []error
}

func (c *recordingYarderClient) Log(_ context.Context, req *connect.Request[yarder.LogReq]) (*connect.Response[emptypb.Empty], error) {
	c.lck.Lock()
	defer c.lck.Unlock()

	c.requests = append(c.requests, req.Msg)
	if len(c.errors) > 0 {
		err := c.errors[0]
		c.errors = c.errors[1:]
		return nil, err
	}
	return connect.NewResponse(&emptypb.Empty{}), nil
}

func TestGetRPCURL(t *testing.T) {
	for _, tt := range []struct {
		Name        string
		URLInput    string
		ExpectedURL string
	}{
		{
			Name:        "http",
			URLInput:    "yarder+http://example.com/logs?app=test",
			ExpectedURL: "http://example.com/logs",
		},
		{
			Name:        "https",
			URLInput:    "yarder+https://example.com/logs?app=test",
			ExpectedURL: "https://example.com/logs",
		},
	} {
		t.Run(tt.Name, func(t *testing.T) {
			u, err := url.Parse(tt.URLInput)
			if err != nil {
				t.Fatal(err)
			}
			got, err := getRpcURL(u)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.ExpectedURL {
				t.Fatalf("getRpcURL() = %q, want %q", got, tt.ExpectedURL)
			}
		})
	}

	u, err := url.Parse("ftp://example.com")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := getRpcURL(u); err == nil {
		t.Fatal("getRpcURL() succeeded for an unsupported scheme")
	}
}

func TestGetInt(t *testing.T) {
	type Expected struct {
		Value int
		Error bool
	}

	for _, tt := range []struct {
		Name     string
		Input    string
		Default  int
		Expected Expected
	}{
		{
			Name:     "default",
			Input:    "",
			Default:  10,
			Expected: Expected{Value: 10, Error: false},
		},
		{
			Name:     "valid",
			Input:    "42",
			Default:  10,
			Expected: Expected{Value: 42, Error: false},
		},
		{
			Name:     "invalid",
			Input:    "nope",
			Default:  10,
			Expected: Expected{Value: 0, Error: true},
		},
	} {
		t.Run(tt.Name, func(t *testing.T) {
			got, err := getInt(tt.Input, tt.Default)
			if (err != nil) != tt.Expected.Error {
				t.Fatalf("getInt(%q, %d) error = %v, want error: %t", tt.Input, tt.Default, err, tt.Expected.Error)
			}
			if got != tt.Expected.Value {
				t.Fatalf("getInt(%q, %d) = %d, want %d", tt.Input, tt.Default, got, tt.Expected.Value)
			}
		})
	}
}

func TestGetDuration(t *testing.T) {
	type Expected struct {
		Value time.Duration
		Error bool
	}
	for _, tt := range []struct {
		Name     string
		Input    string
		Default  time.Duration
		Expected Expected
	}{
		{
			Name:     "default",
			Input:    "",
			Default:  time.Second,
			Expected: Expected{Value: time.Second, Error: false},
		},
		{
			Name:     "valid",
			Input:    "250ms",
			Default:  time.Second,
			Expected: Expected{Value: 250 * time.Millisecond, Error: false},
		},
		{
			Name:     "invalid",
			Input:    "soon",
			Default:  time.Second,
			Expected: Expected{Value: 0, Error: true},
		},
	} {
		t.Run(tt.Name, func(t *testing.T) {
			got, err := getDuration(tt.Input, tt.Default)
			if (err != nil) != tt.Expected.Error {
				t.Fatalf("getDuration(%q, %s) error = %v, want error: %t", tt.Input, tt.Default, err, tt.Expected.Error)
			}
			if got != tt.Expected.Value {
				t.Fatalf("getDuration(%q, %s) = %s, want %s", tt.Input, tt.Default, got, tt.Expected.Value)
			}
		})
	}
}

func TestSinkWriteCloseAndSync(t *testing.T) {
	s := &sink{
		buffer:       newCircBuffer[*yarder.LogReq](2),
		changed:      make(chan struct{}, 1),
		drainTimeout: time.Second,
		app:          "test-app",
		writerKey:    [16]byte{1},
	}

	if got, err := s.Write([]byte("first")); err != nil || got != len("first") {
		t.Fatalf("Write() = (%d, %v), want (5, nil)", got, err)
	}
	req, ok := s.buffer.Pop()
	if !ok || string(req.Data) != "first" {
		t.Fatalf("buffer.Pop() = (%+v, %t), want first data", req, ok)
	}
	if req.App != "test-app" {
		t.Fatalf("request app = %q, want test-app", req.App)
	}
	if req.WriterSeq != 1 {
		t.Fatalf("request writer sequence = %d, want 1", req.WriterSeq)
	}
	if len(req.WriterKey) != len(s.writerKey) {
		t.Fatalf("request writer key length = %d, want %d", len(req.WriterKey), len(s.writerKey))
	}
	var writerKey [16]byte
	copy(writerKey[:], req.WriterKey)
	if writerKey != s.writerKey {
		t.Fatalf("request writer key = %x, want %x", req.WriterKey, s.writerKey)
	}

	drained := s.subscribeForDrain()
	s.ackDrain()
	select {
	case <-drained:
	default:
		t.Fatal("drain subscriber was not notified")
	}

	completedDrain := make(chan struct{})
	close(completedDrain)
	s.drained = completedDrain
	if err := s.Sync(); err != nil {
		t.Fatalf("Sync() = %v, want nil", err)
	}

	if err := s.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}
	if got, err := s.Write([]byte("second")); !errors.Is(err, io.ErrClosedPipe) || got != 0 {
		t.Fatalf("Write() after Close = (%d, %v), want (0, io.ErrClosedPipe)", got, err)
	}
}

func TestSinkSyncTimesOut(t *testing.T) {
	s := &sink{buffer: newCircBuffer[*yarder.LogReq](1), changed: make(chan struct{}, 1), drainTimeout: time.Millisecond}
	if err := s.Sync(); err == nil {
		t.Fatal("Sync() succeeded without a drain acknowledgement")
	}
}

func TestSinkCloseWakesWorker(t *testing.T) {
	s := &sink{buffer: newCircBuffer[*yarder.LogReq](1), changed: make(chan struct{}, 1)}
	if err := s.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}
	select {
	case <-s.changed:
	default:
		t.Fatal("Close() did not notify the worker")
	}
}

func TestDeliverPendingDeliversBufferedRecordsAndAcknowledgesDrain(t *testing.T) {
	s := &sink{buffer: newCircBuffer[*yarder.LogReq](3), drained: make(chan struct{})}
	s.buffer.Push(&yarder.LogReq{App: "test-app", Data: []byte("one"), WriterSeq: 1})
	s.buffer.Push(&yarder.LogReq{App: "test-app", Data: []byte("two"), WriterSeq: 2})
	client := &recordingYarderClient{}

	if keepRunning := s.deliverPending(t.Context(), client); !keepRunning {
		t.Fatal("deliverPending() stopped an open sink")
	}
	if len(client.requests) != 2 {
		t.Fatalf("delivered %d requests, want 2", len(client.requests))
	}
	for i, want := range []string{"one", "two"} {
		if got := string(client.requests[i].Data); got != want {
			t.Fatalf("request %d data = %q, want %q", i, got, want)
		}
		if got := client.requests[i].App; got != "test-app" {
			t.Fatalf("request %d app = %q, want test-app", i, got)
		}
		if got, want := client.requests[i].WriterSeq, uint64(i+1); got != want {
			t.Fatalf("request %d writer sequence = %d, want %d", i, got, want)
		}
	}
	if s.drained != nil {
		t.Fatal("drain subscription was not cleared")
	}
}

func TestDeliverRetriesTransientFailures(t *testing.T) {
	client := &recordingYarderClient{errors: []error{errors.New("temporary failure")}}
	if err := deliver(t.Context(), client, &yarder.LogReq{App: "test", Data: []byte("entry")}, 2); err != nil {
		t.Fatalf("deliver() = %v, want nil", err)
	}
	if len(client.requests) != 2 {
		t.Fatalf("deliver() made %d attempts, want 2", len(client.requests))
	}
}

func TestNewSinkDeliversToYarder(t *testing.T) {
	received := make(chan *yarder.LogReq, 1)
	_, handler := yarder_connect.NewYarderHandler(yarderHandlerFunc(func(_ context.Context, req *connect.Request[yarder.LogReq]) (*connect.Response[emptypb.Empty], error) {
		received <- req.Msg
		return connect.NewResponse(&emptypb.Empty{}), nil
	}))
	server := httptest.NewServer(handler)
	defer server.Close()

	u, err := url.Parse(server.URL + "?app=integration&queue-size=2&drain-timeout=1s")
	if err != nil {
		t.Fatal(err)
	}
	u.Scheme = httpScheme
	writer, err := newSink(u)
	if err != nil {
		t.Fatal(err)
	}
	defer fn.WithPrejudice(writer.Close)

	if _, err := writer.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Sync(); err != nil {
		t.Fatalf("Sync() = %v", err)
	}
	select {
	case req := <-received:
		if req.App != "integration" || string(req.Data) != "hello" {
			t.Fatalf("received request = %+v, want integration/hello", req)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not receive a log request")
	}
}

func TestNewSinkRequiresApp(t *testing.T) {
	u, err := url.Parse("yarder+http://example.com")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := newSink(u); err == nil {
		t.Fatal("newSink() succeeded without an app")
	}
}

type yarderHandlerFunc func(context.Context, *connect.Request[yarder.LogReq]) (*connect.Response[emptypb.Empty], error)

func (f yarderHandlerFunc) Log(ctx context.Context, req *connect.Request[yarder.LogReq]) (*connect.Response[emptypb.Empty], error) {
	return f(ctx, req)
}
