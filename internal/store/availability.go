package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"uptivra/internal/domain"
)

func (s *Store) Availability(ctx context.Context, id string, days int, public bool) (domain.Availability, error) {
	var a domain.Availability
	if !domain.ValidHistoryDays(days) {
		return a, errors.New("history days must be 7, 30 or 90")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	// Hold a shared monitor lock so making a monitor private cannot race projection.
	tx, e := s.DB.BeginTx(ctx, pgx.TxOptions{})
	if e != nil {
		return a, e
	}
	defer tx.Rollback(ctx)
	var exists string
	if e = tx.QueryRow(ctx, `SELECT id FROM monitors WHERE id=$1 AND (NOT $2 OR public) FOR SHARE`, id, public).Scan(&exists); e != nil {
		return a, e
	}
	var retention int
	if e = tx.QueryRow(ctx, `SELECT retention_days FROM settings WHERE id=1`).Scan(&retention); e != nil {
		return a, e
	}
	a = domain.NewAvailability(id, days, retention, time.Now())
	rows, e := tx.Query(ctx, `SELECT checked_at,ok,interval_seconds FROM checks WHERE monitor_id=$1 AND checked_at>=$2 AND checked_at<=$3 ORDER BY checked_at LIMIT 150001`, id, a.From.Add(-24*time.Hour), a.To)
	if e != nil {
		return a, e
	}
	var previous time.Time
	var previousOK bool
	var cadence *int
	add := func(next time.Time) {
		if cadence == nil {
			return
		}
		end := previous.Add(time.Duration(*cadence) * time.Second)
		if next.Before(end) {
			end = next
		}
		a.AddCoverage(previous, end, previousOK)
	}
	count := 0
	for rows.Next() {
		var at time.Time
		var ok bool
		var interval *int
		if e = rows.Scan(&at, &ok, &interval); e != nil {
			rows.Close()
			return a, e
		}
		count++
		if count > 150000 {
			rows.Close()
			return a, errors.New("history sample limit exceeded")
		}
		add(at)
		previous, previousOK, cadence = at, ok, interval
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return a, e
	}
	add(a.To)
	a.Finish()
	rows, e = tx.Query(ctx, `SELECT id,opened_at,resolved_at FROM incidents WHERE monitor_id=$1 AND opened_at<$3 AND (resolved_at IS NULL OR resolved_at>$2) ORDER BY opened_at DESC LIMIT 100`, id, a.From, a.To)
	if e != nil {
		return a, e
	}
	for rows.Next() {
		var incident domain.IncidentDuration
		if e = rows.Scan(&incident.ID, &incident.OpenedAt, &incident.ResolvedAt); e != nil {
			rows.Close()
			return a, e
		}
		end := a.To
		if incident.ResolvedAt != nil {
			end = *incident.ResolvedAt
		}
		incident.DurationSeconds = max(0, end.Sub(incident.OpenedAt).Seconds())
		a.Incidents = append(a.Incidents, incident)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return a, e
	}
	return a, tx.Commit(ctx)
}
