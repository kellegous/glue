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
		name string
		raw  string
		want string
	}{
		{name: "http", raw: "yarder+http://example.com/logs?app=test", want: "http://example.com/logs"},
		{name: "https", raw: "yarder+https://example.com/logs?app=test", want: "https://example.com/logs"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.raw)
			if err != nil {
				t.Fatal(err)
			}
			got, err := getRpcURL(u)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("getRpcURL() = %q, want %q", got, tt.want)
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

func TestGetIntAndDuration(t *testing.T) {
	if got, err := getInt("", 10); err != nil || got != 10 {
		t.Fatalf("getInt(empty) = (%d, %v), want (10, nil)", got, err)
	}
	if got, err := getInt("42", 10); err != nil || got != 42 {
		t.Fatalf("getInt(valid) = (%d, %v), want (42, nil)", got, err)
	}
	if _, err := getInt("nope", 10); err == nil {
		t.Fatal("getInt(invalid) succeeded")
	}

	if got, err := getDuration("", time.Second); err != nil || got != time.Second {
		t.Fatalf("getDuration(empty) = (%s, %v), want (1s, nil)", got, err)
	}
	if got, err := getDuration("250ms", time.Second); err != nil || got != 250*time.Millisecond {
		t.Fatalf("getDuration(valid) = (%s, %v), want (250ms, nil)", got, err)
	}
	if _, err := getDuration("soon", time.Second); err == nil {
		t.Fatal("getDuration(invalid) succeeded")
	}
}

func TestSinkWriteCloseAndSync(t *testing.T) {
	s := &sink{buffer: newCircBuffer[[]byte](2), changed: make(chan struct{}, 1), drainTimeout: time.Second}

	if got, err := s.Write([]byte("first")); err != nil || got != len("first") {
		t.Fatalf("Write() = (%d, %v), want (5, nil)", got, err)
	}
	data, ok := s.buffer.Pop()
	if !ok || string(data) != "first" {
		t.Fatalf("buffer.Pop() = (%q, %t), want (first, true)", data, ok)
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
	s := &sink{buffer: newCircBuffer[[]byte](1), changed: make(chan struct{}, 1), drainTimeout: time.Millisecond}
	if err := s.Sync(); err == nil {
		t.Fatal("Sync() succeeded without a drain acknowledgement")
	}
}

func TestSinkCloseWakesWorker(t *testing.T) {
	s := &sink{buffer: newCircBuffer[[]byte](1), changed: make(chan struct{}, 1)}
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
	s := &sink{buffer: newCircBuffer[[]byte](3), drained: make(chan struct{})}
	s.buffer.Push([]byte("one"))
	s.buffer.Push([]byte("two"))
	client := &recordingYarderClient{}

	if keepRunning := s.deliverPending(context.Background(), client, "test-app"); !keepRunning {
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
	}
	if s.drained != nil {
		t.Fatal("drain subscription was not cleared")
	}
}

func TestDeliverRetriesTransientFailures(t *testing.T) {
	client := &recordingYarderClient{errors: []error{errors.New("temporary failure")}}
	if err := deliver(context.Background(), client, &yarder.LogReq{App: "test", Data: []byte("entry")}); err != nil {
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
