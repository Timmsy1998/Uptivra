package domain

import (
	"errors"
	"strings"
	"time"
)

type Maintenance struct {
	ID          string    `json:"id"`
	MonitorID   string    `json:"monitor_id"`
	MonitorName string    `json:"monitor_name"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at"`
	Weekly      bool      `json:"weekly"`
}

func (m Maintenance) Validate(now time.Time) error {
	d := m.EndsAt.Sub(m.StartsAt)
	if m.MonitorID == "" || strings.TrimSpace(m.Title) == "" || len(m.Title) > 100 || len(m.Description) > 500 || !m.StartsAt.After(now) || m.StartsAt.After(now.AddDate(1, 0, 0)) || d <= 0 || d > 30*24*time.Hour || (m.Weekly && d >= 7*24*time.Hour) {
		return errors.New("maintenance requires a monitor, title (1–100 bytes), description (up to 500 bytes), future start within one year and duration up to 30 days (less than seven days if weekly)")
	}
	return nil
}

// Occurrence returns the active or next occurrence; weekly schedules repeat in UTC.
func (m Maintenance) Occurrence(now time.Time) (Maintenance, bool) {
	if !now.Before(m.EndsAt) {
		if !m.Weekly {
			return m, false
		}
		n := now.Sub(m.StartsAt) / (7 * 24 * time.Hour)
		m.StartsAt = m.StartsAt.Add(n * 7 * 24 * time.Hour)
		m.EndsAt = m.EndsAt.Add(n * 7 * 24 * time.Hour)
		if !now.Before(m.EndsAt) {
			m.StartsAt = m.StartsAt.Add(7 * 24 * time.Hour)
			m.EndsAt = m.EndsAt.Add(7 * 24 * time.Hour)
		}
	}
	return m, true
}
