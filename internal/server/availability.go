package server

import (
	"net/http"
	"strconv"
	"uptivra/internal/domain"
)

func (s *Server) availability(w http.ResponseWriter, r *http.Request) {
	s.sendAvailability(w, r, false)
}
func (s *Server) publicAvailability(w http.ResponseWriter, r *http.Request) {
	s.sendAvailability(w, r, true)
}
func (s *Server) sendAvailability(w http.ResponseWriter, r *http.Request, public bool) {
	days := 30
	if raw := r.URL.Query().Get("days"); raw != "" {
		days, _ = strconv.Atoi(raw)
	}
	if !domain.ValidHistoryDays(days) {
		problem(w, 400, "days must be 7, 30 or 90")
		return
	}
	a, e := s.Store.Availability(r.Context(), r.PathValue("id"), days, public)
	if !dbError(w, e) {
		respond(w, 200, a)
	}
}
