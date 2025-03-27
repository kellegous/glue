package build

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/kellegous/buildname"
)

var vcsInfo string

type Summary struct {
	SHA        string    `json:"sha"`
	CommitTime time.Time `json:"commit_time"`
	Name       string    `json:"name"`
}

func (s *Summary) EncodeToString() (string, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(b), nil
}

func ReadSummary() *Summary {
	s, err := fromBuildInfo()
	if err == nil {
		return s
	}

	s, err = fromLDFlags()
	if err == nil {
		return s
	}

	return &Summary{}
}

func fromLDFlags() (*Summary, error) {
	j, err := base64.StdEncoding.DecodeString(vcsInfo)
	if err != nil {
		return nil, err
	}

	var s Summary
	if err := json.Unmarshal(j, &s); err != nil {
		return nil, err
	}

	return &s, nil
}

func fromBuildInfo() (*Summary, error) {
	var sha string
	var t time.Time

	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return nil, fmt.Errorf("unable to read build info")
	}

	for _, setting := range bi.Settings {
		switch setting.Key {
		case "vcs.revision":
			sha = setting.Value
		case "vcs.time":
			var err error
			t, err = time.Parse(time.RFC3339, setting.Value)
			if err != nil {
				return nil, err
			}
		}
	}

	return &Summary{
		SHA:        sha,
		CommitTime: t,
		Name:       buildname.FromVersion(sha),
	}, nil
}

func ReadSummaryFromGit(ctx context.Context) (*Summary, error) {
	c := exec.CommandContext(
		ctx,
		"git",
		"-c",
		"log.showsignature=false",
		"log",
		"-1",
		"--format=%H:%ct")
	c.Stderr = os.Stderr
	r, err := c.StdoutPipe()
	if err != nil {
		return nil, err
	}
	defer r.Close()

	if err := c.Start(); err != nil {
		return nil, err
	}

	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	sha, ts, ok := strings.Cut(string(bytes.TrimSpace(b)), ":")
	if !ok {
		return nil, fmt.Errorf("unable to parse git log output")
	}

	t, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return nil, err
	}

	return &Summary{
		SHA:        sha,
		CommitTime: time.Unix(t, 0),
		Name:       buildname.FromVersion(sha),
	}, nil
}
