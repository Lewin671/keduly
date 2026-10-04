package store

import (
	"context"
	"database/sql"
	"errors"
)

// LoginLink is one attempt to sign a second device in; see migration 0007.
type LoginLink struct {
	ID         string
	Kind       string // "request" or "code"
	SecretHash string
	UserID     *string
	Pin        string
	Device     string
	ExpiresAt  string
	CreatedAt  string
}

func InsertLoginLink(ctx context.Context, q Q, l *LoginLink) error {
	_, err := q.ExecContext(ctx, `INSERT INTO login_links (id, kind, secret_hash, user_id, pin, device, expires_at, created_at)
		VALUES (?,?,?,?,?,?,?,?)`, l.ID, l.Kind, l.SecretHash, l.UserID, l.Pin, l.Device, l.ExpiresAt, l.CreatedAt)
	return err
}

func DeleteExpiredLoginLinks(ctx context.Context, q Q, now string) error {
	_, err := q.ExecContext(ctx, "DELETE FROM login_links WHERE expires_at <= ?", now)
	return err
}

// DeleteLoginLinksOf removes what a user issued or approved; kind "" means both kinds.
func DeleteLoginLinksOf(ctx context.Context, q Q, userID, kind string) error {
	_, err := q.ExecContext(ctx, "DELETE FROM login_links WHERE user_id = ? AND (kind = ? OR ? = '')", userID, kind, kind)
	return err
}

func DeleteLoginRequest(ctx context.Context, q Q, id string) error {
	_, err := q.ExecContext(ctx, "DELETE FROM login_links WHERE id = ? AND kind = 'request'", id)
	return err
}

// LoginRequestByID returns a request that has not expired, or nil.
func LoginRequestByID(ctx context.Context, q Q, id, now string) (*LoginLink, error) {
	l := &LoginLink{Kind: "request"}
	err := q.QueryRowContext(ctx, `SELECT id, secret_hash, user_id, pin, device, expires_at, created_at FROM login_links
		WHERE id = ? AND kind = 'request' AND expires_at > ?`, id, now).
		Scan(&l.ID, &l.SecretHash, &l.UserID, &l.Pin, &l.Device, &l.ExpiresAt, &l.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return l, err
}

// ApproveLoginRequest hands a pending request to userID when pin matches and reports whether it did.
func ApproveLoginRequest(ctx context.Context, q Q, id, pin, userID, now string, maxAttempts int) (bool, error) {
	res, err := q.ExecContext(ctx, `UPDATE login_links SET user_id = ?
		WHERE id = ? AND kind = 'request' AND user_id IS NULL AND pin = ? AND attempts < ? AND expires_at > ?`,
		userID, id, pin, maxAttempts, now)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// CountLoginRequestMiss records a wrong pin on a pending request and returns how many there
// have been; ok is false when there is no such pending request.
func CountLoginRequestMiss(ctx context.Context, q Q, id, now string) (attempts int, ok bool, err error) {
	err = q.QueryRowContext(ctx, `UPDATE login_links SET attempts = attempts + 1
		WHERE id = ? AND kind = 'request' AND user_id IS NULL AND expires_at > ? RETURNING attempts`, id, now).Scan(&attempts)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	return attempts, err == nil, err
}

// TakeLoginRequest deletes an approved request held by the device with this secret and returns
// the user who approved it. Deleting and reading in one statement makes it single-use.
func TakeLoginRequest(ctx context.Context, q Q, id, secretHash, now string) (userID string, ok bool, err error) {
	err = q.QueryRowContext(ctx, `DELETE FROM login_links
		WHERE id = ? AND kind = 'request' AND secret_hash = ? AND user_id IS NOT NULL AND expires_at > ? RETURNING user_id`,
		id, secretHash, now).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return userID, err == nil, err
}

// LoginCodeUser returns the account a live code signs in to, without using the code up.
func LoginCodeUser(ctx context.Context, q Q, secretHash, now string) (userID string, ok bool, err error) {
	err = q.QueryRowContext(ctx, `SELECT user_id FROM login_links
		WHERE kind = 'code' AND secret_hash = ? AND expires_at > ?`, secretHash, now).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return userID, err == nil, err
}

// TakeLoginCode deletes a live code and returns its account; like TakeLoginRequest it is single-use.
func TakeLoginCode(ctx context.Context, q Q, secretHash, now string) (userID string, ok bool, err error) {
	err = q.QueryRowContext(ctx, `DELETE FROM login_links
		WHERE kind = 'code' AND secret_hash = ? AND expires_at > ? RETURNING user_id`, secretHash, now).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return userID, err == nil, err
}
