package devmode

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"time"

	"github.com/kellegous/glue/build"
)

type ViteOption func(*ViteOptions)

type ViteOptions struct {
	env []string
}

func WithEnv(key string, val string) ViteOption {
	return func(o *ViteOptions) {
		o.env = append(o.env, fmt.Sprintf("%s=%s", key, val))
	}
}

// Deprecated: Use WithBuildSummary instead.
func WithBuildInfo(info *build.Summary) ViteOption {
	return WithBuildSummary(info)
}

func WithBuildSummary(summary *build.Summary) ViteOption {
	return func(o *ViteOptions) {
		o.env = append(o.env,
			fmt.Sprintf("SHA=%s", summary.SHA),
			fmt.Sprintf("BUILD_NAME=%s", summary.Name),
			fmt.Sprintf("COMMIT_TIME=%s", summary.CommitTime.Format(time.RFC3339)),
		)
	}
}

// AssetsFromVite starts a vite server and returns a handler that proxies
// requests to the vite server.
func AssetsFromVite(
	ctx context.Context,
	flag *Flag,
	opts ...ViteOption,
) (http.Handler, error) {
	if !flag.IsEnabled() {
		return nil, fmt.Errorf("devmode flag is not set")
	}

	var options ViteOptions
	for _, opt := range opts {
		opt(&options)
	}

	c := exec.CommandContext(
		ctx,
		"node_modules/.bin/vite",
		"--clearScreen=false",
		fmt.Sprintf("--port=%d", flag.Port))
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Dir = flag.Root
	env := os.Environ()
	env = append(env, options.env...)
	c.Env = env
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
