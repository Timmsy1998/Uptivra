package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"uptivra/internal/cryptobox"
)

// RotateHeartbeatToken returns the plaintext once; only its hash is persisted.
func (s *Store) RotateHeartbeatToken(ctx context.Context, id string) (string, error) {
	token := cryptobox.Token()
	tag, err := s.DB.Exec(ctx, `UPDATE heartbeats SET token_hash=$2 WHERE monitor_id=$1`, id, cryptobox.Hash(token))
	if err != nil {
		return "", err
	}
	if tag.RowsAffected() != 1 {
		return "", pgx.ErrNoRows
	}
	return token, nil
}

var ErrHeartbeatToken = errors.New("heartbeat authentication required")

// ReceiveHeartbeat serializes receipt with recording, edits and deletion.
func (s *Store) ReceiveHeartbeat(ctx context.Context, id, token string) error {
	if token == "" || len(token) > 128 {
		return ErrHeartbeatToken
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var found string
	err = tx.QueryRow(ctx, `SELECT m.id FROM monitors m JOIN heartbeats h ON h.monitor_id=m.id WHERE m.id=$1 AND h.token_hash=$2 AND m.enabled FOR UPDATE OF m`, id, cryptobox.Hash(token)).Scan(&found)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrHeartbeatToken
	}
	if err != nil {
		return err
	}
	// Recheck after acquiring the heartbeat row lock to fence token rotation.
	tag, err := tx.Exec(ctx, `UPDATE heartbeats SET last_received=clock_timestamp() WHERE monitor_id=$1 AND token_hash=$2`, id, cryptobox.Hash(token))
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrHeartbeatToken
	}
	return tx.Commit(ctx)
}
