package server

import (
	"net/http"
)

func (s *Server) deliveries(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	if state != "" && state != "pending" && state != "delivered" && state != "failed" {
		problem(w, 400, "state must be pending, delivered or failed")
		return
	}
	v, e := s.Store.Deliveries(r.Context(), state)
	if !dbError(w, e) {
		respond(w, 200, v)
	}
}
func (s *Server) retryDelivery(w http.ResponseWriter, r *http.Request) {
	if !s.Store.RateLimit(r.Context(), "delivery:retry", 60) {
		problem(w, 429, "too many retry attempts")
		return
	}
	if !dbError(w, s.Store.RetryDelivery(r.Context(), r.PathValue("id"))) {
		respond(w, 204, nil)
	}
}
func (s *Server) testChannel(w http.ResponseWriter, r *http.Request) {
	if !s.Store.RateLimit(r.Context(), "channel:test", 20) {
		problem(w, 429, "too many channel tests")
		return
	}
	id, e := s.Store.TestChannel(r.Context(), r.PathValue("id"))
	if !dbError(w, e) {
		respond(w, 201, map[string]string{"id": id})
	}
}
func (s *Server) schedulerHealth(w http.ResponseWriter, r *http.Request) {
	v, e := s.Store.SchedulerHealth(r.Context())
	if !dbError(w, e) {
		respond(w, 200, v)
	}
}
