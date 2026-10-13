package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchStatus(t *testing.T) {
	for _, tc := range []struct {
		body string
		code int
		ok   bool
	}{
		{`{"title":"API","status":"healthy","updated_at":"2026-10-13T10:00:00Z"}`, 200, true},
		{`{"status":"healthy"}`, 200, false},
		{`<html>unavailable</html>`, 503, false},
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v1/public/status" {
				t.Errorf("unexpected endpoint %s", r.URL.Path)
			}
			w.WriteHeader(tc.code)
			fmt.Fprint(w, tc.body)
		}))
		_, err := FetchStatus(context.Background(), server.URL)
		server.Close()
		if (err == nil) != tc.ok {
			t.Fatalf("body %s error %v", tc.body, err)
		}
	}
}
