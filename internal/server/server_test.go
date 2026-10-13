package server

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAPI(t *testing.T) {
	h := NewHandler(2, "test-token")
	for _, tc := range []struct {
		body, token string
		status      int
	}{
		{`{"points":[{"lon":-43,"lat":-22}]}`, "test-token", 200},
		{`{"points":[{"lon":-43,"lat":-22}]}`, "wrong", 401},
		{`{"points":[{"lat":-22}]}`, "test-token", 400},
		{`{"points":[{"lon":181,"lat":0}]}`, "test-token", 422},
		{`{"points":[{"lon":0,"lat":0}],"config":{"grid_mdoe":"exact"}}`, "test-token", 400},
		{`{"points":[{"lon":0,"lat":0}]} {}`, "test-token", 400},
	} {
		req := httptest.NewRequest("POST", "/v1/calculate", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tc.token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != tc.status {
			t.Fatalf("status %d expected %d: %s", w.Code, tc.status, w.Body.String())
		}
	}
	q := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, q)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
}

func TestConcurrentCalculations(t *testing.T) {
	h := NewHandler(8, "")
	for i := 0; i < 8; i++ {
		t.Run("request", func(t *testing.T) {
			t.Parallel()
			q := httptest.NewRequest("POST", "/v1/calculate", strings.NewReader(`{"points":[{"lon":10,"lat":80},{"lon":10.1,"lat":80},{"lon":10,"lat":80.1}]}`))
			q.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, q)
			if w.Code != 200 {
				t.Fatal(w.Body.String())
			}
		})
	}
}

func TestAPIProtocolReproduction(t *testing.T) {
	h := NewHandler(2, "")
	post := func(body string) *httptest.ResponseRecorder {
		q := httptest.NewRequest("POST", "/v1/calculate", strings.NewReader(body))
		q.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, q)
		return w
	}
	points := `[{"lon":-43.2,"lat":-22.9},{"lon":-44.3,"lat":-21.5},{"lon":-42.9,"lat":-20.8}]`
	w := post(`{"points":` + points + `}`)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var first struct {
		ProtocolSHA256 string          `json:"protocol_sha256"`
		Protocol       json.RawMessage `json:"protocol"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &first); err != nil || len(first.Protocol) == 0 {
		t.Fatalf("%v %s", err, w.Body.String())
	}
	w = post(`{"points":` + points + `,"protocol":` + string(first.Protocol) + `}`)
	if w.Code != 200 || !strings.Contains(w.Body.String(), first.ProtocolSHA256) || !strings.Contains(w.Body.String(), "FROZEN_PROTOCOL") {
		t.Fatal(w.Body.String())
	}
	if w = post(`{"points":` + points + `,"config":{"cell_size_m":1000},"protocol":` + string(first.Protocol) + `}`); w.Code != 422 {
		t.Fatalf("conflicting config accepted: %d %s", w.Code, w.Body.String())
	}
}
