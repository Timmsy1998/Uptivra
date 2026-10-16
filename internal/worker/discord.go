package worker

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"uptivra/internal/domain"
)

type discordField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}
type discordEmbed struct {
	URL         string         `json:"url,omitempty"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Color       int            `json:"color"`
	Timestamp   string         `json:"timestamp,omitempty"`
	Fields      []discordField `json:"fields"`
	Footer      struct {
		Text string `json:"text"`
	} `json:"footer"`
}

func discordPayload(raw []byte, statusPageURL string) ([]byte, error) {
	var p domain.IncidentEvent
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	e := discordEmbed{Title: "🔴 Incident opened", Description: "The monitor has crossed its failure threshold.", Color: 0xad3730}
	status := "Down"
	if p.Type == "notification.test" {
		e.Title, e.Description, e.Color = "🔵 Channel delivery test", "This test notification confirms that Uptivra can deliver to this channel.", 0x4f6389
		status = "Test"
	} else if p.Type == "incident.resolved" {
		e.Title, e.Description, e.Color = "🟢 Service recovered", "The monitor has crossed its recovery threshold. The incident is resolved.", 0x286b4c
		status = "Healthy"
	} else if p.Type == "incident.updated" && p.Update != nil {
		e.Title, e.Description, e.Color = "🔵 Incident update", discordTextLimit(p.Update.Message, 2800), 0x4f6389
		status = "Ongoing"
		if p.ResolvedAt != nil {
			status = "Resolved"
		}
		e.Fields = append(e.Fields, discordField{"Update stage", discordText(p.Update.Status), true})
	} else if p.Type != "incident.opened" {
		e.Title, e.Description, e.Color = "Monitor event", "An uptime monitoring event was recorded.", 0x4f6389
		status = "Unknown"
	}
	if statusPageURL != "" {
		e.URL = statusPageURL
		e.Description += "\n\n[View status page — service status and incident updates](" + statusPageURL + ")"
	}
	add := func(name, value string, inline bool) { e.Fields = append(e.Fields, discordField{name, value, inline}) }
	name := discordText(p.MonitorName)
	if name == "" {
		name = "Unnamed monitor"
	}
	add("Monitor", name, false)
	add("Status", status, true)
	// Missing group in legacy queued payloads cannot be distinguished from ungrouped.
	if p.Check != nil {
		group := discordText(p.Group)
		if group == "" {
			group = "Ungrouped"
		}
		add("Group", group, true)
		kind := "HTTP"
		if p.MonitorType == "heartbeat" {
			kind = "Heartbeat"
		}
		add("Monitor type", kind, true)
		add("Check result", checkReason(p.Check), false)
		if p.MonitorType != "heartbeat" {
			if p.Check.Code != 0 {
				add("HTTP response", fmt.Sprintf("%d (expected %d)", p.Check.Code, p.ExpectedStatus), true)
			}
			add("Response time", fmt.Sprintf("%.1f ms", p.Check.Latency), true)
		}
	}
	if p.Interval > 0 {
		add("Check interval", (time.Duration(p.Interval) * time.Second).String(), true)
	}
	if p.MonitorType == "heartbeat" {
		add("Heartbeat grace", (time.Duration(p.Grace) * time.Second).String(), true)
	}
	if p.FailureThreshold > 0 {
		add("Failure threshold", fmt.Sprintf("%d consecutive failures", p.FailureThreshold), true)
	}
	if p.RecoveryThreshold > 0 {
		add("Recovery threshold", fmt.Sprintf("%d consecutive successes", p.RecoveryThreshold), true)
	}
	if !p.OpenedAt.IsZero() {
		add("Incident opened", discordTime(p.OpenedAt), true)
		if p.Type == "incident.resolved" && !p.At.IsZero() && !p.At.Before(p.OpenedAt) {
			add("Incident duration", p.At.Sub(p.OpenedAt).Round(time.Second).String(), true)
		}
	}
	if !p.At.IsZero() {
		label := "Detected at"
		if p.Type == "incident.updated" {
			label = "Updated at"
		}
		if p.Type == "incident.resolved" {
			label = "Recovered at"
		}
		add(label, discordTime(p.At), true)
		e.Timestamp = p.At.UTC().Format(time.RFC3339)
	}
	if p.IncidentID != "" {
		add("Incident ID", discordText(p.IncidentID), false)
	}
	if p.MonitorID != "" {
		add("Monitor ID", discordText(p.MonitorID), false)
	}
	e.Footer.Text = "Uptivra • Uptime monitoring"
	return json.Marshal(struct {
		Username        string         `json:"username"`
		Embeds          []discordEmbed `json:"embeds"`
		AllowedMentions struct {
			Parse []string `json:"parse"`
		} `json:"allowed_mentions"`
	}{Username: "Uptivra", Embeds: []discordEmbed{e}, AllowedMentions: struct {
		Parse []string `json:"parse"`
	}{Parse: []string{}}})
}

func discordTime(t time.Time) string { return fmt.Sprintf("<t:%d:F> (<t:%d:R>)", t.Unix(), t.Unix()) }

// Keep user labels literal and bounded even for older or externally queued data.
// Each dynamic field is limited to 300 runes, keeping the embed below 6000.
func discordText(s string) string { return discordTextLimit(s, 300) }

func discordTextLimit(s string, limit int) string {
	s = strings.TrimSpace(s)
	s = strings.NewReplacer("\\", "\\\\", "*", "\\*", "_", "\\_", "~", "\\~", "`", "\\`", "|", "\\|", "[", "\\[", "]", "\\]", "<", "‹", ">", "›", "@", "@\u200b").Replace(s)
	r := []rune(s)
	if len(r) > limit {
		s = string(r[:limit-1]) + "…"
	}
	return s
}

func checkReason(c *domain.Check) string {
	if c.OK {
		if c.Reason == "heartbeat_received" {
			return "Heartbeat received within the allowed interval."
		}
		return "HTTP check passed."
	}
	switch c.Reason {
	case "heartbeat_missed":
		return "No heartbeat received within the interval and grace period."
	case "timeout":
		return "Request timed out."
	case "network_error":
		return "Network connection failed."
	case "unexpected_status":
		return "Unexpected HTTP response status."
	case "content_mismatch":
		return "Required response content was missing."
	case "body_error":
		return "Response body could not be read."
	case "body_limit":
		return "Response exceeded the permitted size."
	default:
		return "Check failed."
	}
}
