package logging

import (
	"maps"
	"slices"
	"strings"

	"github.com/kellegous/poop"
	"go.uber.org/zap/zapcore"
)

// LevelFlag is a cli flag that allows the user to specify
// the zap logger level.
type LevelFlag struct {
	level zapcore.Level
}

// Level returns the level for the zap logger.
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

// OutputPathsFlag is a cli flag that allows the user to specify
// the zap logger output paths.
type OutputPathsFlag struct {
	paths map[string]bool
}

// NewOutputPathsFlag creates a new OutputPathsFlag with the given
// default paths.
func NewOutputPathsFlag(paths ...string) OutputPathsFlag {
	ps := make(map[string]bool)
	for _, path := range paths {
		ps[path] = true
	}
	return OutputPathsFlag{
		paths: ps,
	}
}

// Paths returns the output paths for the zap logger.
func (f *OutputPathsFlag) Paths() []string {
	return slices.Collect(maps.Keys(f.paths))
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
