package domain

import "testing"

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
