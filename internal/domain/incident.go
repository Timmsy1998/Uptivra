package domain

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// IncidentUpdate is intentionally public when its monitor is public. Updates do
// not change monitor health or resolve incidents; checks remain authoritative.
type IncidentUpdate struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

type IncidentUpdateInput struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func (u *IncidentUpdateInput) Validate() error {
	u.Message = strings.TrimSpace(u.Message)
	switch u.Status {
	case "update", "investigating", "identified", "monitoring":
	default:
		return errors.New("choose update, investigating, identified or monitoring")
	}
	if !utf8.ValidString(u.Message) || len(u.Message) == 0 || len(u.Message) > 2000 || strings.ContainsRune(u.Message, 0) {
		return errors.New("incident message must be 1–2000 bytes of text")
	}
	return nil
}
