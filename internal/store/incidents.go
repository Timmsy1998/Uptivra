package store

import (
	"context"
	"encoding/json"
	"errors"

	"uptivra/internal/cryptobox"
	"uptivra/internal/domain"
)

var ErrIncidentUpdateLimit = errors.New("incident has reached its limit of 100 updates")

// Append and enqueue atomically. Locking the incident serializes concurrent
// appends so the bounded public timeline cannot exceed its limit.
func (s *Store) AddIncidentUpdate(ctx context.Context, id string, input domain.IncidentUpdateInput) (domain.IncidentUpdate, error) {
	var u domain.IncidentUpdate
	if err := input.Validate(); err != nil {
		return u, err
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return u, err
	}
	defer tx.Rollback(ctx)
	event := domain.IncidentEvent{ID: cryptobox.Token(), Type: "incident.updated", IncidentID: id}
	if err = tx.QueryRow(ctx, `SELECT i.monitor_id,i.opened_at,i.resolved_at,m.config->>'name',COALESCE(m.config->>'group','') FROM incidents i JOIN monitors m ON m.id=i.monitor_id WHERE i.id=$1 FOR UPDATE OF i`, id).Scan(&event.MonitorID, &event.OpenedAt, &event.ResolvedAt, &event.MonitorName, &event.Group); err != nil {
		return u, err
	}
	var count int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM incident_updates WHERE incident_id=$1`, id).Scan(&count); err != nil {
		return u, err
	}
	if count >= 100 {
		return u, ErrIncidentUpdateLimit
	}
	u.ID, u.Status, u.Message = cryptobox.Token(), input.Status, input.Message
	if err = tx.QueryRow(ctx, `INSERT INTO incident_updates(id,incident_id,status,message) VALUES($1,$2,$3,$4) RETURNING created_at`, u.ID, id, u.Status, u.Message).Scan(&u.CreatedAt); err != nil {
		return u, err
	}
	event.At, event.Update = u.CreatedAt, &u
	payload, err := json.Marshal(event)
	if err != nil {
		return u, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO deliveries(id,channel_id,payload) SELECT $1||':'||id,id,$2 FROM channels WHERE monitor_group IS NULL OR monitor_group=$3`, event.ID, payload, event.Group); err != nil {
		return u, err
	}
	return u, tx.Commit(ctx)
}
