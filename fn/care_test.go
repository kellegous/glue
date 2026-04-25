package fn

import (
	"errors"
	"testing"
)

func TestWithCare(t *testing.T) {
	t.Parallel()

	existingErr := errors.New("existing error")
	newErr := errors.New("new error")

	tests := []struct {
		Name     string
		Fn       func() error
		Err      error
		Validate func(t *testing.T, err error)
	}{
		{
			Name: "both nil",
			Fn: func() error {
				return nil
			},
			Err: nil,
			Validate: func(t *testing.T, err error) {
				t.Helper()
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
			},
		},
		{
			Name: "only function returns error",
			Fn: func() error {
				return newErr
			},
			Err: nil,
			Validate: func(t *testing.T, err error) {
				t.Helper()
				if !errors.Is(err, newErr) {
					t.Fatalf("expected errors.Is(err, %v) to be true", newErr)
				}
			},
		},
		{
			Name: "only existing error set",
			Fn: func() error {
				return nil
			},
			Err: existingErr,
			Validate: func(t *testing.T, err error) {
				t.Helper()
				if !errors.Is(err, existingErr) {
					t.Fatalf("expected errors.Is(err, %v) to be true", existingErr)
				}
			},
		},
		{
			Name: "joins existing and function errors",
			Fn: func() error {
				return newErr
			},
			Err: existingErr,
			Validate: func(t *testing.T, err error) {
				t.Helper()
				for _, want := range []error{existingErr, newErr} {
					if !errors.Is(err, want) {
						t.Fatalf("expected joined error to contain %v, got %v", want, err)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()

			err := tt.Err
			WithCare(tt.Fn, &err)
			tt.Validate(t, err)
		})
	}
}
