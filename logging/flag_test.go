package logging

import (
	"reflect"
	"strings"
	"testing"

	"go.uber.org/zap/zapcore"
)

func TestLevelFlagSet(t *testing.T) {
	type Expected struct {
		Level zapcore.Level
		Error bool
	}

	for _, tt := range []struct {
		Name     string
		Input    string
		Expected Expected
	}{
		{
			Name:  "debug",
			Input: "debug",
			Expected: Expected{
				Level: zapcore.DebugLevel,
			},
		},
		{
			Name:  "case insensitive",
			Input: "WARN",
			Expected: Expected{
				Level: zapcore.WarnLevel,
			},
		},
		{
			Name:  "invalid",
			Input: "verbose",
			Expected: Expected{
				Level: zapcore.InfoLevel,
				Error: true,
			},
		},
	} {
		t.Run(tt.Name, func(t *testing.T) {
			flag := &LevelFlag{level: zapcore.InfoLevel}

			err := flag.Set(tt.Input)
			if (err != nil) != tt.Expected.Error {
				t.Fatalf("Set(%q) error = %v, want error: %t", tt.Input, err, tt.Expected.Error)
			}
			if got := flag.Level(); got != tt.Expected.Level {
				t.Errorf("Level() = %s, want %s", got, tt.Expected.Level)
			}
			if got, want := flag.String(), tt.Expected.Level.String(); got != want {
				t.Errorf("String() = %q, want %q", got, want)
			}
		})
	}
}

func TestLevelFlagType(t *testing.T) {
	for _, tt := range []struct {
		Name     string
		Flag     *LevelFlag
		Expected string
	}{
		{Name: "level", Flag: &LevelFlag{}, Expected: "level"},
	} {
		t.Run(tt.Name, func(t *testing.T) {
			if got := tt.Flag.Type(); got != tt.Expected {
				t.Errorf("Type() = %q, want %q", got, tt.Expected)
			}
		})
	}
}

func TestOutputPathsFlag(t *testing.T) {
	type Expected struct {
		Paths []string
	}

	for _, tt := range []struct {
		Name     string
		Flag     OutputPathsFlag
		Inputs   []string
		Expected Expected
	}{
		{
			Name:     "constructor removes duplicates",
			Flag:     NewOutputPathsFlag("stdout", "stderr", "stdout"),
			Expected: Expected{Paths: []string{"stderr", "stdout"}},
		},
		{
			Name:     "set adds distinct paths",
			Flag:     NewOutputPathsFlag("stdout"),
			Inputs:   []string{"stderr", "stdout"},
			Expected: Expected{Paths: []string{"stderr", "stdout"}},
		},
		{
			Name:     "set initializes a zero value",
			Flag:     OutputPathsFlag{},
			Inputs:   []string{"file.log"},
			Expected: Expected{Paths: []string{"file.log"}},
		},
	} {
		t.Run(tt.Name, func(t *testing.T) {
			for _, input := range tt.Inputs {
				if err := tt.Flag.Set(input); err != nil {
					t.Fatalf("Set(%q) error = %v, want nil", input, err)
				}
			}

			gotString := tt.Flag.String()
			gotPaths := strings.Split(gotString, ",")
			if gotString == "" {
				gotPaths = nil
			}
			if !reflect.DeepEqual(asSet(gotPaths), asSet(tt.Expected.Paths)) {
				t.Errorf("String() paths = %v, want %v", gotPaths, tt.Expected.Paths)
			}
		})
	}
}

func TestOutputPathsFlagType(t *testing.T) {
	for _, tt := range []struct {
		Name     string
		Flag     *OutputPathsFlag
		Expected string
	}{
		{Name: "path", Flag: &OutputPathsFlag{}, Expected: "path"},
	} {
		t.Run(tt.Name, func(t *testing.T) {
			if got := tt.Flag.Type(); got != tt.Expected {
				t.Errorf("Type() = %q, want %q", got, tt.Expected)
			}
		})
	}
}

func asSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}
