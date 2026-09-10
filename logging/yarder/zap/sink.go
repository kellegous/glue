package zap

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/avast/retry-go/v4"
	"github.com/kellegous/poop"
	"go.uber.org/zap"

	"github.com/kellegous/glue/logging/yarder"
	"github.com/kellegous/glue/logging/yarder/yarder_connect"
)

const (
	httpScheme          = "yarder+http"
	httpsScheme         = "yarder+https"
	defaultQueueSize    = 100
	defaultDrainTimeout = time.Minute
	defaultRetryLimit   = 5

	initialRetryDelay = 100 * time.Millisecond
	maxRetryDelay     = 10 * time.Second
	requestTimeout    = 30 * time.Second
)

type sink struct {
	lck          sync.RWMutex
	buffer       *circBuffer[[]byte]
	drainTimeout time.Duration
	retryLimit   uint
	drained      chan struct{}
	changed      chan struct{}
	closed       bool
}

func (s *sink) Write(p []byte) (int, error) {
	s.lck.Lock()
	defer s.lck.Unlock()

	if s.closed {
		return 0, io.ErrClosedPipe
	}

	s.buffer.Push(bytes.Clone(p))
	s.wake()
	return len(p), nil
}

func (s *sink) Close() error {
	s.lck.Lock()
	s.closed = true
	s.lck.Unlock()
	s.wake()
	return nil
}

func (s *sink) Sync() error {
	select {
	case <-s.subscribeForDrain():
		return nil
	case <-time.After(s.drainTimeout):
		return poop.New("drain timeout")
	}
}

func (s *sink) subscribeForDrain() <-chan struct{} {
	s.lck.Lock()
	defer s.lck.Unlock()

	if s.drained == nil {
		s.drained = make(chan struct{})
		s.wake()
	}

	return s.drained
}

func (s *sink) ackDrain() {
	s.lck.Lock()
	defer s.lck.Unlock()

	close(s.drained)
	s.drained = nil
}

func newSink(u *url.URL) (zap.Sink, error) {
	rpcURL, err := getRpcURL(u)
	if err != nil {
		return nil, poop.Chain(err)
	}

	q := u.Query()

	app := q.Get("app")
	if app == "" {
		return nil, poop.New("app is required")
	}

	queueSize, err := getInt(q.Get("queue-size"), defaultQueueSize)
	if err != nil {
		return nil, poop.Chain(err)
	}

	drainTimeout, err := getDuration(q.Get("drain-timeout"), defaultDrainTimeout)
	if err != nil {
		return nil, poop.Chain(err)
	}

	retryLimit, err := getUint(q.Get("retry-limit"), defaultRetryLimit)
	if err != nil {
		return nil, poop.Chain(err)
	}

	buffer := newCircBuffer[[]byte](queueSize)

	client := yarder_connect.NewYarderClient(http.DefaultClient, rpcURL)

	ctx := context.Background()

	s := &sink{
		buffer:       buffer,
		drainTimeout: drainTimeout,
		retryLimit:   retryLimit,
		changed:      make(chan struct{}, 1),
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.changed:
			}

			if !s.deliverPending(ctx, client, app) {
				return
			}
		}
	}()

	return s, nil
}

func (s *sink) wake() {
	select {
	case s.changed <- struct{}{}:
	default:
	}
}

func (s *sink) deliverPending(
	ctx context.Context,
	client yarder_connect.YarderClient,
	app string,
) bool {
	for {
		data, ok, draining, closed := func() ([]byte, bool, bool, bool) {
			s.lck.Lock()
			defer s.lck.Unlock()
			data, ok := s.buffer.Pop()
			return data, ok, s.drained != nil, s.closed
		}()

		// TODO(kellegous): What happens here if the context is cancelled? Does this keep retrying?
		if ok {
			if err := deliver(
				ctx,
				client,
				&yarder.LogReq{
					App:  app,
					Data: data,
				},
				s.retryLimit,
			); err != nil {
				continue
			}
		} else {
			if draining {
				s.ackDrain()
			}

			return !closed
		}
	}
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

func getUint(v string, def uint) (uint, error) {
	if v == "" {
		return def, nil
	}
	limit, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return 0, poop.Chain(err)
	}
	return uint(limit), nil
}

func getDuration(v string, def time.Duration) (time.Duration, error) {
	if v == "" {
		return def, nil
	}
	duration, err := time.ParseDuration(v)
	if err != nil {
		return 0, poop.Chain(err)
	}
	return duration, nil
}

func deliver(
	ctx context.Context,
	client yarder_connect.YarderClient,
	req *yarder.LogReq,
	retryLimit uint,
) error {
	return retry.Do(
		func() error {
			ctx, done := context.WithTimeout(ctx, requestTimeout)
			defer done()

			_, err := client.Log(ctx, connect.NewRequest(req))
			return poop.Chain(err)
		},
		retry.Context(ctx),
		retry.Attempts(retryLimit),
		retry.Delay(initialRetryDelay),
		retry.MaxDelay(maxRetryDelay),
		retry.DelayType(retry.BackOffDelay),
		retry.LastErrorOnly(true),
		retry.RetryIf(func(err error) bool {
			return !errors.Is(err, context.Canceled)
		}),
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
