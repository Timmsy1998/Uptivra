package domain

import (
	"strings"
	"testing"
)

func TestIncidentUpdateValidation(t *testing.T) {
	for _, tc := range []struct {
		status, message string
		valid           bool
	}{
		{"update", "  Fix deployed\nWatching recovery  ", true}, {"identified", strings.Repeat("a", 2000), true},
		{"monitoring", strings.Repeat("é", 1001), false}, {"resolved", "done", false}, {"investigating", " \n ", false}, {"update", "bad\x00text", false},
	} {
		u := IncidentUpdateInput{Status: tc.status, Message: tc.message}
		if err := u.Validate(); (err == nil) != tc.valid {
			t.Errorf("validation: %v", err)
		}
		if tc.valid && u.Message != strings.TrimSpace(tc.message) {
			t.Fatal("not trimmed")
		}
	}
}
