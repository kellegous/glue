package must

import (
	"errors"
	"testing"
)

func TestDo(t *testing.T) {
	tests := []struct {
		name      string
		fn        func() error
		wantPanic bool
	}{
		{
			name:      "nil error",
			fn:        func() error { return nil },
			wantPanic: false,
		},
		{
			name:      "non-nil error",
			fn:        func() error { return errors.New("close failed") },
			wantPanic: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				switch {
				case tt.wantPanic && r == nil:
					t.Fatal("expected panic")
				case !tt.wantPanic && r != nil:
					t.Fatalf("unexpected panic: %v", r)
				}
			}()
			Do(tt.fn)
		})
	}
}

func TestBeOK(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantPanic bool
	}{
		{
			name:      "nil error",
			err:       nil,
			wantPanic: false,
		},
		{
			name:      "non-nil error",
			err:       errors.New("failed"),
			wantPanic: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				switch {
				case tt.wantPanic && r == nil:
					t.Fatal("expected panic")
				case !tt.wantPanic && r != nil:
					t.Fatalf("unexpected panic: %v", r)
				}
			}()
			BeOK(tt.err)
		})
	}
}

func TestGetValue(t *testing.T) {
	tests := []struct {
		name      string
		value     int
		err       error
		want      int
		wantPanic bool
	}{
		{
			name:      "nil error returns value",
			value:     7,
			err:       nil,
			want:      7,
			wantPanic: false,
		},
		{
			name:      "non-nil error panics",
			value:     0,
			err:       errors.New("load failed"),
			want:      0,
			wantPanic: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				switch {
				case tt.wantPanic && r == nil:
					t.Fatal("expected panic")
				case !tt.wantPanic && r != nil:
					t.Fatalf("unexpected panic: %v", r)
				}
			}()
			got := GetValue(tt.value, tt.err)
			if !tt.wantPanic && got != tt.want {
				t.Fatalf("GetValue() = %v, want %v", got, tt.want)
			}
		})
	}
}
