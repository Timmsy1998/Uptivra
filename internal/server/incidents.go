package server

import (
	"errors"
	"net/http"

	"uptivra/internal/domain"
	"uptivra/internal/store"
)

func (s *Server) addIncidentUpdate(w http.ResponseWriter, r *http.Request) {
	var input domain.IncidentUpdateInput
	if !decode(w, r, &input) {
		return
	}
	if err := input.Validate(); err != nil {
		problem(w, 400, err.Error())
		return
	}
	u, err := s.Store.AddIncidentUpdate(r.Context(), r.PathValue("id"), input)
	if errors.Is(err, store.ErrIncidentUpdateLimit) {
		problem(w, 409, err.Error())
		return
	}
	if dbError(w, err) {
		return
	}
	respond(w, 201, u)
}
