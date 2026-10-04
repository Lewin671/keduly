package core

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"

	"github.com/Lewin671/keduly/internal/api"
	"github.com/Lewin671/keduly/internal/store"
)

const (
	SessionTTL  = 30 * 24 * time.Hour
	TokenPrefix = "kdl_"

	argonTime    = 2
	argonMemory  = 19 * 1024 // KiB
	argonThreads = 1
	argonKeyLen  = 32
)

func (s *Service) hashPassword(password string) string {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		panic(err)
	}
	s.hashSlots <- struct{}{}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	<-s.hashSlots
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key))
}

func (s *Service) verifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var memory, iterations uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &threads); err != nil {
		return false
	}
	salt, err1 := base64.RawStdEncoding.DecodeString(parts[4])
	want, err2 := base64.RawStdEncoding.DecodeString(parts[5])
	if err1 != nil || err2 != nil {
		return false
	}
	s.hashSlots <- struct{}{}
	got := argon2.IDKey([]byte(password), salt, iterations, memory, threads, uint32(len(want)))
	<-s.hashSlots
	return subtle.ConstantTimeCompare(got, want) == 1
}

// newSecret returns a random 32-byte secret; only its hash is stored.
func newSecret(prefix string) (secret, hash string) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	secret = prefix + base64.RawURLEncoding.EncodeToString(b)
	return secret, hashSecret(secret)
}

func hashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func validZone(name string) bool {
	if name == "" || name == "Local" {
		return false
	}
	_, err := time.LoadLocation(name)
	return err == nil
}

func (s *Service) now() time.Time { return s.Now().UTC().Truncate(time.Second) }

// Register creates an account and its first session.
func (s *Service) Register(ctx context.Context, f Fields) (*api.User, string, error) {
	if s.Registration != "open" {
		return nil, "", &Error{403, "registration_closed", "registration is closed on this server"}
	}
	var email, password, name string
	zone := "UTC"
	r := newReader(f)
	r.str("email", &email, 254)
	r.str("password", &password, 1024)
	r.str("name", &name, 100)
	r.str("timezone", &zone, 64)
	if err := r.done(); err != nil {
		return nil, "", err
	}
	email, name = strings.ToLower(strings.TrimSpace(email)), strings.TrimSpace(name)
	addr, err := mail.ParseAddress(email)
	switch {
	case err != nil || addr.Address != email || !strings.Contains(email, "."):
		return nil, "", Invalid("email must be a valid email address")
	case len(password) < 8:
		return nil, "", Invalid("password must be at least 8 characters")
	case !validZone(zone):
		return nil, "", Invalid("timezone must be an IANA time zone such as Asia/Shanghai")
	}
	if name == "" {
		name = email[:strings.Index(email, "@")]
	}
	if existing, err := store.UserByEmail(ctx, s.DB, email); err != nil {
		return nil, "", err
	} else if existing != nil {
		return nil, "", Conflict("email is already registered")
	}
	u := &store.User{ID: NewID(), Email: email, Name: name, PasswordHash: s.hashPassword(password),
		Timezone: zone, WorkStart: "09:00", WorkEnd: "18:00", CreatedAt: store.FormatTime(s.now())}
	if err := store.InsertUser(ctx, s.DB, u); err != nil {
		if strings.Contains(err.Error(), "constraint failed") {
			return nil, "", Conflict("email is already registered")
		}
		return nil, "", err
	}
	secret, err := s.startSession(ctx, u.ID)
	out := userJSON(u)
	return &out, secret, err
}

func (s *Service) startSession(ctx context.Context, userID string) (string, error) {
	secret, hash := newSecret("")
	now := s.now()
	_ = store.DeleteExpiredSessions(ctx, s.DB, store.FormatTime(now))
	return secret, store.InsertSession(ctx, s.DB, &store.Session{Hash: hash, UserID: userID,
		ExpiresAt: store.FormatTime(now.Add(SessionTTL)), CreatedAt: store.FormatTime(now)})
}

// dummyHash keeps the cost of a login the same whether or not the email exists.
const dummyHash = "$argon2id$v=19$m=19456,t=2,p=1$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

func (s *Service) Login(ctx context.Context, f Fields) (*api.User, string, error) {
	var email, password string
	r := newReader(f)
	r.str("email", &email, 254)
	r.str("password", &password, 1024)
	if err := r.done(); err != nil {
		return nil, "", err
	}
	u, err := store.UserByEmail(ctx, s.DB, strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		return nil, "", err
	}
	hash := dummyHash
	if u != nil {
		hash = u.PasswordHash
	}
	if !s.verifyPassword(hash, password) || u == nil {
		return nil, "", &Error{401, "unauthenticated", "wrong email or password"}
	}
	secret, err := s.startSession(ctx, u.ID)
	out := userJSON(u)
	return &out, secret, err
}

// Session resolves a session cookie. renewed is true when the sliding expiry
// was pushed out and the cookie should be sent again.
func (s *Service) Session(ctx context.Context, secret string) (id *Identity, renewed bool, err error) {
	hash := hashSecret(secret)
	sess, err := store.SessionByHash(ctx, s.DB, hash)
	if err != nil {
		return nil, false, err
	}
	now := s.now()
	if sess == nil || !store.ParseTime(sess.ExpiresAt).After(now) {
		return nil, false, ErrUnauthenticated
	}
	u, err := store.UserByID(ctx, s.DB, sess.UserID)
	if err != nil || u == nil {
		return nil, false, ErrUnauthenticated
	}
	if store.ParseTime(sess.ExpiresAt).Sub(now) < SessionTTL-24*time.Hour {
		if err := store.ExtendSession(ctx, s.DB, hash, store.FormatTime(now.Add(SessionTTL))); err != nil {
			return nil, false, err
		}
		renewed = true
	}
	return &Identity{User: u, Actor: api.Actor{Kind: "user", Name: u.Name}, SessionHash: hash}, renewed, nil
}

func (s *Service) Logout(ctx context.Context, secret string) error {
	return store.DeleteSession(ctx, s.DB, hashSecret(secret))
}

// Bearer resolves an agent token used with the JSON API.
func (s *Service) Bearer(ctx context.Context, secret string) (*Identity, error) {
	return s.token(ctx, secret, "agent")
}

// Basic resolves the email and app password of a CalDAV client.
func (s *Service) Basic(ctx context.Context, email, password string) (*Identity, error) {
	id, err := s.token(ctx, password, "caldav")
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(strings.TrimSpace(email), id.User.Email) {
		return nil, ErrUnauthenticated
	}
	return id, nil
}

func (s *Service) token(ctx context.Context, secret, kind string) (*Identity, error) {
	if !strings.HasPrefix(secret, TokenPrefix) {
		return nil, ErrUnauthenticated
	}
	t, err := store.TokenByHash(ctx, s.DB, hashSecret(secret))
	if err != nil {
		return nil, err
	}
	if t == nil || t.Kind != kind {
		return nil, ErrUnauthenticated
	}
	u, err := store.UserByID(ctx, s.DB, t.UserID)
	if err != nil || u == nil {
		return nil, ErrUnauthenticated
	}
	now := s.now()
	// One write a minute is enough to show when a token was last used.
	if t.LastUsedAt == nil || now.Sub(store.ParseTime(*t.LastUsedAt)) >= time.Minute {
		_ = store.TouchToken(ctx, s.DB, t.ID, store.FormatTime(now))
	}
	return &Identity{User: u, Actor: api.Actor{Kind: kind, Name: t.Name}, Token: t}, nil
}

// UpdateMe changes the caller's profile.
func (op *Op) UpdateMe(f Fields) (*api.User, error) {
	u := *op.User
	r := newReader(f)
	r.name("name", &u.Name, 100)
	r.str("timezone", &u.Timezone, 64)
	r.boolean("timezone_auto", &u.TimezoneAuto)
	r.str("work_start", &u.WorkStart, 5)
	r.str("work_end", &u.WorkEnd, 5)
	if err := r.done(); err != nil {
		return nil, err
	}
	switch {
	case !validZone(u.Timezone):
		return nil, Invalid("timezone must be an IANA time zone such as Asia/Shanghai")
	case !validClock(u.WorkStart) || !validClock(u.WorkEnd):
		return nil, Invalid("work_start and work_end must be times of day (HH:MM)")
	case u.WorkStart >= u.WorkEnd:
		return nil, Invalid("work_end must be after work_start")
	}
	if err := store.UpdateUser(op.ctx, op.q, &u); err != nil {
		return nil, err
	}
	out := userJSON(&u)
	return &out, nil
}

// ChangePassword sets a new password and ends every other session.
func (op *Op) ChangePassword(f Fields) error {
	var current, next string
	r := newReader(f)
	r.str("current", &current, 1024)
	r.str("new", &next, 1024)
	if err := r.done(); err != nil {
		return err
	}
	if len(next) < 8 {
		return Invalid("new must be at least 8 characters")
	}
	if !op.svc.verifyPassword(op.User.PasswordHash, current) {
		return Invalid("current is not your current password")
	}
	u := *op.User
	u.PasswordHash = op.svc.hashPassword(next)
	if err := store.UpdateUser(op.ctx, op.q, &u); err != nil {
		return err
	}
	return store.DeleteOtherSessions(op.ctx, op.q, u.ID, op.ID.SessionHash)
}

func tokenJSON(t *store.Token) api.Token {
	return api.Token{ID: t.ID, Name: t.Name, Kind: t.Kind, Scope: t.Scope, ConfirmDelete: t.ConfirmDelete,
		LastUsedAt: t.LastUsedAt, CreatedAt: t.CreatedAt}
}

func (op *Op) Tokens() ([]api.Token, error) {
	rows, err := store.Tokens.List(op.ctx, op.q, op.User.ID, "")
	if err != nil {
		return nil, err
	}
	out := make([]api.Token, 0, len(rows))
	for _, t := range rows {
		out = append(out, tokenJSON(t))
	}
	return out, nil
}

// CreateToken returns the new token with its secret, which is shown only once.
func (op *Op) CreateToken(f Fields) (*api.Token, error) {
	t := &store.Token{ID: NewID(), UserID: op.User.ID, Scope: "write", ConfirmDelete: true, CreatedAt: op.now()}
	r := newReader(f)
	if !r.has("name") || !r.has("kind") {
		r.fail("name and kind are required")
	}
	r.name("name", &t.Name, 100)
	r.str("kind", &t.Kind, 10)
	r.str("scope", &t.Scope, 10)
	r.boolean("confirm_delete", &t.ConfirmDelete)
	if err := r.done(); err != nil {
		return nil, err
	}
	if t.Kind != "agent" && t.Kind != "caldav" {
		return nil, Invalid("kind must be agent or caldav")
	}
	if t.Scope != "read" && t.Scope != "write" {
		return nil, Invalid("scope must be read or write")
	}
	n, err := store.Tokens.Count(op.ctx, op.q, op.User.ID, "")
	if err != nil {
		return nil, err
	}
	if err := op.checkLimit(n, op.svc.Limits.Tokens, "tokens"); err != nil {
		return nil, err
	}
	var secret string
	secret, t.Hash = newSecret(TokenPrefix)
	if err := store.Tokens.Put(op.ctx, op.q, t); err != nil {
		return nil, err
	}
	out := tokenJSON(t)
	out.Token = secret
	return &out, nil
}

func (op *Op) DeleteToken(id string) error {
	t, err := store.Tokens.Get(op.ctx, op.q, op.User.ID, id)
	if err != nil {
		return err
	}
	if t == nil {
		return NotFound("token")
	}
	return store.Tokens.Delete(op.ctx, op.q, op.User.ID, id)
}
