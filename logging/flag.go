package logging

import (
	"maps"
	"slices"
	"strings"

	"github.com/kellegous/poop"
	"go.uber.org/zap/zapcore"
)

type LevelFlag struct {
	level zapcore.Level
}

func (f *LevelFlag) Level() zapcore.Level {
	return f.level
}

func (f *LevelFlag) Set(s string) error {
	l, err := zapcore.ParseLevel(s)
	if err != nil {
		return poop.Chain(err)
	}
	f.level = l
	return err
}

func (f *LevelFlag) String() string {
	return f.level.String()
}

func (f *LevelFlag) Type() string {
	return "level"
}

type OutputPathsFlag struct {
	paths map[string]bool
}

func NewOutputPathsFlag(paths ...string) *OutputPathsFlag {
	f := &OutputPathsFlag{
		paths: make(map[string]bool),
	}
	for _, path := range paths {
		f.paths[path] = true
	}
	return f
}

func (f *OutputPathsFlag) Set(s string) error {
	if f.paths == nil {
		f.paths = make(map[string]bool)
	}
	f.paths[s] = true
	return nil
}

func (f *OutputPathsFlag) String() string {
	return strings.Join(slices.Collect(maps.Keys(f.paths)), ",")
}

func (f *OutputPathsFlag) Type() string {
	return "path"
}
