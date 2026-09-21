package main

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/kellegous/poop"
)

func main() {
	if err := run(context.Background()); err != nil {
		poop.HitFan(err)
	}
}

func run(ctx context.Context) error {
	latest, err := latestTag(ctx)
	if err != nil {
		return poop.Chain(err)
	}

	next := latest.Next()
	if err := exec.CommandContext(ctx, "git", "tag", next.String()).Run(); err != nil {
		return poop.Chain(err)
	}

	fmt.Printf("Tag: %s\n", next)
	fmt.Println("To push the new tag to origin, run:\ngit push origin --tags")
	return nil
}

type version struct {
	major int
	minor int
	patch int
}

func (v version) Next() version {
	return version{
		major: v.major,
		minor: v.minor + 1,
		patch: 0,
	}
}

func (v version) String() string {
	return fmt.Sprintf("v%d.%d.%d", v.major, v.minor, v.patch)
}

var releaseTag = regexp.MustCompile(`^v(?P<major>[0-9]+)\.(?P<minor>[0-9]+)\.(?P<patch>[0-9]+)$`)

func latestTag(ctx context.Context) (version, error) {
	output, err := exec.CommandContext(ctx, "git", "tag", "--sort=-version:refname").Output()
	if err != nil {
		return version{}, poop.Chain(err)
	}

	for _, tag := range strings.Fields(string(output)) {
		matches := releaseTag.FindStringSubmatch(tag)
		if matches == nil {
			continue
		}

		major, err := strconv.Atoi(matches[1])
		if err != nil {
			return version{}, poop.Chain(err)
		}
		minor, err := strconv.Atoi(matches[2])
		if err != nil {
			return version{}, poop.Chain(err)
		}
		patch, err := strconv.Atoi(matches[3])
		if err != nil {
			return version{}, poop.Chain(err)
		}
		return version{major: major, minor: minor, patch: patch}, nil
	}

	return version{}, poop.New("no release tags matching vMAJOR.MINOR.PATCH found")
}
