// Package server exposes a bounded, stateless HTTP interface for local stacks.
package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	geo "github.com/lsbjordao/go-iucn-eoo-aoo"
	"github.com/lsbjordao/go-iucn-eoo-aoo/internal/input"
	"io"
	"mime"
	"net/http"
	"time"
)

type request struct {
	Points json.RawMessage `json:"points"`
	Config geo.Config      `json:"config"`
}

func send(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// NewHandler runs at most workers calculations simultaneously. token may be
// empty for loopback use; a configured token protects the calculation endpoint.
func NewHandler(workers int, token string) http.Handler {
	if workers < 1 {
		workers = 1
	}
	if workers > 32 {
		workers = 32
	}
	sem := make(chan struct{}, workers)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		send(w, 200, map[string]string{"status": "ok", "version": geo.Version, "gdal": geo.GDALVersion()})
	})
	mux.HandleFunc("POST /v1/calculate", func(w http.ResponseWriter, r *http.Request) {
		if token != "" && subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+token)) != 1 {
			send(w, 401, map[string]string{"error": "unauthorized"})
			return
		}
		mediaType, _, mediaErr := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if mediaErr != nil || mediaType != "application/json" {
			send(w, 415, map[string]string{"error": "Content-Type must be application/json"})
			return
		}
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
		default:
			send(w, 503, map[string]string{"error": "calculation capacity reached; retry later"})
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 16<<20)
		d := json.NewDecoder(r.Body)
		d.DisallowUnknownFields()
		var req request
		if e := d.Decode(&req); e != nil {
			status := 400
			var limitErr *http.MaxBytesError
			if errors.As(e, &limitErr) {
				status = 413
			}
			send(w, status, map[string]string{"error": e.Error()})
			return
		}
		var trailing any
		if e := d.Decode(&trailing); e != io.EOF {
			send(w, 400, map[string]string{"error": "expected one JSON request"})
			return
		}
		data, e := input.Read(req.Points, "json", input.Options{})
		if e != nil {
			send(w, 400, map[string]string{"error": e.Error()})
			return
		}
		if data.GeoJSON && req.Config.InputCRS != "" && req.Config.InputCRS != "EPSG:4326" {
			send(w, 400, map[string]string{"error": "RFC7946 GeoJSON requires EPSG:4326 input"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		result, e := geo.Calculate(ctx, data.Points, req.Config)
		if e != nil {
			send(w, 422, map[string]string{"error": e.Error()})
			return
		}
		if r.URL.Query().Get("details") != "1" {
			result = result.Summary()
		}
		send(w, 200, result)
	})
	return mux
}
