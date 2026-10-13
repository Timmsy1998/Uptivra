package worker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"uptivra/internal/store"
)

func TestWebhookSigningAndFailures(t *testing.T) {
	d := store.Delivery{ID: "delivery-id", Payload: json.RawMessage(`{"type":"incident.opened"}`), Channel: store.Channel{Kind: "webhook", SigningSecret: "secret"}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ts := r.Header.Get("X-Uptivra-Timestamp")
		if r.Header.Get("X-Uptivra-Signature") != Signature("secret", ts, d.Payload) {
			t.Error("bad signature")
		}
		if r.Header.Get("X-Uptivra-Delivery") != d.ID {
			t.Error("missing delivery id")
		}
		w.WriteHeader(204)
	}))
	defer srv.Close()
	d.Channel.URL = srv.URL
	if !Send(context.Background(), d, srv.Client()) {
		t.Fatal("delivery failed")
	}
	d.Channel.URL = ":"
	if Send(context.Background(), d, srv.Client()) {
		t.Fatal("bad destination accepted")
	}
}
