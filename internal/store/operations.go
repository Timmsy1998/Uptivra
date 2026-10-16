package store

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"time"
	"uptivra/internal/cryptobox"
	"uptivra/internal/domain"
)

type DeliveryStatus struct {
	ID            string     `json:"id"`
	ChannelID     string     `json:"channel_id"`
	ChannelName   string     `json:"channel_name"`
	State         string     `json:"state"`
	Event         string     `json:"event"`
	Attempts      int        `json:"attempts"`
	CreatedAt     *time.Time `json:"created_at"`
	LastAttemptAt *time.Time `json:"last_attempt_at"`
	NextAttempt   time.Time  `json:"next_attempt"`
	DeliveredAt   *time.Time `json:"delivered_at"`
	FailedAt      *time.Time `json:"failed_at"`
	Outcome       string     `json:"outcome"`
}

func (s *Store) Deliveries(ctx context.Context, state string) ([]DeliveryStatus, error) {
	rows, e := s.DB.Query(ctx, `SELECT d.id,c.id,c.name,CASE WHEN delivered_at IS NOT NULL THEN 'delivered' WHEN failed_at IS NOT NULL THEN 'failed' ELSE 'pending' END,COALESCE(payload->>'type',''),attempts,d.created_at,last_attempt_at,next_attempt,delivered_at,failed_at,outcome FROM deliveries d JOIN channels c ON c.id=d.channel_id WHERE $1='' OR ($1='pending' AND delivered_at IS NULL AND failed_at IS NULL) OR ($1='delivered' AND delivered_at IS NOT NULL) OR ($1='failed' AND failed_at IS NOT NULL) ORDER BY d.created_at DESC NULLS LAST,d.id LIMIT 100`, state)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []DeliveryStatus{}
	for rows.Next() {
		var d DeliveryStatus
		if e = rows.Scan(&d.ID, &d.ChannelID, &d.ChannelName, &d.State, &d.Event, &d.Attempts, &d.CreatedAt, &d.LastAttemptAt, &d.NextAttempt, &d.DeliveredAt, &d.FailedAt, &d.Outcome); e != nil {
			return nil, e
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
func (s *Store) RetryDelivery(ctx context.Context, id string) error {
	// Clearing the fencing token prevents a late completion from the old attempt.
	tag, e := s.DB.Exec(ctx, `UPDATE deliveries SET attempts=0,failed_at=NULL,next_attempt=now(),lease_token=NULL,lease_until=NULL WHERE id=$1 AND failed_at IS NOT NULL AND delivered_at IS NULL`, id)
	if e == nil && tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return e
}
func (s *Store) TestChannel(ctx context.Context, id string) (string, error) {
	token := cryptobox.Token()
	now := time.Now().UTC()
	payload, e := json.Marshal(domain.IncidentEvent{ID: token, Type: "notification.test", MonitorName: "Channel delivery test", At: now})
	if e != nil {
		return "", e
	}
	tag, e := s.DB.Exec(ctx, `INSERT INTO deliveries(id,channel_id,payload) SELECT $1,id,$3 FROM channels WHERE id=$2`, token, id, payload)
	if e == nil && tag.RowsAffected() == 0 {
		return "", pgx.ErrNoRows
	}
	return token, e
}

type SchedulerRun struct {
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
	Success    *bool      `json:"success"`
	Claimed    int        `json:"claimed"`
	Recorded   int64      `json:"recorded"`
	Delivered  int64      `json:"delivered"`
}
type SchedulerHealth struct {
	LastRun       *SchedulerRun `json:"last_run"`
	Overdue       int           `json:"overdue_monitors"`
	Pending       int           `json:"pending_deliveries"`
	Failed        int           `json:"failed_deliveries"`
	OldestPending *time.Time    `json:"oldest_pending_at"`
	ObservedAt    time.Time     `json:"observed_at"`
}

func (s *Store) StartSchedulerRun(ctx context.Context) (string, error) {
	id := cryptobox.Token()
	_, e := s.DB.Exec(ctx, `INSERT INTO scheduler_runs(id) VALUES($1)`, id)
	return id, e
}
func (s *Store) FinishSchedulerRun(ctx context.Context, id string, ok bool, claimed int, recorded, delivered int64) error {
	_, e := s.DB.Exec(ctx, `UPDATE scheduler_runs SET finished_at=now(),success=$2,claimed=$3,recorded=$4,delivered=$5 WHERE id=$1`, id, ok, claimed, recorded, delivered)
	return e
}
func (s *Store) SchedulerHealth(ctx context.Context) (SchedulerHealth, error) {
	var h SchedulerHealth
	var run SchedulerRun
	e := s.DB.QueryRow(ctx, `SELECT started_at,finished_at,success,claimed,recorded,delivered FROM scheduler_runs ORDER BY started_at DESC LIMIT 1`).Scan(&run.StartedAt, &run.FinishedAt, &run.Success, &run.Claimed, &run.Recorded, &run.Delivered)
	if e != nil && e != pgx.ErrNoRows {
		return h, e
	}
	if e == nil {
		h.LastRun = &run
	}
	e = s.DB.QueryRow(ctx, `SELECT count(*) FROM monitors WHERE enabled AND next_check<now()-interval '90 seconds' AND (maintenance_until IS NULL OR maintenance_until<=now()) AND active_maintenance_end(id,now()) IS NULL`).Scan(&h.Overdue)
	if e != nil {
		return h, e
	}
	e = s.DB.QueryRow(ctx, `SELECT count(*) FILTER(WHERE delivered_at IS NULL AND failed_at IS NULL),count(*) FILTER(WHERE failed_at IS NOT NULL),min(created_at) FILTER(WHERE delivered_at IS NULL AND failed_at IS NULL),now() FROM deliveries`).Scan(&h.Pending, &h.Failed, &h.OldestPending, &h.ObservedAt)
	return h, e
}
