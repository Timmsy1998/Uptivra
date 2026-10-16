package server

import (
	"fmt"
	"net/http"
	"time"

	"uptivra/internal/domain"
)

type PublicMonitor struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Group     string     `json:"group"`
	Status    string     `json:"status"`
	Uptime    *float64   `json:"uptime"`
	Latency   *float64   `json:"latency_ms"`
	LastCheck *time.Time `json:"last_check"`
	Samples   int        `json:"samples"`
}
type PublicStatus struct {
	Maintenance []domain.Maintenance `json:"maintenance"`
	Title       string               `json:"title"`
	Description string               `json:"description"`
	Status      string               `json:"status"`
	UpdatedAt   time.Time            `json:"updated_at"`
	Monitors    []PublicMonitor      `json:"monitors"`
	Incidents   []domain.Incident    `json:"incidents"`
}

func (s *Server) public(w http.ResponseWriter, r *http.Request) {
	settings, e := s.Store.Settings(r.Context())
	if dbError(w, e) {
		return
	}
	ms, e := s.Store.Monitors(r.Context())
	if dbError(w, e) {
		return
	}
	inc, e := s.Store.Incidents(r.Context(), true)
	if dbError(w, e) {
		return
	}
	out := PublicStatus{Title: settings.Title, Description: settings.Description, Status: "unknown", UpdatedAt: time.Now().UTC(), Monitors: []PublicMonitor{}, Incidents: inc}
	out.Maintenance, e = s.Store.Maintenance(r.Context(), true)
	if dbError(w, e) {
		return
	}
	rank := map[string]int{"healthy": 0, "paused": 1, "maintenance": 2, "unknown": 3, "degraded": 4, "down": 5}
	worst := -1
	for _, m := range ms {
		if !m.Public {
			continue
		}
		out.Monitors = append(out.Monitors, PublicMonitor{m.ID, m.Name, m.Group, m.Status, m.Uptime, m.Latency, m.LastCheck, m.Samples})
		if rank[m.Status] > worst {
			out.Status = m.Status
			worst = rank[m.Status]
		}
	}
	respond(w, 200, out)
}
func (s *Server) badge(w http.ResponseWriter, r *http.Request) {
	ms, e := s.Store.Monitors(r.Context())
	if dbError(w, e) {
		return
	}
	state := "unknown"
	rank := map[string]int{"healthy": 0, "paused": 1, "maintenance": 2, "unknown": 3, "degraded": 4, "down": 5}
	worst := -1
	for _, m := range ms {
		if m.Public && rank[m.Status] > worst {
			worst = rank[m.Status]
			state = m.Status
		}
	}
	color := map[string]string{"healthy": "#256348", "down": "#b63832", "degraded": "#986517", "maintenance": "#52698b", "paused": "#626962", "unknown": "#626962"}[state]
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
	fmt.Fprintf(w, `<svg xmlns="http://www.w3.org/2000/svg" width="176" height="24" role="img" aria-label="Uptivra: %s"><title>Uptivra: %s</title><rect width="176" height="24" rx="3" fill="#242923"/><path d="M74 0h99q3 0 3 3v18q0 3-3 3H74z" fill="%s"/><g fill="#fff" font-family="Verdana,sans-serif" font-size="11" text-anchor="middle"><text x="37" y="16">uptivra</text><text x="125" y="16">%s</text></g></svg>`, state, state, color, state)
}
