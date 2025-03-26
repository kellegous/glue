package build

import (
	"runtime/debug"
	"time"

	"github.com/kellegous/buildname"
)

type Summary struct {
	SHA        string    `json:"sha"`
	CommitTime time.Time `json:"commit_time"`
	Name       string    `json:"name"`
}

func ReadSummary() (*Summary, error) {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return &Summary{}, nil
	}

	sha, t, err := vcsInfoFrom(bi.Settings)
	if err != nil {
		return nil, err
	}

	return &Summary{
		SHA:        sha,
		CommitTime: t,
		Name:       buildname.FromVersion(sha),
	}, nil
}

func vcsInfoFrom(settings []debug.BuildSetting) (string, time.Time, error) {
	var sha string
	var t time.Time
	for _, setting := range settings {
		switch setting.Key {
		case "vcs.revision":
			sha = setting.Value
		case "vcs.time":
			var err error
			t, err = time.Parse(time.RFC3339, setting.Value)
			if err != nil {
				return "", time.Time{}, err
			}
		}
	}
	return sha, t, nil
}
