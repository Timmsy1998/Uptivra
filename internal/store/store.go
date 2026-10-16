package store

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"uptivra/internal/cryptobox"
	"uptivra/internal/domain"
)

//go:embed schema.sql
var schema string

//go:embed migrations/002_heartbeats.sql
var heartbeatMigration string

//go:embed migrations/003_channel_groups.sql
var channelGroupsMigration string

//go:embed migrations/004_incident_updates.sql
var incidentUpdatesMigration string

//go:embed migrations/005_admins.sql
var adminsMigration string

//go:embed migrations/006_operations.sql
var operationsMigration string

//go:embed migrations/007_maintenance.sql
var maintenanceMigration string

type Store struct {
	DB  *pgxpool.Pool
	Box *cryptobox.Box
}

func Open(ctx context.Context, url string, box *cryptobox.Box) (*Store, error) {
	c, e := pgxpool.ParseConfig(url)
	if e != nil {
		return nil, errors.New("invalid DATABASE_URL")
	}
	c.MaxConns = 4
	c.MinConns = 0
	c.MaxConnIdleTime = time.Minute
	c.MaxConnLifetime = 5 * time.Minute
	c.ConnConfig.ConnectTimeout = 5 * time.Second
	p, e := pgxpool.NewWithConfig(ctx, c)
	if e != nil {
		return nil, e
	}
	if e = p.Ping(ctx); e != nil {
		p.Close()
		return nil, errors.New("database unavailable")
	}
	return &Store{p, box}, nil
}
func (s *Store) Migrate(ctx context.Context) error {
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(884127)"); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, schema); e != nil {
		return e
	}
	for _, migration := range []struct {
		version int
		sql     string
	}{{2, heartbeatMigration}, {3, channelGroupsMigration}, {4, incidentUpdatesMigration}, {5, adminsMigration}, {6, operationsMigration}, {7, maintenanceMigration}} {
		var applied bool
		if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_versions WHERE version=$1)`, migration.version).Scan(&applied); e != nil {
			return e
		}
		if !applied {
			if _, e = tx.Exec(ctx, migration.sql); e != nil {
				return e
			}
		}
	}
	return tx.Commit(ctx)
}

const monitorColumns = `m.id,m.config,m.secrets,m.enabled,m.public,m.interval_seconds,m.maintenance_until,m.status,m.failures,m.successes,m.last_check,COALESCE(m.lease_token,''),
 (SELECT 100.0*count(*) FILTER(WHERE ok)/NULLIF(count(*),0) FROM checks WHERE monitor_id=m.id AND checked_at>now()-interval '24 hours'),
 (SELECT avg(latency_ms) FROM checks WHERE monitor_id=m.id AND checked_at>now()-interval '24 hours'),
 (SELECT percentile_cont(0.95) WITHIN GROUP(ORDER BY latency_ms) FROM checks WHERE monitor_id=m.id AND checked_at>now()-interval '24 hours'),
 (SELECT count(*) FROM checks WHERE monitor_id=m.id AND checked_at>now()-interval '24 hours'),active_maintenance_end(m.id,now())`

type scanner interface{ Scan(...any) error }

func (s *Store) scan(row scanner) (domain.Monitor, error) {
	var m domain.Monitor
	var config []byte
	var sec string
	var enabled, public bool
	var interval int
	var maintenance, last, scheduled *time.Time
	var status, lease, id string
	var failures, successes, samples int
	var uptime, latency, p95 *float64
	e := row.Scan(&id, &config, &sec, &enabled, &public, &interval, &maintenance, &status, &failures, &successes, &last, &lease, &uptime, &latency, &p95, &samples, &scheduled)
	if e != nil {
		return m, e
	}
	if e = json.Unmarshal(config, &m); e != nil {
		return m, e
	}
	plain, e := s.Box.Open(sec)
	if e != nil {
		return m, e
	}
	if e = json.Unmarshal([]byte(plain), &m.Headers); e != nil {
		return m, e
	}
	m.ID = id
	m.Enabled = enabled
	m.Public = public
	m.Interval = interval
	m.MaintenanceUntil = maintenance
	m.ScheduledMaintenanceUntil = scheduled
	m.Status = status
	m.Failures = failures
	m.Successes = successes
	m.LastCheck = last
	m.Lease = lease
	m.Uptime = uptime
	m.Latency = latency
	m.P95 = p95
	if m.Type == "heartbeat" {
		m.Latency = nil
		m.P95 = nil
	}
	m.Samples = samples
	return m, nil
}
func (s *Store) Monitors(ctx context.Context) ([]domain.Monitor, error) {
	rows, e := s.DB.Query(ctx, "SELECT "+monitorColumns+" FROM monitors m ORDER BY m.created_at,m.id LIMIT 1000")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	ms := []domain.Monitor{}
	for rows.Next() {
		m, e := s.scan(rows)
		if e != nil {
			return nil, e
		}
		m.Status = m.Health(time.Now())
		ms = append(ms, m)
	}
	return ms, rows.Err()
}
func (s *Store) Monitor(ctx context.Context, id string) (domain.Monitor, error) {
	return s.scan(s.DB.QueryRow(ctx, "SELECT "+monitorColumns+" FROM monitors m WHERE m.id=$1", id))
}
func (s *Store) SaveMonitor(ctx context.Context, m domain.Monitor, create bool) error {
	if e := m.Validate(); e != nil {
		return e
	}
	headers, e := json.Marshal(m.Headers)
	if e != nil {
		return e
	}
	m.Headers = nil
	m.ScheduledMaintenanceUntil = nil
	m.Status = ""
	m.LastCheck = nil
	m.Uptime = nil
	m.Latency = nil
	m.P95 = nil
	m.Samples = 0
	config, e := json.Marshal(m)
	if e != nil {
		return e
	}
	secret := s.Box.Seal(string(headers))
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if create {
		_, e = tx.Exec(ctx, `INSERT INTO monitors(id,config,secrets,enabled,public,interval_seconds,maintenance_until) VALUES($1,$2,$3,$4,$5,$6,$7)`, m.ID, config, secret, m.Enabled, m.Public, m.Interval, m.MaintenanceUntil)
	} else {
		var tag pgconn.CommandTag
		tag, e = tx.Exec(ctx, `UPDATE monitors SET config=$2,secrets=$3,enabled=$4,public=$5,interval_seconds=$6,maintenance_until=$7,lease_token=NULL,lease_until=NULL,next_check=now(),failures=0,successes=0 WHERE id=$1 AND COALESCE(NULLIF(config->>'type',''),'http')=COALESCE(NULLIF($2::jsonb->>'type',''),'http')`, m.ID, config, secret, m.Enabled, m.Public, m.Interval, m.MaintenanceUntil)
		if e == nil && tag.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
	}
	if e != nil {
		return e
	}
	if m.Type == "heartbeat" {
		if _, e = tx.Exec(ctx, `INSERT INTO heartbeats(monitor_id) VALUES($1) ON CONFLICT(monitor_id) DO NOTHING`, m.ID); e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, `UPDATE monitors SET next_check=GREATEST(now(),COALESCE(maintenance_until,now()))+($2::integer+$3::integer)*interval '1 second' WHERE id=$1`, m.ID, m.Interval, m.Grace); e != nil {
			return e
		}
	}
	return tx.Commit(ctx)
}
func (s *Store) DeleteMonitor(ctx context.Context, id string) error {
	t, e := s.DB.Exec(ctx, "DELETE FROM monitors WHERE id=$1", id)
	if e == nil && t.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return e
}
func (s *Store) History(ctx context.Context, id string) ([]domain.Check, error) {
	rows, e := s.DB.Query(ctx, `SELECT checked_at,ok,latency_ms,status_code,reason FROM checks WHERE monitor_id=$1 ORDER BY checked_at DESC LIMIT 100`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []domain.Check{}
	for rows.Next() {
		var c domain.Check
		if e = rows.Scan(&c.At, &c.OK, &c.Latency, &c.Code, &c.Reason); e != nil {
			return nil, e
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (s *Store) Incidents(ctx context.Context, public bool) ([]domain.Incident, error) {
	rows, e := s.DB.Query(ctx, `SELECT i.id,i.monitor_id,m.config->>'name',i.opened_at,i.resolved_at,COALESCE((SELECT jsonb_agg(jsonb_build_object('id',u.id,'status',u.status,'message',u.message,'created_at',u.created_at) ORDER BY u.created_at,u.id) FROM incident_updates u WHERE u.incident_id=i.id),'[]'::jsonb) FROM incidents i JOIN monitors m ON m.id=i.monitor_id WHERE (NOT $1 OR m.public) ORDER BY i.opened_at DESC LIMIT 100`, public)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []domain.Incident{}
	for rows.Next() {
		var i domain.Incident
		if e = rows.Scan(&i.ID, &i.MonitorID, &i.Name, &i.OpenedAt, &i.ResolvedAt, &i.Updates); e != nil {
			return nil, e
		}
		out = append(out, i)
	}
	return out, rows.Err()
}
func (s *Store) Settings(ctx context.Context) (domain.Settings, error) {
	var v domain.Settings
	e := s.DB.QueryRow(ctx, `SELECT title,description,retention_days FROM settings WHERE id=1`).Scan(&v.Title, &v.Description, &v.RetentionDays)
	return v, e
}
func (s *Store) SaveSettings(ctx context.Context, v domain.Settings) error {
	if len(v.Title) < 1 || len(v.Title) > 100 || len(v.Description) > 500 || v.RetentionDays < 7 || v.RetentionDays > 365 {
		return errors.New("invalid status page settings or retention (7–365 days)")
	}
	_, e := s.DB.Exec(ctx, `UPDATE settings SET title=$1,description=$2,retention_days=$3 WHERE id=1`, v.Title, v.Description, v.RetentionDays)
	return e
}
func (s *Store) Claim(ctx context.Context) ([]domain.Monitor, error) {
	token := cryptobox.Token()
	rows, e := s.DB.Query(ctx, `WITH due AS (SELECT id FROM monitors WHERE enabled AND next_check<=now() AND (maintenance_until IS NULL OR maintenance_until<=now()) AND active_maintenance_end(id,now()) IS NULL AND (lease_until IS NULL OR lease_until<now()) ORDER BY next_check LIMIT 32 FOR UPDATE SKIP LOCKED) UPDATE monitors m SET lease_token=$1,lease_until=now()+interval '90 seconds' FROM due WHERE m.id=due.id RETURNING m.id`, token)
	if e != nil {
		return nil, e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return nil, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	ms := []domain.Monitor{}
	for _, id := range ids {
		m, e := s.Monitor(ctx, id)
		if errors.Is(e, pgx.ErrNoRows) {
			continue
		}
		if e != nil {
			return nil, e
		}
		if m.Lease == token {
			ms = append(ms, m)
		}
	}
	return ms, nil
}
func (s *Store) Record(ctx context.Context, m domain.Monitor, c domain.Check) error {
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var status string
	var f, r int
	e = tx.QueryRow(ctx, `SELECT status,failures,successes FROM monitors WHERE id=$1 AND lease_token=$2 AND lease_until>now() FOR UPDATE`, m.ID, m.Lease).Scan(&status, &f, &r)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil
	}
	if e != nil {
		return e
	}
	var maintenance bool
	if e = tx.QueryRow(ctx, `SELECT COALESCE(maintenance_until>clock_timestamp(),false) OR active_maintenance_end(id,clock_timestamp()) IS NOT NULL FROM monitors WHERE id=$1`, m.ID).Scan(&maintenance); e != nil {
		return e
	}
	if maintenance {
		return nil
	}
	if m.Type == "heartbeat" {
		// Receipt takes the same monitor-row lock, so freshness is evaluated
		// inside the fenced recording transaction, never from a stale claim.
		var last *time.Time
		if e = tx.QueryRow(ctx, `SELECT last_received,clock_timestamp() FROM heartbeats WHERE monitor_id=$1`, m.ID).Scan(&last, &c.At); e != nil {
			return e
		}
		c.OK = m.HeartbeatFresh(last, c.At)
		c.Latency, c.Code = 0, 0
		c.Reason = "heartbeat_missed"
		if c.OK {
			c.Reason = "heartbeat_received"
		}
	}
	status, f, r, event := domain.Transition(status, f, r, c.OK, m.FailureThreshold, m.RecoveryThreshold)
	_, e = tx.Exec(ctx, `INSERT INTO checks(monitor_id,checked_at,ok,latency_ms,status_code,reason) VALUES($1,$2,$3,$4,$5,$6)`, m.ID, c.At, c.OK, c.Latency, c.Code, c.Reason)
	if e != nil {
		return e
	}
	// Advance the original schedule past now; completion-relative intervals drift across cron ticks.
	_, e = tx.Exec(ctx, `UPDATE monitors SET status=$3,failures=$4,successes=$5,last_check=$6,next_check=next_check+(floor(extract(epoch FROM (now()-next_check))/interval_seconds)+1)*interval_seconds*interval '1 second',lease_token=NULL,lease_until=NULL WHERE id=$1 AND lease_token=$2`, m.ID, m.Lease, status, f, r, c.At)
	if e != nil {
		return e
	}
	if event != "" {
		incidentID := cryptobox.Token()
		var openedAt time.Time
		if event == "opened" {
			e = tx.QueryRow(ctx, `INSERT INTO incidents(id,monitor_id) VALUES($1,$2) RETURNING opened_at`, incidentID, m.ID).Scan(&openedAt)
		} else {
			e = tx.QueryRow(ctx, `UPDATE incidents SET resolved_at=now() WHERE monitor_id=$1 AND resolved_at IS NULL RETURNING id,opened_at`, m.ID).Scan(&incidentID, &openedAt)
		}
		if e != nil {
			return e
		}
		payload, e := json.Marshal(domain.IncidentEvent{ID: cryptobox.Token(), Type: "incident." + event, IncidentID: incidentID, MonitorID: m.ID, MonitorName: m.Name, At: c.At, OpenedAt: openedAt, Group: m.Group, MonitorType: m.Type, Interval: m.Interval, Grace: m.Grace, FailureThreshold: m.FailureThreshold, RecoveryThreshold: m.RecoveryThreshold, ExpectedStatus: m.ExpectedStatus, Check: &c})
		if e != nil {
			return fmt.Errorf("encode incident notification: %w", e)
		}
		_, e = tx.Exec(ctx, `INSERT INTO deliveries(id,channel_id,payload) SELECT $1||':'||id,id,$2 FROM channels WHERE monitor_group IS NULL OR monitor_group=$3`, cryptobox.Token(), payload, m.Group)
		if e != nil {
			return e
		}
	}
	return tx.Commit(ctx)
}
func (s *Store) Cleanup(ctx context.Context) error {
	_, e := s.DB.Exec(ctx, `DELETE FROM checks WHERE id IN (SELECT id FROM checks WHERE checked_at<now()-(SELECT retention_days FROM settings WHERE id=1)*interval '1 day' LIMIT 5000)`)
	if e != nil {
		return e
	}
	for _, q := range []string{`DELETE FROM scheduler_runs WHERE id IN (SELECT id FROM scheduler_runs WHERE started_at<now()-interval '30 days' LIMIT 5000)`, `DELETE FROM sessions WHERE expires_at<now()`, `DELETE FROM rate_limits WHERE resets_at<now()`, `DELETE FROM deliveries WHERE COALESCE(delivered_at,failed_at)<now()-interval '30 days'`, `DELETE FROM incidents WHERE resolved_at<now()-(SELECT retention_days FROM settings WHERE id=1)*interval '1 day'`} {
		if _, e = s.DB.Exec(ctx, q); e != nil {
			return fmt.Errorf("cleanup: %w", e)
		}
	}
	return nil
}
