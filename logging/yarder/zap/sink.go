package zap

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"connectrpc.com/connect"
	"github.com/avast/retry-go/v4"
	"github.com/kellegous/poop"
	"go.uber.org/zap"

	"github.com/kellegous/glue/logging/yarder"
	"github.com/kellegous/glue/logging/yarder/yarder_connect"
)

const (
	httpScheme  = "yarder+http"
	httpsScheme = "yarder+https"

	defaultQueueSize      = 100
	defaultDrainTimeout   = time.Minute
	defaultRetryLimit     = 5
	defaultBaseRetryDelay = 100 * time.Millisecond
	defaultMaxRetryDelay  = 10 * time.Second
	defaultRequestTimeout = 30 * time.Second
)

type sink struct {
	lck     sync.RWMutex
	buffer  *circBuffer[*yarder.LogReq]
	options *RegisterOptions
	drained chan struct{}
	changed chan struct{}
	closed  bool

	writerKey [16]byte
	writerSeq atomic.Uint64
}

func (s *sink) Write(p []byte) (int, error) {
	s.lck.Lock()
	defer s.lck.Unlock()

	if s.closed {
		return 0, io.ErrClosedPipe
	}

	s.buffer.Push(&yarder.LogReq{
		App:       s.options.app,
		Data:      bytes.Clone(p),
		WriterKey: s.writerKey[:],
		WriterSeq: s.writerSeq.Add(1),
	})
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
	case <-time.After(s.options.drainTimeout):
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

	options := globalRegisterOptions

	options.app = getString(q.Get("app"), options.app)

	options.queueSize, err = getInt(q.Get("queue-size"), options.queueSize)
	if err != nil {
		return nil, poop.Chain(err)
	}

	options.drainTimeout, err = getDuration(q.Get("drain-timeout"), options.drainTimeout)
	if err != nil {
		return nil, poop.Chain(err)
	}

	options.retryLimit, err = getUint(q.Get("retry-limit"), options.retryLimit)
	if err != nil {
		return nil, poop.Chain(err)
	}

	options.baseRetryDelay, err = getDuration(q.Get("base-retry-delay"), options.baseRetryDelay)
	if err != nil {
		return nil, poop.Chain(err)
	}

	options.maxRetryDelay, err = getDuration(q.Get("max-retry-delay"), options.maxRetryDelay)
	if err != nil {
		return nil, poop.Chain(err)
	}

	options.requestTimeout, err = getDuration(q.Get("request-timeout"), options.requestTimeout)
	if err != nil {
		return nil, poop.Chain(err)
	}

	if options.queueSize <= 0 {
		return nil, poop.New("queue size must be greater than 0")
	}

	buffer := newCircBuffer[*yarder.LogReq](options.queueSize)

	client := yarder_connect.NewYarderClient(http.DefaultClient, rpcURL)

	ctx := context.Background()

	s := &sink{
		buffer:  buffer,
		options: &options,
		changed: make(chan struct{}, 1),
	}

	if _, err := rand.Read(s.writerKey[:]); err != nil {
		return nil, poop.Chain(err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.changed:
			}

			if !s.deliverPending(ctx, client) {
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
) bool {
	for {
		req, ok, draining, closed := func() (*yarder.LogReq, bool, bool, bool) {
			s.lck.Lock()
			defer s.lck.Unlock()
			data, ok := s.buffer.Pop()
			return data, ok, s.drained != nil, s.closed
		}()

		// TODO(kellegous): What happens here if the context is cancelled? Does this keep retrying?
		if ok {
			if err := s.deliver(ctx, client, req); err != nil {
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

func getString(v string, def string) string {
	if v == "" {
		return def
	}
	return v
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

func (s *sink) deliver(
	ctx context.Context,
	client yarder_connect.YarderClient,
	req *yarder.LogReq,
) error {
	return retry.Do(
		func() error {
			ctx, done := context.WithTimeout(ctx, s.options.requestTimeout)
			defer done()

			_, err := client.Log(ctx, connect.NewRequest(req))
			return poop.Chain(err)
		},
		retry.Context(ctx),
		retry.Attempts(s.options.retryLimit),
		retry.Delay(s.options.baseRetryDelay),
		retry.MaxDelay(s.options.maxRetryDelay),
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
