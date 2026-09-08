package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"time"

	"connectrpc.com/connect"
	"github.com/kellegous/poop"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/kellegous/glue/fn"
	"github.com/kellegous/glue/logging"
	"github.com/kellegous/glue/logging/yarder"
	"github.com/kellegous/glue/logging/yarder/yarder_connect"
	yarder_zap "github.com/kellegous/glue/logging/yarder/zap"
)

func main() {
	if err := run(context.Background()); err != nil {
		poop.HitFan(err)
	}
}

func run(ctx context.Context) (err error) {
	if err := yarder_zap.Register(); err != nil {
		return poop.Chain(err)
	}

	ctx, done := signal.NotifyContext(ctx, os.Interrupt)
	defer done()

	addr, err := runServer(ctx)
	if err != nil {
		return poop.Chain(err)
	}

	params := url.Values{
		"app":           {"example"},
		"drain-timeout": {"3s"},
	}

	lg, err := logging.Setup(
		logging.WithOutputPaths(
			"stderr",
			fmt.Sprintf("yarder+http://%s?%s", addr, params.Encode()),
		),
	)
	if err != nil {
		return poop.Chain(err)
	}
	defer fn.WithAbandon(lg.Sync)

	fmt.Println("Server is running on", addr)

	go func() {
		for i := 0; ; i++ {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}

			lg.Info("sending log",
				zap.Int("seq", i),
				zap.Time("time", time.Now()))
		}
	}()

	<-ctx.Done()

	return nil
}

type service struct {
	rng *rand.Rand
}

var _ yarder_connect.YarderHandler = (*service)(nil)

func (s *service) Log(
	ctx context.Context,
	req *connect.Request[yarder.LogReq],
) (*connect.Response[emptypb.Empty], error) {
	msg := req.Msg

	if s.rng.Float64() > 0.5 {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("service is unavailable"))
	}

	data := struct {
		App string
		Log json.RawMessage
	}{
		App: msg.App,
		Log: json.RawMessage(msg.Data),
	}

	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, poop.Chain(err)
	}

	fmt.Printf("%s\n", b)

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func runServer(ctx context.Context) (string, error) {
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return "", poop.Chain(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	seed := uint64(time.Now().UnixNano())
	path, handler := yarder_connect.NewYarderHandler(&service{
		rng: rand.New(rand.NewPCG(seed, seed)),
	})
	mux.Handle(path, handler)

	s := &http.Server{
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		_ = s.Shutdown(context.Background())
	}()

	ch := make(chan error)
	go func() {
		if err := s.Serve(l); err != nil && err != http.ErrServerClosed {
			ch <- err
		}
	}()

	url := "http://" + l.Addr().String() + "/healthz"
	for {
		ready, err := isReady(ctx, url)
		if err != nil {
			return "", poop.Chain(err)
		}
		if ready {
			break
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}

	return l.Addr().String(), nil
}

func isReady(
	ctx context.Context,
	url string,
) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, poop.Chain(err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, poop.Chain(err)
	}
	defer fn.WithAbandon(res.Body.Close)

	return res.StatusCode == http.StatusOK, nil
}
