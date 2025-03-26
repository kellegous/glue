package build

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
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

func ReadSummary() (*Summary, error) {
	if vcsInfo == "" {
		return &Summary{}, nil
	}

	rev, ts, ok := strings.Cut(vcsInfo, ",")
	if !ok {
		return nil, errors.New("invalid build info")
	}

	t, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return nil, errors.New("invalid timestamp")
	}

	return &Summary{
		SHA:        rev,
		CommitTime: time.Unix(t, 0),
		Name:       buildname.FromVersion(rev),
	}, nil
}

func SetVCSInfo(info string) {
	vcsInfo = info
}

func VCSInfo() string {
	return vcsInfo
}

func VCSInfoFromGit(ctx context.Context) (string, error) {
	c := exec.CommandContext(ctx, "git", "log", "-1", "--pretty=format:%H,%ct")
	c.Stderr = os.Stderr
	r, err := c.StdoutPipe()
	if err != nil {
		return "", err
	}
	defer r.Close()

	if err := c.Start(); err != nil {
		return "", err
	}

	b, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}

	if err := c.Wait(); err != nil {
		return "", err
	}

	return string(b), nil
}
