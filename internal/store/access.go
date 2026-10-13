package store

import (
	"context"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"

	"uptivra/internal/cryptobox"
)

func (s *Store) Bootstrap(ctx context.Context, email, password string) error {
	if len(password) < 14 || len(password) > 72 || len(email) > 254 || email == "" {
		return errors.New("email and a password of 14–72 bytes required")
	}
	h, e := bcrypt.GenerateFromPassword([]byte(password), 12)
	if e != nil {
		return e
	}
	t, e := s.DB.Exec(ctx, `INSERT INTO accounts(id,email,password_hash) VALUES(1,$1,$2) ON CONFLICT DO NOTHING`, email, string(h))
	if e == nil && t.RowsAffected() == 0 {
		return errors.New("administrator already exists")
	}
	return e
}
func (s *Store) Login(ctx context.Context, email, password string) (string, error) {
	var got, hash string
	e := s.DB.QueryRow(ctx, `SELECT email,password_hash FROM accounts WHERE id=1`).Scan(&got, &hash)
	if e != nil {
		hash = "$2a$12$YB6xQQ9Vskz0TgD8AHN66.FvN01F1Ad9X55zTwJMPt5iF8nbtspLa"
	}
	passErr := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if e != nil || passErr != nil || got != email {
		return "", errors.New("invalid credentials")
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return "", e
	}
	defer tx.Rollback(ctx)
	// Serialize session issuance with password changes without holding a lock during bcrypt.
	var current string
	if e = tx.QueryRow(ctx, `SELECT password_hash FROM accounts WHERE id=1 FOR SHARE`).Scan(&current); e != nil {
		return "", e
	}
	if current != hash {
		return "", errors.New("credentials changed; sign in again")
	}
	token := cryptobox.Token()
	if _, e = tx.Exec(ctx, `INSERT INTO sessions(token_hash,expires_at) VALUES($1,$2)`, cryptobox.Hash(token), time.Now().Add(12*time.Hour)); e != nil {
		return "", e
	}
	return token, tx.Commit(ctx)
}
func (s *Store) Session(ctx context.Context, token string) bool {
	var ok bool
	e := s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM sessions WHERE token_hash=$1 AND expires_at>now())`, cryptobox.Hash(token)).Scan(&ok)
	return e == nil && ok
}
func (s *Store) Logout(ctx context.Context, token string) error {
	_, e := s.DB.Exec(ctx, `DELETE FROM sessions WHERE token_hash=$1`, cryptobox.Hash(token))
	return e
}
func (s *Store) RateLimit(ctx context.Context, bucket string, max int) bool {
	var n int
	e := s.DB.QueryRow(ctx, `INSERT INTO rate_limits(bucket,attempts,resets_at) VALUES($1,1,now()+interval '15 minutes') ON CONFLICT(bucket) DO UPDATE SET attempts=CASE WHEN rate_limits.resets_at<now() THEN 1 ELSE rate_limits.attempts+1 END,resets_at=CASE WHEN rate_limits.resets_at<now() THEN now()+interval '15 minutes' ELSE rate_limits.resets_at END RETURNING attempts`, cryptobox.Hash(bucket)).Scan(&n)
	return e == nil && n <= max
}

type Key struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Scope     string    `json:"scope"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Store) Keys(ctx context.Context) ([]Key, error) {
	rows, e := s.DB.Query(ctx, `SELECT id,name,scope,created_at FROM api_keys ORDER BY created_at DESC`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Key{}
	for rows.Next() {
		var k Key
		if e = rows.Scan(&k.ID, &k.Name, &k.Scope, &k.CreatedAt); e != nil {
			return nil, e
		}
		out = append(out, k)
	}
	return out, rows.Err()
}
func (s *Store) CreateKey(ctx context.Context, name, scope string) (string, error) {
	if len(name) < 1 || len(name) > 80 || (scope != "read" && scope != "write") {
		return "", errors.New("name and read/write scope required")
	}
	token := "up_" + cryptobox.Token()
	_, e := s.DB.Exec(ctx, `INSERT INTO api_keys(id,name,scope,token_hash) VALUES($1,$2,$3,$4)`, cryptobox.Token(), name, scope, cryptobox.Hash(token))
	return token, e
}
func (s *Store) KeyScope(ctx context.Context, token string) string {
	var scope string
	s.DB.QueryRow(ctx, `SELECT scope FROM api_keys WHERE token_hash=$1`, cryptobox.Hash(token)).Scan(&scope)
	return scope
}
func (s *Store) DeleteKey(ctx context.Context, id string) error {
	_, e := s.DB.Exec(ctx, `DELETE FROM api_keys WHERE id=$1`, id)
	return e
}
func (s *Store) ChangePassword(ctx context.Context, old, next string) error {
	if len(next) < 14 || len(next) > 72 {
		return errors.New("password must contain 14–72 bytes")
	}
	var hash string
	if e := s.DB.QueryRow(ctx, `SELECT password_hash FROM accounts WHERE id=1`).Scan(&hash); e != nil {
		return e
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(old)) != nil {
		return errors.New("current password is incorrect")
	}
	h, e := bcrypt.GenerateFromPassword([]byte(next), 12)
	if e != nil {
		return e
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	updated, e := tx.Exec(ctx, `UPDATE accounts SET password_hash=$1 WHERE id=1 AND password_hash=$2`, string(h), hash)
	if e != nil {
		return e
	}
	if updated.RowsAffected() != 1 {
		return errors.New("credentials changed; sign in again")
	}
	if _, e = tx.Exec(ctx, `DELETE FROM sessions`); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
