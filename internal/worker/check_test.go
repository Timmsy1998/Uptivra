package worker

import (
	"context"
	"github.com/timmsy1998/uptivra/internal/domain"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestExecution(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/slow" {
			time.Sleep(1100 * time.Millisecond)
		}
		w.Write([]byte("ready"))
	}))
	defer s.Close()
	m := domain.Monitor{URL: s.URL, Timeout: 1, ExpectedStatus: 200, Contains: "ready"}
	if c := Execute(context.Background(), m, s.Client()); !c.OK {
		t.Fatal(c)
	}
	m.Contains = "missing"
	if c := Execute(context.Background(), m, s.Client()); c.OK || c.Reason != "content_mismatch" {
		t.Fatal(c)
	}
	m.URL += "/slow"
	if c := Execute(context.Background(), m, s.Client()); c.OK || c.Reason != "timeout" {
		t.Fatal(c)
	}
}
func TestGuardedExecution(t *testing.T) {
	m := domain.Monitor{URL: "http://127.0.0.1", Timeout: 1, ExpectedStatus: 200}
	if c := Check(context.Background(), m); c.OK {
		t.Fatal("loopback allowed")
	}
}
