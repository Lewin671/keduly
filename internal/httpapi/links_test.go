package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/Lewin671/keduly/internal/api"
	"github.com/Lewin671/keduly/internal/testutil"
)

type loginRequest struct {
	Request api.LoginRequest `json:"request"`
	Pin     string           `json:"pin"`
	Secret  string           `json:"secret"`
}

func startRequest(t *testing.T, c *testutil.Client) loginRequest {
	t.Helper()
	var r loginRequest
	c.Call("POST", "/auth/requests", nil, http.StatusCreated, &r)
	if len(r.Pin) != 4 || len(r.Secret) < 40 || len(r.Request.ID) != 16 {
		t.Fatalf("unexpected login request %+v", r)
	}
	return r
}

// wrongPin is a pin that is certainly not r's.
func wrongPin(r loginRequest) string {
	if r.Pin == "0000" {
		return "0001"
	}
	return "0000"
}

func whoIs(t *testing.T, c *testutil.Client) string {
	t.Helper()
	var got struct {
		User api.User `json:"user"`
	}
	c.Call("GET", "/me", nil, http.StatusOK, &got)
	return got.User.Email
}

func TestLoginRequestApprovedByASignedInDevice(t *testing.T) {
	s, phone := setup(t)
	laptop := &testutil.Client{S: s}
	r := startRequest(t, laptop)
	claim := "/auth/requests/" + r.Request.ID + "/claim"

	var status struct {
		Status string `json:"status"`
	}
	laptop.Call("POST", claim, M{"secret": r.Secret}, http.StatusOK, &status)
	if status.Status != "pending" {
		t.Fatalf("status %q before approval", status.Status)
	}

	// The phone sees which device asks but never its pin, and approving needs a session.
	var seen struct {
		Request map[string]any `json:"request"`
	}
	phone.Call("GET", "/auth/requests/"+r.Request.ID, nil, http.StatusOK, &seen)
	if len(seen.Request) != 3 || seen.Request["id"] != r.Request.ID || seen.Request["expires_at"] == "" {
		t.Fatalf("the request shown to the approver must be id, device and expires_at only: %+v", seen.Request)
	}
	wantCode(t, laptop, "GET", "/auth/requests/"+r.Request.ID, nil, 401, "unauthenticated")
	wantCode(t, laptop, "POST", "/auth/requests/"+r.Request.ID+"/approve", M{"pin": r.Pin}, 401, "unauthenticated")
	token := phone.NewToken("agent", "agent", "write", true)
	wantCode(t, token, "POST", "/auth/requests/"+r.Request.ID+"/approve", M{"pin": r.Pin}, 403, "forbidden")
	wantCode(t, &testutil.Client{S: s, Cookie: phone.Cookie, NoCSRF: true}, "POST", "/auth/requests/"+r.Request.ID+"/approve", M{"pin": r.Pin}, 403, "csrf")

	phone.Call("POST", "/auth/requests/"+r.Request.ID+"/approve", M{"pin": r.Pin}, http.StatusNoContent, nil)

	// Knowing the id from the QR code is not enough to take the session.
	wantCode(t, laptop, "POST", claim, M{"secret": "not the secret"}, 404, "not_found")
	wantCode(t, laptop, "POST", claim, M{}, 404, "not_found")

	code, _, header := laptop.Do("POST", claim, M{"secret": r.Secret})
	cookie := testutil.SessionCookie(header)
	if code != http.StatusOK || cookie == nil || !cookie.HttpOnly {
		t.Fatalf("claim after approval: %d, cookie %+v", code, cookie)
	}
	laptop.Cookie = cookie
	if whoIs(t, laptop) != "me@example.com" {
		t.Fatal("the laptop is not signed in as the approver")
	}
	// A request signs one device in, once.
	wantCode(t, &testutil.Client{S: s}, "POST", claim, M{"secret": r.Secret}, 404, "not_found")
	wantCode(t, phone, "GET", "/auth/requests/"+r.Request.ID, nil, 404, "not_found")
	wantCode(t, phone, "POST", "/auth/requests/"+r.Request.ID+"/approve", M{"pin": r.Pin}, 404, "not_found")
}

func TestLoginRequestPinGuessesAreLimited(t *testing.T) {
	s, phone := setup(t)
	laptop := &testutil.Client{S: s}
	r := startRequest(t, laptop)
	approve := "/auth/requests/" + r.Request.ID + "/approve"

	wantCode(t, phone, "POST", approve, M{}, 400, "invalid_request")
	wantCode(t, phone, "POST", approve, M{"pin": wrongPin(r)}, 400, "invalid_request")
	// The third wrong pin ends the request, so the right one no longer works.
	wantCode(t, phone, "POST", approve, M{"pin": wrongPin(r)}, 404, "not_found")
	wantCode(t, phone, "POST", approve, M{"pin": r.Pin}, 404, "not_found")
	wantCode(t, laptop, "POST", "/auth/requests/"+r.Request.ID+"/claim", M{"secret": r.Secret}, 404, "not_found")
}

func TestLoginRequestExpiresAndCanBeRefused(t *testing.T) {
	s, phone := setup(t)
	laptop := &testutil.Client{S: s}

	r := startRequest(t, laptop)
	phone.Call("DELETE", "/auth/requests/"+r.Request.ID, nil, http.StatusNoContent, nil)
	wantCode(t, laptop, "POST", "/auth/requests/"+r.Request.ID+"/claim", M{"secret": r.Secret}, 404, "not_found")

	// Approved in time but claimed too late.
	r = startRequest(t, laptop)
	phone.Call("POST", "/auth/requests/"+r.Request.ID+"/approve", M{"pin": r.Pin}, http.StatusNoContent, nil)
	s.SetNow(testutil.Clock.Add(2 * time.Minute))
	wantCode(t, laptop, "POST", "/auth/requests/"+r.Request.ID+"/claim", M{"secret": r.Secret}, 404, "not_found")

	// Not approved in time.
	r = startRequest(t, laptop)
	s.SetNow(s.Now().Add(2 * time.Minute))
	wantCode(t, phone, "GET", "/auth/requests/"+r.Request.ID, nil, 404, "not_found")
	wantCode(t, phone, "POST", "/auth/requests/"+r.Request.ID+"/approve", M{"pin": r.Pin}, 404, "not_found")

	// Changing the password withdraws what the account approved.
	r = startRequest(t, laptop)
	phone.Call("POST", "/auth/requests/"+r.Request.ID+"/approve", M{"pin": r.Pin}, http.StatusNoContent, nil)
	phone.Call("POST", "/me/password", M{"current": "correct horse", "new": "another horse"}, http.StatusNoContent, nil)
	wantCode(t, laptop, "POST", "/auth/requests/"+r.Request.ID+"/claim", M{"secret": r.Secret}, 404, "not_found")
}

type loginCode struct {
	Code      string `json:"code"`
	ExpiresAt string `json:"expires_at"`
}

func TestLoginCodeSignsAnotherDeviceInOnce(t *testing.T) {
	s, laptop := setup(t)
	phone := &testutil.Client{S: s}

	wantCode(t, phone, "POST", "/auth/codes", nil, 401, "unauthenticated")
	wantCode(t, laptop.NewToken("agent", "agent", "write", true), "POST", "/auth/codes", nil, 403, "forbidden")

	var c loginCode
	laptop.Call("POST", "/auth/codes", nil, http.StatusCreated, &c)
	if len(c.Code) < 40 || c.ExpiresAt != "2026-10-13T02:02:00Z" {
		t.Fatalf("unexpected code %+v", c)
	}

	var who struct{ Name, Email string }
	phone.Call("POST", "/auth/codes/check", M{"code": c.Code}, http.StatusOK, &who)
	if who.Email != "me@example.com" || who.Name != "Tester" {
		t.Fatalf("check %+v", who)
	}
	wantCode(t, phone, "POST", "/auth/codes/check", M{"code": "guess"}, 404, "not_found")
	wantCode(t, phone, "POST", "/auth/codes/redeem", M{"code": "guess"}, 404, "not_found")
	wantCode(t, &testutil.Client{S: s, NoCSRF: true}, "POST", "/auth/codes/redeem", M{"code": c.Code}, 403, "csrf")

	status, _, header := phone.Do("POST", "/auth/codes/redeem", M{"code": c.Code})
	if status != http.StatusOK || testutil.SessionCookie(header) == nil {
		t.Fatalf("redeem: %d", status)
	}
	phone.Cookie = testutil.SessionCookie(header)
	if whoIs(t, phone) != "me@example.com" {
		t.Fatal("the phone is not signed in as the issuer")
	}
	wantCode(t, &testutil.Client{S: s}, "POST", "/auth/codes/redeem", M{"code": c.Code}, 404, "not_found")
	wantCode(t, &testutil.Client{S: s}, "POST", "/auth/codes/check", M{"code": c.Code}, 404, "not_found")
}

func TestLoginCodeIsReplacedRevokedAndExpires(t *testing.T) {
	s, laptop := setup(t)
	phone := &testutil.Client{S: s}
	var first, second, third loginCode

	// A new code replaces the one before it.
	laptop.Call("POST", "/auth/codes", nil, http.StatusCreated, &first)
	laptop.Call("POST", "/auth/codes", nil, http.StatusCreated, &second)
	wantCode(t, phone, "POST", "/auth/codes/redeem", M{"code": first.Code}, 404, "not_found")
	phone.Call("POST", "/auth/codes/check", M{"code": second.Code}, http.StatusOK, nil)

	laptop.Call("DELETE", "/auth/codes", nil, http.StatusNoContent, nil)
	wantCode(t, phone, "POST", "/auth/codes/redeem", M{"code": second.Code}, 404, "not_found")

	laptop.Call("POST", "/auth/codes", nil, http.StatusCreated, &third)
	s.SetNow(testutil.Clock.Add(2 * time.Minute))
	wantCode(t, phone, "POST", "/auth/codes/check", M{"code": third.Code}, 404, "not_found")
	wantCode(t, phone, "POST", "/auth/codes/redeem", M{"code": third.Code}, 404, "not_found")
}

func TestLoginCodeEndsTheSessionItReplaces(t *testing.T) {
	s, alice := setup(t)
	bob := s.Register("bob@example.com")
	var c loginCode
	alice.Call("POST", "/auth/codes", nil, http.StatusCreated, &c)

	old := bob.Cookie
	_, _, header := bob.Do("POST", "/auth/codes/redeem", M{"code": c.Code})
	bob.Cookie = testutil.SessionCookie(header)
	if whoIs(t, bob) != "me@example.com" {
		t.Fatal("the device did not switch accounts")
	}
	wantCode(t, &testutil.Client{S: s, Cookie: old}, "GET", "/me", nil, 401, "unauthenticated")
	// One account's code and another's stay apart.
	alice.Call("GET", "/me", nil, http.StatusOK, nil)
}

func TestLoginLinksAreRateLimitedButPollingIsNot(t *testing.T) {
	s := testutil.New(t, testutil.Options{AuthPerMinute: 3})
	anon := &testutil.Client{S: s}
	r := startRequest(t, anon)
	for i := 0; i < 20; i++ {
		anon.Call("POST", "/auth/requests/"+r.Request.ID+"/claim", M{"secret": r.Secret}, http.StatusOK, nil)
	}
	startRequest(t, anon)
	wantCode(t, anon, "POST", "/auth/codes/redeem", M{"code": "guess"}, 404, "not_found")
	wantCode(t, anon, "POST", "/auth/codes/redeem", M{"code": "guess"}, 429, "rate_limited")
	wantCode(t, anon, "POST", "/auth/requests", nil, 429, "rate_limited")
}

func TestPagesCannotBeFramed(t *testing.T) {
	s := testutil.New(t, testutil.Options{})
	for _, path := range []string{"/", "/api/v1/config", "/healthz"} {
		resp, err := http.Get(s.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		h := resp.Header
		if h.Get("X-Frame-Options") != "DENY" || h.Get("Content-Security-Policy") != "frame-ancestors 'none'" || h.Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("%s: missing security headers: %v", path, h)
		}
	}
}

func TestLoginRequestNamesTheDevice(t *testing.T) {
	s, phone := setup(t)
	for agent, want := range map[string]string{
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36":                   "Chrome · Mac",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 19_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/19.0 Mobile/15E148 Safari/604.1": "Safari · iPhone",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36 Edg/140.0.0.0":           "Edge · Windows",
		"Mozilla/5.0 (X11; Linux x86_64; rv:143.0) Gecko/20100101 Firefox/143.0":                                                                  "Firefox · Linux",
		"Tap allow, this is your own phone": "",
	} {
		req, _ := http.NewRequest("POST", s.URL+"/api/v1/auth/requests", nil)
		req.Header.Set("X-Keduly-Request", "1")
		req.Header.Set("User-Agent", agent)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		var r loginRequest
		err = json.NewDecoder(resp.Body).Decode(&r)
		resp.Body.Close()
		if err != nil || resp.StatusCode != http.StatusCreated {
			t.Fatalf("start: %d %v", resp.StatusCode, err)
		}
		var seen struct {
			Request api.LoginRequest `json:"request"`
		}
		phone.Call("GET", "/auth/requests/"+r.Request.ID, nil, http.StatusOK, &seen)
		if seen.Request.Device != want {
			t.Errorf("%s: device %q, want %q", agent, seen.Request.Device, want)
		}
	}
}
