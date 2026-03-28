package devmode

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
)

//go:embed banner.ans
var devmodeBanner []byte

// Flag provides a flag to configure the devmode where the application
// assets are proxied to a vite server.
type Flag struct {
	Root string
	Port int
}

func (f *Flag) viteURL() *url.URL {
	return &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("localhost:%d", f.Port),
		Path:   "/",
	}
}

func toAppURL(addr string) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", err
	}

	if host == "" {
		host = "localhost"
	}

	return fmt.Sprintf("http://%s:%s/", host, port), nil
}

// PrintBanner prints the devmode banner to the writer.
func (f *Flag) PrintBanner(w io.Writer, appAddr string) error {
	cyan := color.New(color.FgCyan).SprintFunc()

	appURL, err := toAppURL(appAddr)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "\n%s\n", devmodeBanner); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "App:    %s\n", cyan(appURL)); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "Vite:   %s\n", cyan(f.viteURL().String())); err != nil {
		return err
	}

	return nil
}

func isAlive(ctx context.Context, url string) (bool, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodHead,
		url,
		nil,
	)
	if err != nil {
		return false, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, nil
	}
	defer res.Body.Close()

	return res.StatusCode == http.StatusOK, nil
}

// WaitForReady waits for the vite server to be ready.
func (f *Flag) WaitForReady(ctx context.Context) error {
	url := f.viteURL().String()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}

		if alive, err := isAlive(ctx, url); err != nil {
			return err
		} else if alive {
			return nil
		}
	}
}

// ShowBannerWhenReady waits for the vite server to be ready and then prints the banner.
// If devmode is not enabled, this is a no-op.
func (f *Flag) ShowBannerWhenReady(
	ctx context.Context,
	w io.Writer,
	appAddr string,
) error {
	if !f.IsEnabled() {
		return nil
	}

	if err := f.WaitForReady(ctx); err != nil {
		return err
	}

	return f.PrintBanner(w, appAddr)
}

// IsEnabled returns true if the flag is enabled.
func (f *Flag) IsEnabled() bool {
	return f.Port > 0 && f.Root != ""
}

func (f *Flag) Set(v string) error {
	root, ps, ok := strings.Cut(v, ":")
	if !ok {
		root = "."
		ps = v
	}
	port, err := strconv.Atoi(ps)
	if err != nil {
		return err
	}
	f.Port = port
	f.Root = root
	return nil
}

func (f *Flag) String() string {
	return fmt.Sprintf("%s:%d", f.Root, f.Port)
}

func (f *Flag) Type() string {
	return "root:port"
}
