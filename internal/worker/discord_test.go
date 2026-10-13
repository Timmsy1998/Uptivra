package worker

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"uptivra/internal/domain"
	"uptivra/internal/store"
)

func TestDiscordIncidentEmbeds(t *testing.T) {
	at := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, kind, event, reason string
		ok                        bool
		color                     int
	}{
		{"http outage", "http", "incident.opened", "unexpected_status", false, 0xad3730},
		{"http recovery", "http", "incident.resolved", "", true, 0x286b4c},
		{"heartbeat outage", "heartbeat", "incident.opened", "heartbeat_missed", false, 0xad3730},
		{"heartbeat recovery", "heartbeat", "incident.resolved", "heartbeat_received", true, 0x286b4c},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := domain.IncidentEvent{ID: "event", Type: tc.event, MonitorID: "monitor", MonitorName: "API **prod** @everyone", IncidentID: "incident", Group: "Bots", MonitorType: tc.kind, At: at, OpenedAt: at.Add(-5 * time.Minute), Interval: 60, Grace: 30, FailureThreshold: 2, RecoveryThreshold: 2, ExpectedStatus: 200, Check: &domain.Check{At: at, OK: tc.ok, Code: 503, Latency: 42.5, Reason: tc.reason}}
			payload, err := json.Marshal(p)
			if err != nil {
				t.Fatal(err)
			}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				raw, err := io.ReadAll(io.LimitReader(r.Body, 16384))
				if err != nil {
					t.Error(err)
					return
				}
				var message struct {
					Username        string         `json:"username"`
					Embeds          []discordEmbed `json:"embeds"`
					AllowedMentions struct {
						Parse []string `json:"parse"`
					} `json:"allowed_mentions"`
				}
				if err := json.Unmarshal(raw, &message); err != nil {
					t.Error(err)
					return
				}
				if message.Username != "Uptivra" || len(message.Embeds) != 1 || message.AllowedMentions.Parse == nil || len(message.AllowedMentions.Parse) != 0 {
					t.Errorf("invalid Discord envelope: %s", raw)
					return
				}
				e := message.Embeds[0]
				if e.URL != "https://uptime.example.com/status" || !strings.Contains(e.Description, "[View status page — service status and incident updates](https://uptime.example.com/status)") {
					t.Errorf("missing status page link: %+v", e)
				}
				if e.Color != tc.color || e.Timestamp != at.Format(time.RFC3339) {
					t.Errorf("wrong status/time: %+v", e)
				}
				fields := map[string]string{}
				for _, f := range e.Fields {
					fields[f.Name] = f.Value
				}
				if fields["Group"] != "Bots" || fields["Incident ID"] != "incident" || fields["Monitor ID"] != "monitor" || fields["Check interval"] != "1m0s" || fields["Failure threshold"] != "2 consecutive failures" || fields["Recovery threshold"] != "2 consecutive successes" {
					t.Errorf("missing details: %v", fields)
				}
				if strings.Contains(fields["Monitor"], "@everyone") || strings.Contains(fields["Monitor"], "**prod**") {
					t.Error("unescaped monitor label")
				}
				if tc.kind == "heartbeat" {
					if fields["HTTP response"] != "" || fields["Response time"] != "" || fields["Heartbeat grace"] != "30s" || !strings.Contains(strings.ToLower(fields["Check result"]), "heartbeat") {
						t.Errorf("wrong heartbeat fields: %v", fields)
					}
				} else if fields["HTTP response"] != "503 (expected 200)" || fields["Response time"] != "42.5 ms" {
					t.Errorf("wrong HTTP fields: %v", fields)
				}
				if tc.ok && (fields["Incident duration"] != "5m0s" || !strings.Contains(e.Title, "recovered")) {
					t.Errorf("missing recovery details: %v", fields)
				}
				if !tc.ok && fields["Incident duration"] != "" {
					t.Error("open incident has fabricated duration")
				}
				if r.Header.Get("X-Uptivra-Signature") != "" || r.Header.Get("X-Uptivra-Delivery") != "delivery" || r.Header.Get("Content-Type") != "application/json" {
					t.Error("wrong delivery headers")
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer srv.Close()
			if !Send(context.Background(), store.Delivery{ID: "delivery", Payload: payload, Channel: store.Channel{Kind: "discord", URL: srv.URL}}, srv.Client(), "https://uptime.example.com/status") {
				t.Fatal("Discord delivery failed")
			}
		})
	}
}

func TestDiscordLegacyAndBounds(t *testing.T) {
	raw, err := discordPayload([]byte(`{"type":"incident.resolved","monitor_name":"Legacy monitor","incident_id":"old-incident","at":"2026-10-13T12:00:00Z"}`), "https://uptime.example.com/status")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Embeds []discordEmbed `json:"embeds"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	for _, f := range m.Embeds[0].Fields {
		if f.Name == "Incident duration" || f.Name == "Response time" || f.Name == "Monitor type" || f.Name == "Group" {
			t.Errorf("fabricated legacy field: %s", f.Name)
		}
	}
	p := domain.IncidentEvent{Type: "incident.opened", MonitorName: strings.Repeat("😀*", 2000), Group: strings.Repeat("😀", 2000), IncidentID: strings.Repeat("x", 2000), MonitorID: strings.Repeat("x", 2000), Check: &domain.Check{Reason: "https://secret.invalid/token"}}
	input, _ := json.Marshal(p)
	raw, err = discordPayload(input, "https://uptime.example.com/status")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "secret.invalid") {
		t.Fatal("unrecognized reason exposed")
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	e := m.Embeds[0]
	units := func(s string) int { return len(utf16.Encode([]rune(s))) }
	total := units(e.Title) + units(e.Description) + units(e.Footer.Text)
	for _, f := range e.Fields {
		if units(f.Value) > 1024 || units(f.Name) > 256 || f.Value == "" {
			t.Errorf("invalid field length: %s", f.Name)
		}
		total += units(f.Name) + units(f.Value)
	}
	if total > 6000 || len(e.Fields) > 25 {
		t.Fatal("embed exceeds Discord limits")
	}
	if _, err := discordPayload([]byte(`{`), "https://uptime.example.com/status"); err == nil {
		t.Fatal("malformed event accepted")
	}
}

func TestDiscordFailuresRemainRetryable(t *testing.T) {
	for _, code := range []int{400, 401, 404, 429, 500} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) }))
		d := store.Delivery{Payload: json.RawMessage(`{"type":"incident.opened","monitor_name":"API"}`), Channel: store.Channel{Kind: "discord", URL: srv.URL}}
		if Send(context.Background(), d, srv.Client(), "https://uptime.example.com/status") {
			t.Errorf("status %d marked delivered", code)
		}
		srv.Close()
	}
}

func TestDiscordIncidentUpdate(t *testing.T) {
	for _, resolved := range []bool{false, true} {
		at := time.Now().UTC()
		p := domain.IncidentEvent{Type: "incident.updated", MonitorName: "Bot", At: at, Update: &domain.IncidentUpdate{Status: "monitoring", Message: strings.Repeat("*", 2000) + " @everyone"}}
		if resolved {
			p.ResolvedAt = &at
		}
		raw, _ := json.Marshal(p)
		out, err := discordPayload(raw, "https://status.example.com/status")
		if err != nil {
			t.Fatal(err)
		}
		var result struct {
			Embeds []discordEmbed `json:"embeds"`
		}
		if err = json.Unmarshal(out, &result); err != nil {
			t.Fatal(err)
		}
		e := result.Embeds[0]
		if e.Title != "🔵 Incident update" || len([]rune(e.Description)) > 4096 || !strings.Contains(e.Description, "[View status page") {
			t.Fatal(e)
		}
		expected := "Ongoing"
		if resolved {
			expected = "Resolved"
		}
		found := false
		for _, f := range e.Fields {
			if f.Name == "Status" {
				found = f.Value == expected
			}
		}
		if !found {
			t.Fatal("incorrect update status", e)
		}
	}
}
