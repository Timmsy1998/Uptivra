package store

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"sort"
	"time"
	"uptivra/internal/cryptobox"
	"uptivra/internal/domain"
)

func (s *Store) Maintenance(ctx context.Context, public bool) ([]domain.Maintenance, error) {
	rows, e := s.DB.Query(ctx, `SELECT w.id,w.monitor_id,COALESCE(m.config->>'name',''),w.title,w.description,w.starts_at,w.ends_at,w.weekly FROM maintenance_windows w JOIN monitors m ON m.id=w.monitor_id WHERE NOT $1 OR m.public ORDER BY w.starts_at LIMIT 1000`, public)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []domain.Maintenance{}
	now := time.Now().UTC()
	for rows.Next() {
		var m domain.Maintenance
		if e = rows.Scan(&m.ID, &m.MonitorID, &m.MonitorName, &m.Title, &m.Description, &m.StartsAt, &m.EndsAt, &m.Weekly); e != nil {
			return nil, e
		}
		if public {
			var ok bool
			m, ok = m.Occurrence(now)
			if !ok || m.StartsAt.After(now.AddDate(0, 0, 30)) {
				continue
			}
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartsAt.Before(out[j].StartsAt) })
	return out, rows.Err()
}
func (s *Store) CreateMaintenance(ctx context.Context, m domain.Maintenance) (string, error) {
	if e := m.Validate(time.Now()); e != nil {
		return "", e
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return "", e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(884129)`); e != nil {
		return "", e
	}
	var count int
	if e = tx.QueryRow(ctx, `SELECT count(*) FROM maintenance_windows`).Scan(&count); e != nil {
		return "", e
	}
	if count >= 1000 {
		return "", errors.New("maintenance window limit reached")
	}
	var id string
	if e = tx.QueryRow(ctx, `SELECT id FROM monitors WHERE id=$1 FOR UPDATE`, m.MonitorID).Scan(&id); e != nil {
		return "", e
	}
	m.ID = cryptobox.Token()
	if _, e = tx.Exec(ctx, `INSERT INTO maintenance_windows(id,monitor_id,title,description,starts_at,ends_at,weekly) VALUES($1,$2,$3,$4,$5,$6,$7)`, m.ID, m.MonitorID, m.Title, m.Description, m.StartsAt.UTC(), m.EndsAt.UTC(), m.Weekly); e != nil {
		return "", e
	}
	// Fence any in-flight check; Record also rechecks windows that start after claiming.
	if _, e = tx.Exec(ctx, `UPDATE monitors SET lease_token=NULL,lease_until=NULL WHERE id=$1`, m.MonitorID); e != nil {
		return "", e
	}
	return m.ID, tx.Commit(ctx)
}
func (s *Store) DeleteMaintenance(ctx context.Context, id string) error {
	tag, e := s.DB.Exec(ctx, `DELETE FROM maintenance_windows WHERE id=$1`, id)
	if e == nil && tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return e
}
