package domain

import (
	"testing"
	"time"
)

func TestTransitions(t *testing.T) {
	s, f, r, e := Transition("healthy", 0, 0, false, 2, 2)
	if s != "degraded" || e != "" {
		t.Fatal(s, e)
	}
	s, f, r, e = Transition(s, f, r, false, 2, 2)
	if s != "down" || e != "opened" {
		t.Fatal(s, e)
	}
	s, f, r, e = Transition(s, f, r, true, 2, 2)
	if s != "down" || e != "" {
		t.Fatal(s, e)
	}
	s, _, _, e = Transition(s, f, r, true, 2, 2)
	if s != "healthy" || e != "resolved" {
		t.Fatal(s, e)
	}
}

func TestHeartbeatValidation(t *testing.T) {
	m := Monitor{Type: "heartbeat", Name: "Bot", Interval: 60, Grace: 120, FailureThreshold: 1, RecoveryThreshold: 1, Authorized: true}
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Monitor){
		func(m *Monitor) { m.URL = "https://example.com" },
		func(m *Monitor) { m.Headers = map[string]string{"Authorization": "secret"} },
		func(m *Monitor) { m.Grace = -1 },
		func(m *Monitor) { m.Grace = 86401 },
		func(m *Monitor) { m.Type = "unknown" },
		func(m *Monitor) { m.Interval = 59 },
	} {
		bad := m
		mutate(&bad)
		if bad.Validate() == nil {
			t.Fatal("invalid heartbeat accepted", bad)
		}
	}
}

func TestHeartbeatFreshness(t *testing.T) {
	m := Monitor{Interval: 60, Grace: 120}
	now := time.Now()
	if m.HeartbeatFresh(nil, now) {
		t.Fatal("missing receipt healthy")
	}
	for _, test := range []struct {
		age   time.Duration
		fresh bool
	}{{0, true}, {180 * time.Second, true}, {180*time.Second + time.Nanosecond, false}, {-time.Second, false}} {
		last := now.Add(-test.age)
		if m.HeartbeatFresh(&last, now) != test.fresh {
			t.Fatal(test)
		}
	}
}
