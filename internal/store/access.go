package store

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"uptivra/internal/cryptobox"
)

func validCredentials(email, password string) bool {
	address, err := mail.ParseAddress(email)
	return err == nil && address.Address == email && len(email) <= 254 && len(password) >= 14 && len(password) <= 72
}
func (s *Store) Bootstrap(ctx context.Context, email, password string) error {
	if !validCredentials(email, password) {
		return errors.New("valid email and a password of 14–72 bytes required")
	}
	h, e := bcrypt.GenerateFromPassword([]byte(password), 12)
	if e != nil {
		return e
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(884128)`); e != nil {
		return e
	}
	var exists bool
	if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM accounts)`).Scan(&exists); e != nil {
		return e
	}
	if exists {
		return errors.New("administrator already exists")
	}
	if _, e = tx.Exec(ctx, `INSERT INTO accounts(id,email,password_hash,owner) VALUES(1,$1,$2,true)`, strings.ToLower(email), string(h)); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (s *Store) Login(ctx context.Context, email, password string) (string, error) {
	var got, hash string
	var id int
	e := s.DB.QueryRow(ctx, `SELECT id,email,password_hash FROM accounts WHERE lower(email)=lower($1)`, email).Scan(&id, &got, &hash)
	if e != nil {
		hash = "$2a$12$YB6xQQ9Vskz0TgD8AHN66.FvN01F1Ad9X55zTwJMPt5iF8nbtspLa"
	}
	passErr := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if e != nil || passErr != nil || !strings.EqualFold(got, email) {
		return "", errors.New("invalid credentials")
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return "", e
	}
	defer tx.Rollback(ctx)
	// Serialize session issuance with password changes without holding a lock during bcrypt.
	var current string
	if e = tx.QueryRow(ctx, `SELECT password_hash FROM accounts WHERE id=$1 FOR SHARE`, id).Scan(&current); e != nil {
		return "", e
	}
	if current != hash {
		return "", errors.New("credentials changed; sign in again")
	}
	token := cryptobox.Token()
	if _, e = tx.Exec(ctx, `INSERT INTO sessions(token_hash,expires_at,account_id) VALUES($1,$2,$3)`, cryptobox.Hash(token), time.Now().Add(12*time.Hour), id); e != nil {
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
	return s.ChangeAdminPassword(ctx, 1, old, next)
}
func (s *Store) ChangeAdminPassword(ctx context.Context, id int, old, next string) error {
	if len(next) < 14 || len(next) > 72 {
		return errors.New("password must contain 14–72 bytes")
	}
	var hash string
	if e := s.DB.QueryRow(ctx, `SELECT password_hash FROM accounts WHERE id=$1`, id).Scan(&hash); e != nil {
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
	updated, e := tx.Exec(ctx, `UPDATE accounts SET password_hash=$1 WHERE id=$3 AND password_hash=$2`, string(h), hash, id)
	if e != nil {
		return e
	}
	if updated.RowsAffected() != 1 {
		return errors.New("credentials changed; sign in again")
	}
	if _, e = tx.Exec(ctx, `DELETE FROM sessions WHERE account_id=$1`, id); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// ResetPassword is a trusted CLI recovery operation requiring database access.
// The account lock also fences session issuance by concurrent logins.
func (s *Store) ResetPassword(ctx context.Context, email, next string) error {
	if email == "" || len(email) > 254 || len(next) < 14 || len(next) > 72 {
		return errors.New("email and a password of 14–72 bytes required")
	}
	h, e := bcrypt.GenerateFromPassword([]byte(next), 12)
	if e != nil {
		return e
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return errors.New("password reset transaction unavailable")
	}
	defer tx.Rollback(ctx)
	updated, e := tx.Exec(ctx, `UPDATE accounts SET password_hash=$1 WHERE email=$2`, string(h), email)
	if e != nil {
		return errors.New("password reset failed")
	}
	if updated.RowsAffected() != 1 {
		return errors.New("administrator email not found (match exact stored spelling)")
	}
	if _, e = tx.Exec(ctx, `DELETE FROM sessions WHERE account_id IN (SELECT id FROM accounts WHERE email=$1)`, email); e != nil {
		return errors.New("password reset session revocation failed")
	}
	if e = tx.Commit(ctx); e != nil {
		return errors.New("password reset commit failed")
	}
	return nil
}

// Admin is a safe projection; credentials never leave the store.
type Admin struct {
	ID    int    `json:"id"`
	Email string `json:"email"`
	Owner bool   `json:"owner"`
}

var ErrAdminAccess = errors.New("owner access required; owner and current account cannot be deleted")

func (s *Store) SetupRequired(ctx context.Context) (bool, error) {
	var exists bool
	err := s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM accounts)`).Scan(&exists)
	return !exists, err
}
func (s *Store) SessionAdmin(ctx context.Context, token string) (Admin, error) {
	var a Admin
	err := s.DB.QueryRow(ctx, `SELECT a.id,a.email,a.owner FROM accounts a JOIN sessions s ON s.account_id=a.id WHERE s.token_hash=$1 AND s.expires_at>now()`, cryptobox.Hash(token)).Scan(&a.ID, &a.Email, &a.Owner)
	return a, err
}
func (s *Store) Admins(ctx context.Context) ([]Admin, error) {
	rows, err := s.DB.Query(ctx, `SELECT id,email,owner FROM accounts ORDER BY id LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Admin{}
	for rows.Next() {
		var a Admin
		if err = rows.Scan(&a.ID, &a.Email, &a.Owner); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// Serialize ownership changes and validate the acting session inside the transaction.
func ownerTransaction(ctx context.Context, tx pgx.Tx, token string) (int, error) {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(884128)`); err != nil {
		return 0, err
	}
	var id int
	err := tx.QueryRow(ctx, `SELECT a.id FROM accounts a JOIN sessions s ON s.account_id=a.id WHERE a.owner AND s.token_hash=$1 AND s.expires_at>now() FOR UPDATE OF a`, cryptobox.Hash(token)).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrAdminAccess
	}
	return id, err
}
func (s *Store) CreateAdmin(ctx context.Context, token, email, password string) error {
	if !validCredentials(email, password) {
		return errors.New("valid email and a password of 14–72 bytes required")
	}
	h, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return err
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = ownerTransaction(ctx, tx, token); err != nil {
		return err
	}
	var count int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM accounts`).Scan(&count); err != nil {
		return err
	}
	if count >= 100 {
		return errors.New("administrator limit reached")
	}
	if _, err = tx.Exec(ctx, `INSERT INTO accounts(email,password_hash) VALUES($1,$2)`, strings.ToLower(email), string(h)); err != nil {
		return errors.New("administrator could not be created; email may already exist")
	}
	return tx.Commit(ctx)
}
func (s *Store) ManageAdmin(ctx context.Context, token string, target int, transfer bool) error {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	actor, err := ownerTransaction(ctx, tx, token)
	if err != nil {
		return err
	}
	if actor == target {
		return ErrAdminAccess
	}
	var owner bool
	if err = tx.QueryRow(ctx, `SELECT owner FROM accounts WHERE id=$1 FOR UPDATE`, target).Scan(&owner); err != nil {
		return err
	}
	if owner {
		return ErrAdminAccess
	}
	if transfer {
		if _, err = tx.Exec(ctx, `UPDATE accounts SET owner=false WHERE id=$1`, actor); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE accounts SET owner=true WHERE id=$1`, target)
	} else {
		_, err = tx.Exec(ctx, `DELETE FROM accounts WHERE id=$1 AND NOT owner`, target)
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
