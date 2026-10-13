package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"uptivra/internal/cryptobox"
	"uptivra/internal/netguard"
)

type Channel struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Kind          string `json:"kind"`
	URL           string `json:"url,omitempty"`
	SigningSecret string `json:"signing_secret,omitempty"`
}

func (s *Store) Channels(ctx context.Context) ([]Channel, error) {
	rows, e := s.DB.Query(ctx, `SELECT id,name,kind FROM channels ORDER BY created_at`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Channel{}
	for rows.Next() {
		var c Channel
		if e = rows.Scan(&c.ID, &c.Name, &c.Kind); e != nil {
			return nil, e
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (s *Store) CreateChannel(ctx context.Context, c Channel) error {
	if len(c.Name) < 1 || len(c.Name) > 80 || (c.Kind != "webhook" && c.Kind != "discord") {
		return errors.New("name and webhook/discord kind required")
	}
	if _, e := netguard.URL(c.URL); e != nil {
		return e
	}
	if c.Kind == "webhook" && (len(c.SigningSecret) < 32 || len(c.SigningSecret) > 256) {
		return errors.New("webhook signing secret must contain 32–256 bytes")
	}
	b, _ := json.Marshal(c)
	_, e := s.DB.Exec(ctx, `INSERT INTO channels(id,name,kind,secret_config) VALUES($1,$2,$3,$4)`, cryptobox.Token(), c.Name, c.Kind, s.Box.Seal(string(b)))
	return e
}
func (s *Store) DeleteChannel(ctx context.Context, id string) error {
	_, e := s.DB.Exec(ctx, `DELETE FROM channels WHERE id=$1`, id)
	return e
}

type Delivery struct {
	ID, Lease string
	Payload   json.RawMessage
	Attempts  int
	Channel   Channel
}

func (s *Store) ClaimDeliveries(ctx context.Context) ([]Delivery, error) {
	token := cryptobox.Token()
	rows, e := s.DB.Query(ctx, `WITH due AS (SELECT id FROM deliveries WHERE delivered_at IS NULL AND failed_at IS NULL AND next_attempt<=now() AND (lease_until IS NULL OR lease_until<now()) ORDER BY next_attempt LIMIT 16 FOR UPDATE SKIP LOCKED) UPDATE deliveries d SET lease_token=$1,lease_until=now()+interval '90 seconds' FROM due,channels c WHERE d.id=due.id AND c.id=d.channel_id RETURNING d.id,d.payload,d.attempts,c.secret_config`, token)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Delivery{}
	for rows.Next() {
		d := Delivery{Lease: token}
		var sec string
		if e = rows.Scan(&d.ID, &d.Payload, &d.Attempts, &sec); e != nil {
			return nil, e
		}
		plain, e := s.Box.Open(sec)
		if e != nil {
			return nil, e
		}
		if e = json.Unmarshal([]byte(plain), &d.Channel); e != nil {
			return nil, e
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
func (s *Store) FinishDelivery(ctx context.Context, d Delivery, ok bool) error {
	delay := time.Duration(1<<min(d.Attempts, 8)) * time.Minute
	_, e := s.DB.Exec(ctx, `UPDATE deliveries SET attempts=attempts+1,next_attempt=$3,delivered_at=CASE WHEN $4 THEN now() ELSE NULL END,failed_at=CASE WHEN NOT $4 AND attempts>=7 THEN now() ELSE NULL END,lease_token=NULL,lease_until=NULL WHERE id=$1 AND lease_token=$2 AND lease_until>now()`, d.ID, d.Lease, time.Now().Add(delay), ok)
	return e
}
