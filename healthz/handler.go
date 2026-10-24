package healthz

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/kellegous/glue/build"
)

const Path = "/healthz"

type healthzData struct {
	*build.Summary
	StartedAt time.Time `json:"started_at"`
}

func NewHandler(startedAt time.Time) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bs := build.ReadSummary()
		w.Header().Set("Content-Type", "application/json")
		data := healthzData{
			Summary:   bs,
			StartedAt: startedAt,
		}
		if err := json.NewEncoder(w).Encode(data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
