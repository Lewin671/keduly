package core

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/Lewin671/keduly/internal/api"
	"github.com/Lewin671/keduly/internal/store"
)

// Signing in on another device without the password. Either the new device shows a QR code and
// a signed-in one approves it (a login request), or a signed-in device shows a QR code that the
// new one opens (a login code). What makes each safe:
//
//   - A request's QR code holds only its id. The session goes to whoever holds the secret, which
//     never leaves the device that asked, and only after someone signed in typed the pin shown on
//     that device's screen: a link sent to a victim is not enough, they must also see the screen.
//   - A code is a credential, so it is random, stored hashed, travels in the URL fragment (which
//     browsers do not send to servers), and is redeemed by a POST the user has to confirm.
//   - Both live for LinkTTL and are deleted by the statement that uses them.
const (
	LinkTTL     = 2 * time.Minute
	pinAttempts = 3
)

var errLinkGone = NotFound("sign-in request or code")

func newPin() string {
	n, err := rand.Int(rand.Reader, big.NewInt(10000))
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%04d", n.Int64())
}

// deviceLabel names the browser and system of a User-Agent from a fixed vocabulary, so the
// approval screen never shows text that the requesting device chose.
func deviceLabel(userAgent string) string {
	has := func(s string) bool { return strings.Contains(userAgent, s) }
	var browser, system string
	switch {
	case has("Edg/"), has("EdgiOS/"), has("EdgA/"):
		browser = "Edge"
	case has("Firefox/"), has("FxiOS/"):
		browser = "Firefox"
	case has("Chrome/"), has("CriOS/"):
		browser = "Chrome"
	case has("Safari/"):
		browser = "Safari"
	}
	switch {
	case has("iPhone"):
		system = "iPhone"
	case has("iPad"):
		system = "iPad"
	case has("Android"):
		system = "Android"
	case has("Macintosh"):
		system = "Mac"
	case has("Windows"):
		system = "Windows"
	case has("Linux"), has("X11"):
		system = "Linux"
	}
	if browser == "" || system == "" {
		return browser + system
	}
	return browser + " · " + system
}

func noFields(f Fields) error { return newReader(f).done() }

func loginRequestJSON(l *store.LoginLink) *api.LoginRequest {
	return &api.LoginRequest{ID: l.ID, Device: l.Device, ExpiresAt: l.ExpiresAt}
}

// StartLoginRequest is called by a device that wants to be signed in. It returns the secret that
// device claims the session with and the pin it shows next to the QR code.
func (s *Service) StartLoginRequest(ctx context.Context, f Fields, userAgent string) (req *api.LoginRequest, pin, secret string, err error) {
	if err := noFields(f); err != nil {
		return nil, "", "", err
	}
	now := s.now()
	_ = store.DeleteExpiredLoginLinks(ctx, s.DB, store.FormatTime(now))
	secret, hash := newSecret("")
	l := &store.LoginLink{ID: NewID(), Kind: "request", SecretHash: hash, Pin: newPin(), Device: deviceLabel(userAgent),
		ExpiresAt: store.FormatTime(now.Add(LinkTTL)), CreatedAt: store.FormatTime(now)}
	if err := store.InsertLoginLink(ctx, s.DB, l); err != nil {
		return nil, "", "", err
	}
	return loginRequestJSON(l), l.Pin, secret, nil
}

// ClaimLoginRequest is polled by the device that asked. It returns a nil user while the request
// waits, and the new session's secret once it was approved; that works exactly once.
func (s *Service) ClaimLoginRequest(ctx context.Context, id string, f Fields) (*api.User, string, error) {
	var secret string
	r := newReader(f)
	r.str("secret", &secret, 200)
	if err := r.done(); err != nil {
		return nil, "", err
	}
	now, hash := store.FormatTime(s.now()), hashSecret(secret)
	userID, ok, err := store.TakeLoginRequest(ctx, s.DB, id, hash, now)
	if err != nil {
		return nil, "", err
	}
	if !ok {
		l, err := store.LoginRequestByID(ctx, s.DB, id, now)
		if err != nil {
			return nil, "", err
		}
		if l == nil || l.SecretHash != hash {
			return nil, "", errLinkGone
		}
		return nil, "", nil
	}
	return s.signIn(ctx, userID)
}

// signIn starts a session for an account that a login request or code vouched for.
func (s *Service) signIn(ctx context.Context, userID string) (*api.User, string, error) {
	u, err := store.UserByID(ctx, s.DB, userID)
	if err != nil {
		return nil, "", err
	}
	if u == nil {
		return nil, "", errLinkGone
	}
	session, err := s.startSession(ctx, u.ID)
	if err != nil {
		return nil, "", err
	}
	out := userJSON(u)
	return &out, session, nil
}

// LoginRequest describes a pending request to the signed-in user who scanned it. It leaves the
// pin out: the user has to read it off the other device.
func (s *Service) LoginRequest(ctx context.Context, requestID string) (*api.LoginRequest, error) {
	l, err := store.LoginRequestByID(ctx, s.DB, requestID, store.FormatTime(s.now()))
	if err != nil {
		return nil, err
	}
	if l == nil || l.UserID != nil {
		return nil, errLinkGone
	}
	return loginRequestJSON(l), nil
}

// ApproveLoginRequest lets the requesting device sign in as id's user. A wrong pin is counted,
// and the request is deleted at the third.
func (s *Service) ApproveLoginRequest(ctx context.Context, id *Identity, requestID string, f Fields) error {
	var pin string
	r := newReader(f)
	r.str("pin", &pin, 4)
	if err := r.done(); err != nil {
		return err
	}
	now := store.FormatTime(s.now())
	ok, err := store.ApproveLoginRequest(ctx, s.DB, requestID, pin, id.User.ID, now, pinAttempts)
	if err != nil || ok {
		return err
	}
	attempts, pending, err := store.CountLoginRequestMiss(ctx, s.DB, requestID, now)
	if err != nil {
		return err
	}
	if !pending {
		return errLinkGone
	}
	if attempts >= pinAttempts {
		if err := store.DeleteLoginRequest(ctx, s.DB, requestID); err != nil {
			return err
		}
		return errLinkGone
	}
	return Invalid("pin is not the number shown on the other device")
}

// RefuseLoginRequest ends a request, pending or approved but not yet claimed.
func (s *Service) RefuseLoginRequest(ctx context.Context, requestID string) error {
	return store.DeleteLoginRequest(ctx, s.DB, requestID)
}

// CreateLoginCode issues the one code of id's user that signs another device in, replacing any
// earlier one.
func (s *Service) CreateLoginCode(ctx context.Context, id *Identity, f Fields) (code, expiresAt string, err error) {
	if err := noFields(f); err != nil {
		return "", "", err
	}
	now := s.now()
	_ = store.DeleteExpiredLoginLinks(ctx, s.DB, store.FormatTime(now))
	if err := store.DeleteLoginLinksOf(ctx, s.DB, id.User.ID, "code"); err != nil {
		return "", "", err
	}
	code, hash := newSecret("")
	l := &store.LoginLink{ID: NewID(), Kind: "code", SecretHash: hash, UserID: &id.User.ID,
		ExpiresAt: store.FormatTime(now.Add(LinkTTL)), CreatedAt: store.FormatTime(now)}
	return code, l.ExpiresAt, store.InsertLoginLink(ctx, s.DB, l)
}

// RevokeLoginCode withdraws the code of id's user, if one is live.
func (s *Service) RevokeLoginCode(ctx context.Context, id *Identity) error {
	return store.DeleteLoginLinksOf(ctx, s.DB, id.User.ID, "code")
}

func readCode(f Fields) (hash string, err error) {
	var code string
	r := newReader(f)
	r.str("code", &code, 200)
	if err := r.done(); err != nil {
		return "", err
	}
	return hashSecret(code), nil
}

// CheckLoginCode says whose account a code signs in to, so the device can ask before it does.
func (s *Service) CheckLoginCode(ctx context.Context, f Fields) (name, email string, err error) {
	hash, err := readCode(f)
	if err != nil {
		return "", "", err
	}
	userID, ok, err := store.LoginCodeUser(ctx, s.DB, hash, store.FormatTime(s.now()))
	if err != nil {
		return "", "", err
	}
	if !ok {
		return "", "", errLinkGone
	}
	u, err := store.UserByID(ctx, s.DB, userID)
	if err != nil {
		return "", "", err
	}
	if u == nil {
		return "", "", errLinkGone
	}
	return u.Name, u.Email, nil
}

// RedeemLoginCode uses a code up and returns the new session's secret.
func (s *Service) RedeemLoginCode(ctx context.Context, f Fields) (*api.User, string, error) {
	hash, err := readCode(f)
	if err != nil {
		return nil, "", err
	}
	userID, ok, err := store.TakeLoginCode(ctx, s.DB, hash, store.FormatTime(s.now()))
	if err != nil {
		return nil, "", err
	}
	if !ok {
		return nil, "", errLinkGone
	}
	return s.signIn(ctx, userID)
}
