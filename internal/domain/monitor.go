package domain

import (
	"errors"
	"github.com/timmsy1998/uptivra/internal/netguard"
	"net/http"
	"strings"
	"time"
)

type Monitor struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	Description       string            `json:"description"`
	Group             string            `json:"group"`
	URL               string            `json:"url"`
	Interval          int               `json:"interval_seconds"`
	Timeout           int               `json:"timeout_seconds"`
	ExpectedStatus    int               `json:"expected_status"`
	Contains          string            `json:"contains"`
	Headers           map[string]string `json:"headers,omitempty"`
	FailureThreshold  int               `json:"failure_threshold"`
	RecoveryThreshold int               `json:"recovery_threshold"`
	Enabled           bool              `json:"enabled"`
	Public            bool              `json:"public"`
	Authorized        bool              `json:"authorized"`
	MaintenanceUntil  *time.Time        `json:"maintenance_until"`
	Status            string            `json:"status"`
	LastCheck         *time.Time        `json:"last_check"`
	Failures          int               `json:"-"`
	Successes         int               `json:"-"`
	Lease             string            `json:"-"`
	Uptime            *float64          `json:"uptime"`
	Latency           *float64          `json:"latency_ms"`
	P95               *float64          `json:"p95_ms"`
	Samples           int               `json:"samples"`
}

func (m Monitor) Validate() error {
	if strings.TrimSpace(m.Name) == "" || len(m.Name) > 100 || len(m.Description) > 500 || len(m.Group) > 80 {
		return errors.New("name (1–100), description (up to 500) and group (up to 80 characters) required")
	}
	if _, e := netguard.URL(m.URL); e != nil {
		return e
	}
	if !m.Authorized {
		return errors.New("confirm you are authorized to monitor this endpoint")
	}
	if m.Interval < 60 || m.Interval > 86400 || m.Timeout < 1 || m.Timeout > 10 || m.ExpectedStatus < 100 || m.ExpectedStatus > 599 || m.FailureThreshold < 1 || m.FailureThreshold > 10 || m.RecoveryThreshold < 1 || m.RecoveryThreshold > 10 || len(m.Contains) > 1024 {
		return errors.New("invalid interval, timeout, status, content or thresholds")
	}
	if len(m.Headers) > 16 {
		return errors.New("at most 16 request headers")
	}
	for k, v := range m.Headers {
		if len(k) == 0 || len(k) > 80 || len(v) > 2048 || strings.ContainsAny(k+v, "\r\n") {
			return errors.New("invalid request header")
		}
		for _, c := range k {
			if !strings.ContainsRune("!#$%&'*+-.^_`|~0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ", c) {
				return errors.New("invalid header name")
			}
		}
		switch strings.ToLower(k) {
		case "host", "connection", "content-length", "transfer-encoding", "proxy-authorization", "cookie", "upgrade":
			return errors.New("restricted request header")
		}
		if http.CanonicalHeaderKey(k) == "" {
			return errors.New("invalid header")
		}
	}
	return nil
}
func (m Monitor) Health(now time.Time) string {
	if !m.Enabled {
		return "paused"
	}
	if m.MaintenanceUntil != nil && m.MaintenanceUntil.After(now) {
		return "maintenance"
	}
	if m.LastCheck == nil || now.Sub(*m.LastCheck) > time.Duration(m.Interval*2+30)*time.Second {
		return "unknown"
	}
	return m.Status
}

type Check struct {
	At      time.Time `json:"checked_at"`
	OK      bool      `json:"ok"`
	Latency float64   `json:"latency_ms"`
	Code    int       `json:"status_code"`
	Reason  string    `json:"reason"`
}

// Transition preserves an open incident through partial recovery.
func Transition(status string, failures, successes int, ok bool, failureThreshold, recoveryThreshold int) (string, int, int, string) {
	if ok {
		successes++
		failures = 0
		if status == "down" {
			if successes >= recoveryThreshold {
				return "healthy", 0, successes, "resolved"
			}
			return "down", 0, successes, ""
		}
		return "healthy", 0, successes, ""
	}
	failures++
	successes = 0
	if status == "down" {
		return "down", failures, 0, ""
	}
	if failures >= failureThreshold {
		return "down", failures, 0, "opened"
	}
	return "degraded", failures, 0, ""
}

type Incident struct {
	ID         string     `json:"id"`
	MonitorID  string     `json:"monitor_id"`
	Name       string     `json:"name"`
	OpenedAt   time.Time  `json:"opened_at"`
	ResolvedAt *time.Time `json:"resolved_at"`
}
type Settings struct {
	Title         string `json:"title"`
	Description   string `json:"description"`
	RetentionDays int    `json:"retention_days"`
}
