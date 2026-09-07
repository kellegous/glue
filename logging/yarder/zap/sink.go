package zap

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"github.com/avast/retry-go"
	"github.com/kellegous/poop"
	"go.uber.org/zap"

	"github.com/kellegous/glue/logging/yarder"
	"github.com/kellegous/glue/logging/yarder/yarder_connect"
)

const (
	httpScheme       = "yarder+http"
	httpsScheme      = "yarder+https"
	defaultQueueSize = 100

	initialRetryDelay = 100 * time.Millisecond
	maxRetryDelay     = time.Minute
	requestTimeout    = time.Minute
)

type sink struct {
	buffer *circBuffer[[]byte]
}

func (s *sink) Write(p []byte) (int, error) {
	s.buffer.Push(p)
	return len(p), nil
}

func (s *sink) Close() error {
	return nil
}

func (s *sink) Sync() error {
	// TODO(kellegous): This should flush the buffer.
	return nil
}

func newSink(u *url.URL) (zap.Sink, error) {
	rpcURL, err := getRpcURL(u)
	if err != nil {
		return nil, poop.Chain(err)
	}

	q := u.Query()

	queueSize, err := getInt(q.Get("queue-size"), defaultQueueSize)
	if err != nil {
		return nil, poop.Chain(err)
	}

	app := q.Get("app")
	if app == "" {
		return nil, poop.New("app is required")
	}

	buffer := newCircBuffer[[]byte](queueSize)

	client := yarder_connect.NewYarderClient(http.DefaultClient, rpcURL)

	go func() {
		for {
			if err := deliver(context.Background(), client, &yarder.LogReq{
				App:  app,
				Data: buffer.Pop(),
			}); err != nil {
				continue
			}
		}
	}()

	return &sink{buffer: buffer}, nil
}

func getInt(v string, def int) (int, error) {
	if v == "" {
		return def, nil
	}
	limit, err := strconv.Atoi(v)
	if err != nil {
		return 0, poop.Chain(err)
	}
	return limit, nil
}

// func getDuration(v string, def time.Duration) (time.Duration, error) {
// 	if v == "" {
// 		return def, nil
// 	}
// 	duration, err := time.ParseDuration(v)
// 	if err != nil {
// 		return 0, poop.Chain(err)
// 	}
// 	return duration, nil
// }

func deliver(ctx context.Context, client yarder_connect.YarderClient, req *yarder.LogReq) error {
	return retry.Do(
		func() error {
			ctx, done := context.WithTimeout(ctx, requestTimeout)
			defer done()

			_, err := client.Log(ctx, connect.NewRequest(req))
			return poop.Chain(err)
		},
		retry.Context(ctx),
		retry.Attempts(0), // Retry indefinitely.
		retry.Delay(initialRetryDelay),
		retry.MaxDelay(maxRetryDelay),
		retry.DelayType(retry.BackOffDelay),
		retry.LastErrorOnly(true),
	)
}

func getRpcURL(u *url.URL) (string, error) {
	ru := &url.URL{
		Host: u.Host,
		Path: u.Path,
	}

	switch u.Scheme {
	case httpScheme:
		ru.Scheme = "http"
	case httpsScheme:
		ru.Scheme = "https"
	default:
		return "", poop.Newf("invalid scheme: %s", u.Scheme)
	}

	return ru.String(), nil
}

func Register() error {
	if err := zap.RegisterSink(httpScheme, newSink); err != nil {
		return poop.Chain(err)
	}
	if err := zap.RegisterSink(httpsScheme, newSink); err != nil {
		return poop.Chain(err)
	}
	return nil
}
