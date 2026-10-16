package server

import (
	"net/http"
	"time"
	"uptivra/internal/domain"
)

func (s *Server) maintenance(w http.ResponseWriter, r *http.Request) {
	v, e := s.Store.Maintenance(r.Context(), false)
	if !dbError(w, e) {
		respond(w, 200, v)
	}
}
func (s *Server) createMaintenance(w http.ResponseWriter, r *http.Request) {
	var m domain.Maintenance
	if !decode(w, r, &m) {
		return
	}
	if e := m.Validate(time.Now()); e != nil {
		problem(w, 400, e.Error())
		return
	}
	id, e := s.Store.CreateMaintenance(r.Context(), m)
	if !dbError(w, e) {
		respond(w, 201, map[string]string{"id": id})
	}
}
func (s *Server) deleteMaintenance(w http.ResponseWriter, r *http.Request) {
	if !dbError(w, s.Store.DeleteMaintenance(r.Context(), r.PathValue("id"))) {
		respond(w, 204, nil)
	}
}
