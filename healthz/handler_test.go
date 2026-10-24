package healthz_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kellegous/glue/build"
	"github.com/kellegous/glue/healthz"
)

func TestNewHandler(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		Name      string
		StartedAt time.Time
	}{
		{
			Name:      "startup time with offset and nanoseconds",
			StartedAt: time.Date(2026, time.October, 24, 12, 34, 56, 123456789, time.FixedZone("offset", -4*60*60)),
		},
		{
			Name: "zero startup time",
		},
	} {
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			healthz.NewHandler(tt.StartedAt).ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type: got %q, want application/json", got)
			}

			var got map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			summary := build.ReadSummary()
			want := map[string]string{
				"sha":         summary.SHA,
				"commit_time": summary.CommitTime.Format(time.RFC3339Nano),
				"name":        summary.Name,
				"started_at":  tt.StartedAt.Format(time.RFC3339Nano),
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("response: got %v, want %v", got, want)
			}
		})
	}
}

func TestNewHandlerEncodingError(t *testing.T) {
	t.Parallel()

	// JSON timestamps must have a year in the range [0, 9999].
	startedAt := time.Date(10000, time.January, 1, 0, 0, 0, 0, time.UTC)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	healthz.NewHandler(startedAt).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if got := rec.Body.String(); !strings.Contains(got, "year outside of range [0,9999]") {
		t.Errorf("body: got %q, want timestamp encoding error", got)
	}
}
