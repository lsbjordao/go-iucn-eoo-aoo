package server

import (
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
