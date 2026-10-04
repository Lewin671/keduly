package httpapi_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Lewin671/keduly/internal/api"
	"github.com/Lewin671/keduly/internal/testutil"
)

func TestRegisterLoginAndCookie(t *testing.T) {
	s := testutil.New(t, testutil.Options{})
	anon := &testutil.Client{S: s}

	status, _, header := anon.Do("POST", "/auth/register", M{"email": "Me@Example.com", "password": "correct horse", "name": "Me", "timezone": "Asia/Shanghai"})
	if status != http.StatusCreated {
		t.Fatalf("register: %d", status)
	}
	cookie := testutil.SessionCookie(header)
	if cookie == nil || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Secure {
		t.Fatalf("unexpected cookie %+v", cookie)
	}
	if cookie.MaxAge != 30*24*3600 {
		t.Fatalf("cookie max age %d", cookie.MaxAge)
	}

	me := &testutil.Client{S: s, Cookie: cookie}
	var got struct {
		User  api.User  `json:"user"`
		Actor api.Actor `json:"actor"`
	}
	me.Call("GET", "/me", nil, http.StatusOK, &got)
	if got.User.Email != "me@example.com" || got.User.Timezone != "Asia/Shanghai" || got.User.WorkStart != "09:00" || len(got.User.ID) != 16 {
		t.Fatalf("unexpected user %+v", got.User)
	}
	if got.Actor.Kind != "user" {
		t.Fatalf("actor %+v", got.Actor)
	}

	wantCode(t, anon, "POST", "/auth/register", M{"email": "me@example.com", "password": "correct horse", "name": "Me", "timezone": "UTC"}, 409, "conflict")
	wantCode(t, anon, "POST", "/auth/register", M{"email": "b@example.com", "password": "short", "name": "B", "timezone": "UTC"}, 400, "invalid_request")
	wantCode(t, anon, "POST", "/auth/register", M{"email": "b@example.com", "password": "correct horse", "name": "B", "timezone": "Mars/Base"}, 400, "invalid_request")
	wantCode(t, anon, "POST", "/auth/login", M{"email": "me@example.com", "password": "wrong password"}, 401, "unauthenticated")
	wantCode(t, anon, "POST", "/auth/login", M{"email": "nobody@example.com", "password": "correct horse"}, 401, "unauthenticated")
	wantCode(t, anon, "GET", "/me", nil, 401, "unauthenticated")

	status, _, header = anon.Do("POST", "/auth/login", M{"email": "me@example.com", "password": "correct horse"})
	if status != http.StatusOK || testutil.SessionCookie(header) == nil {
		t.Fatalf("login: %d", status)
	}
	second := &testutil.Client{S: s, Cookie: testutil.SessionCookie(header)}

	// Changing the password ends every other session.
	wantCode(t, me, "POST", "/me/password", M{"current": "nope nope", "new": "another horse"}, 400, "invalid_request")
	me.Call("POST", "/me/password", M{"current": "correct horse", "new": "another horse"}, http.StatusNoContent, nil)
	me.Call("GET", "/me", nil, http.StatusOK, nil)
	wantCode(t, second, "GET", "/me", nil, 401, "unauthenticated")
	anon.Call("POST", "/auth/login", M{"email": "me@example.com", "password": "another horse"}, http.StatusOK, nil)

	me.Call("POST", "/auth/logout", nil, http.StatusNoContent, nil)
	wantCode(t, me, "GET", "/me", nil, 401, "unauthenticated")
}

func TestSecureCookieBehindHTTPSProxy(t *testing.T) {
	s := testutil.New(t, testutil.Options{})
	req, _ := http.NewRequest("POST", s.URL+"/api/v1/auth/register",
		strings.NewReader(`{"email":"me@example.com","password":"correct horse","name":"Me","timezone":"UTC"}`))
	req.Header.Set("X-Keduly-Request", "1")
	req.Header.Set("X-Forwarded-Proto", "https")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if c := testutil.SessionCookie(resp.Header); c == nil || !c.Secure {
		t.Fatalf("cookie should be Secure behind an HTTPS proxy: %+v", c)
	}
}

func TestSessionSlidingExpiry(t *testing.T) {
	s, me := setup(t)
	s.SetNow(testutil.Clock.Add(20 * 24 * time.Hour))
	status, _, header := me.Do("GET", "/me", nil)
	if status != http.StatusOK || testutil.SessionCookie(header) == nil {
		t.Fatalf("an active session should be renewed: %d", status)
	}
	// 45 days after sign-up, but only 25 after the last use.
	s.SetNow(testutil.Clock.Add(45 * 24 * time.Hour))
	me.Call("GET", "/me", nil, http.StatusOK, nil)
	s.SetNow(testutil.Clock.Add(80 * 24 * time.Hour))
	wantCode(t, me, "GET", "/me", nil, 401, "unauthenticated")
}

func TestRegistrationClosed(t *testing.T) {
	s := testutil.New(t, testutil.Options{Registration: "closed"})
	anon := &testutil.Client{S: s}
	wantCode(t, anon, "POST", "/auth/register", M{"email": "me@example.com", "password": "correct horse", "name": "Me", "timezone": "UTC"}, 403, "registration_closed")
	var cfg struct {
		Registration string `json:"registration"`
		Version      string `json:"version"`
	}
	anon.Call("GET", "/config", nil, http.StatusOK, &cfg)
	if cfg.Registration != "closed" || cfg.Version == "" {
		t.Fatalf("config %+v", cfg)
	}
}

func TestCSRF(t *testing.T) {
	s, me := setup(t)
	bare := &testutil.Client{S: s, Cookie: me.Cookie, NoCSRF: true}
	wantCode(t, bare, "POST", "/items", M{"title": "x"}, 403, "csrf")
	wantCode(t, bare, "POST", "/auth/login", M{"email": "me@example.com", "password": "correct horse"}, 403, "csrf")
	bare.Call("GET", "/me", nil, http.StatusOK, nil)
	// A bearer token is not sent by browsers on their own, so it needs no header.
	token := me.NewToken("agent", "agent", "write", true)
	token.NoCSRF = true
	token.Call("POST", "/items", M{"title": "x"}, http.StatusCreated, nil)
}

func TestTokenScopes(t *testing.T) {
	_, me := setup(t)
	reader := me.NewToken("Morning brief", "agent", "read", true)
	writer := me.NewToken("Claude Code · MacBook", "agent", "write", true)
	calendar := me.NewToken("Phone", "caldav", "write", true)

	reader.Call("GET", "/items", nil, http.StatusOK, nil)
	wantCode(t, reader, "POST", "/items", M{"title": "x"}, 403, "forbidden")
	item := mkItem(t, writer, M{"title": "from the agent"})
	if item.CreatedBy.Kind != "agent" || item.CreatedBy.Name != "Claude Code · MacBook" {
		t.Fatalf("created_by %+v", item.CreatedBy)
	}
	var who struct {
		Actor api.Actor `json:"actor"`
	}
	writer.Call("GET", "/me", nil, http.StatusOK, &who)
	if who.Actor.Kind != "agent" || who.Actor.Name != "Claude Code · MacBook" {
		t.Fatalf("actor %+v", who.Actor)
	}

	// Account, token and session management always need the session cookie.
	wantCode(t, writer, "GET", "/tokens", nil, 403, "forbidden")
	wantCode(t, writer, "POST", "/tokens", M{"name": "x", "kind": "agent"}, 403, "forbidden")
	wantCode(t, writer, "PATCH", "/me", M{"name": "x"}, 403, "forbidden")
	wantCode(t, writer, "POST", "/me/password", M{"current": "correct horse", "new": "another horse"}, 403, "forbidden")
	// An app password is for CalDAV only.
	wantCode(t, calendar, "GET", "/items", nil, 401, "unauthenticated")
	wantCode(t, &testutil.Client{S: me.S, Token: "kdl_wrong"}, "GET", "/items", nil, 401, "unauthenticated")

	var list struct {
		Tokens []api.Token `json:"tokens"`
	}
	me.Call("GET", "/tokens", nil, http.StatusOK, &list)
	if len(list.Tokens) != 3 || list.Tokens[0].Token != "" || list.Tokens[0].Scope != "read" || list.Tokens[1].LastUsedAt == nil {
		t.Fatalf("tokens %+v", list.Tokens)
	}
	me.Call("DELETE", "/tokens/"+list.Tokens[1].ID, nil, http.StatusNoContent, nil)
	wantCode(t, writer, "GET", "/items", nil, 401, "unauthenticated")
	wantCode(t, me, "POST", "/tokens", M{"name": "x", "kind": "robot"}, 400, "invalid_request")
}

func TestRequestValidation(t *testing.T) {
	_, me := setup(t)
	wantCode(t, me, "POST", "/items", M{"title": "x", "colour": "red"}, 400, "invalid_request")
	wantCode(t, me, "POST", "/items", `{"title":`, 400, "invalid_request")
	wantCode(t, me, "POST", "/items", M{"title": "  "}, 400, "invalid_request")
	wantCode(t, me, "POST", "/items", M{"title": strings.Repeat("长", 501)}, 400, "invalid_request")
	wantCode(t, me, "POST", "/items", M{"title": "x", "notes": strings.Repeat("n", 20001)}, 400, "invalid_request")
	wantCode(t, me, "POST", "/items", M{"title": "x", "due_time": "18:00"}, 400, "invalid_request")
	wantCode(t, me, "POST", "/items", M{"title": "x", "due_date": "2026-13-40"}, 400, "invalid_request")
	wantCode(t, me, "POST", "/items", M{"title": "x", "project_id": "nosuchproject0000"}, 400, "invalid_request")
	wantCode(t, me, "POST", "/items", M{"title": "x", "notes": strings.Repeat("n", 1<<20)}, 400, "invalid_request")
	wantCode(t, me, "GET", "/nope", nil, 404, "not_found")
	wantCode(t, me, "PUT", "/items", M{}, 404, "not_found")
	wantCode(t, me, "GET", "/items/aaaaaaaaaaaaaaaa", nil, 404, "not_found")
	mkItem(t, me, M{"title": strings.Repeat("长", 500)})
}

func TestRateLimit(t *testing.T) {
	s := testutil.New(t, testutil.Options{AuthPerMinute: 3})
	anon := &testutil.Client{S: s}
	for i := 0; i < 3; i++ {
		wantCode(t, anon, "POST", "/auth/login", M{"email": "me@example.com", "password": "wrong password"}, 401, "unauthenticated")
	}
	wantCode(t, anon, "POST", "/auth/login", M{"email": "me@example.com", "password": "wrong password"}, 429, "rate_limited")
	// The rest of the API has its own, larger budget.
	anon.Call("GET", "/config", nil, http.StatusOK, nil)

	// Behind a proxy on a private address, each forwarded client has its own budget.
	req, _ := http.NewRequest("POST", s.URL+"/api/v1/auth/login", strings.NewReader(`{"email":"me@example.com","password":"wrong password"}`))
	req.Header.Set("X-Keduly-Request", "1")
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("a different forwarded client should not be limited: %d", resp.StatusCode)
	}
}

func TestIsolationBetweenUsers(t *testing.T) {
	s, alice := setup(t)
	bob := s.Register("bob@example.com")

	project := mkProject(t, alice, "Alice's project")
	item := mkItem(t, alice, M{"title": "Alice's item", "project_id": project.ID})
	event := mkEvent(t, alice, M{"title": "Alice's event", "start": "2026-10-13T02:00:00Z", "end": "2026-10-13T03:00:00Z"})
	sug := suggest(t, alice, M{"kind": "delete_item", "item_id": item.ID, "reason": "test"})
	act := activities(t, alice)[0]

	wantCode(t, bob, "GET", "/items/"+item.ID, nil, 404, "not_found")
	wantCode(t, bob, "PATCH", "/items/"+item.ID, M{"title": "mine now"}, 404, "not_found")
	wantCode(t, bob, "DELETE", "/items/"+item.ID, nil, 404, "not_found")
	wantCode(t, bob, "POST", "/items/"+item.ID+"/schedule", M{"start": "2026-10-13T02:00:00Z", "end": "2026-10-13T03:00:00Z"}, 404, "not_found")
	wantCode(t, bob, "GET", "/events/"+event.ID, nil, 404, "not_found")
	wantCode(t, bob, "PATCH", "/events/"+event.ID, M{"title": "mine now"}, 404, "not_found")
	wantCode(t, bob, "DELETE", "/events/"+event.ID, nil, 404, "not_found")
	wantCode(t, bob, "GET", "/projects/"+project.ID, nil, 404, "not_found")
	wantCode(t, bob, "DELETE", "/projects/"+project.ID, nil, 404, "not_found")
	wantCode(t, bob, "POST", "/projects/"+project.ID+"/headings", M{"name": "x"}, 404, "not_found")
	wantCode(t, bob, "POST", "/suggestions/"+sug.ID+"/accept", nil, 404, "not_found")
	wantCode(t, bob, "POST", "/activity/"+act.ID+"/undo", nil, 404, "not_found")
	wantCode(t, bob, "POST", "/activity/undo", M{"ids": []string{act.ID}}, 404, "not_found")
	wantCode(t, bob, "POST", "/items", M{"title": "x", "project_id": project.ID}, 400, "invalid_request")
	wantCode(t, bob, "POST", "/suggestions", M{"kind": "schedule_item", "item_id": item.ID, "start": "2026-10-13T02:00:00Z", "end": "2026-10-13T03:00:00Z", "reason": "x"}, 404, "not_found")

	var list struct {
		Items []api.Item `json:"items"`
		Total int        `json:"total"`
	}
	bob.Call("GET", "/items?status=any", nil, http.StatusOK, &list)
	if list.Total != 0 || len(listEvents(t, bob, dayUTC, endUTC)) != 0 || len(activities(t, bob)) != 0 {
		t.Fatal("bob sees alice's data")
	}
	if getItem(t, alice, item.ID).Title != "Alice's item" {
		t.Fatal("alice's item changed")
	}
}

func TestTimeZoneFollowsDeviceUntilChosen(t *testing.T) {
	s := testutil.New(t, testutil.Options{})
	anon := &testutil.Client{S: s}
	_, _, header := anon.Do("POST", "/auth/register", M{"email": "zone@example.com", "password": "correct horse", "timezone": "America/Los_Angeles"})
	me := &testutil.Client{S: s, Cookie: testutil.SessionCookie(header)}

	var got struct {
		User api.User `json:"user"`
	}
	me.Call("GET", "/me", nil, http.StatusOK, &got)
	if !got.User.TimezoneAuto || got.User.Timezone != "America/Los_Angeles" {
		t.Fatalf("a new account should follow the device: %+v", got.User)
	}

	me.Call("PATCH", "/me", M{"timezone": "Asia/Shanghai", "timezone_auto": false}, http.StatusOK, &got)
	if got.User.TimezoneAuto || got.User.Timezone != "Asia/Shanghai" {
		t.Fatalf("the chosen zone was not kept: %+v", got.User)
	}
	me.Call("GET", "/me", nil, http.StatusOK, &got)
	if got.User.TimezoneAuto || got.User.Timezone != "Asia/Shanghai" {
		t.Fatalf("the chosen zone did not persist: %+v", got.User)
	}
	wantCode(t, me, "PATCH", "/me", M{"timezone": "Mars/Olympus"}, 400, "invalid_request")
}
