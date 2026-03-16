package devmode

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
)

// AssetsFromVite starts a vite server and returns a handler that proxies
// requests to the vite server.
func AssetsFromVite(
	ctx context.Context,
	flag *Flag,
) (http.Handler, error) {
	if !flag.IsEnabled() {
		return nil, fmt.Errorf("devmode flag is not set")
	}

	c := exec.CommandContext(
		ctx,
		"node_modules/.bin/vite",
		"--clearScreen=false",
		fmt.Sprintf("--port=%d", flag.Port))
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Dir = flag.Root
	if err := c.Start(); err != nil {
		return nil, err
	}

	p := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(flag.viteURL())
			r.Out.Host = flag.viteURL().Host
		},
	}

	return p, nil
}
