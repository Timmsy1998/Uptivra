package domain

import "time"

// IncidentEvent is a safe notification snapshot. Never include targets, headers,
// response bodies, or credentials. Optional details preserve older queued events.
type IncidentEvent struct {
	ID                string    `json:"id"`
	Type              string    `json:"type"`
	IncidentID        string    `json:"incident_id"`
	MonitorID         string    `json:"monitor_id"`
	MonitorName       string    `json:"monitor_name"`
	At                time.Time `json:"at"`
	OpenedAt          time.Time `json:"opened_at,omitempty"`
	Group             string    `json:"monitor_group,omitempty"`
	MonitorType       string    `json:"monitor_type,omitempty"`
	Interval          int       `json:"interval_seconds,omitempty"`
	Grace             int       `json:"grace_seconds,omitempty"`
	FailureThreshold  int       `json:"failure_threshold,omitempty"`
	RecoveryThreshold int       `json:"recovery_threshold,omitempty"`
	ExpectedStatus    int       `json:"expected_status,omitempty"`
	Check             *Check    `json:"check,omitempty"`
}
