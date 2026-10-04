package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/Lewin671/keduly/internal/api"
	"github.com/Lewin671/keduly/internal/testutil"
)

type M = map[string]any

func setup(t *testing.T) (*testutil.Server, *testutil.Client) {
	t.Helper()
	s := testutil.New(t, testutil.Options{})
	return s, s.Register("me@example.com")
}

func mkItem(t *testing.T, c *testutil.Client, body M) api.Item {
	t.Helper()
	var resp struct {
		Item api.Item `json:"item"`
	}
	c.Call("POST", "/items", body, http.StatusCreated, &resp)
	return resp.Item
}

func getItem(t *testing.T, c *testutil.Client, id string) api.Item {
	t.Helper()
	var resp struct {
		Item api.Item `json:"item"`
	}
	c.Call("GET", "/items/"+id, nil, http.StatusOK, &resp)
	return resp.Item
}

func mkEvent(t *testing.T, c *testutil.Client, body M) api.Event {
	t.Helper()
	var resp struct {
		Event api.Event `json:"event"`
	}
	c.Call("POST", "/events", body, http.StatusCreated, &resp)
	return resp.Event
}

func mkProject(t *testing.T, c *testutil.Client, name string) api.Project {
	t.Helper()
	var resp struct {
		Project api.Project `json:"project"`
	}
	c.Call("POST", "/projects", M{"name": name}, http.StatusCreated, &resp)
	return resp.Project
}

func listEvents(t *testing.T, c *testutil.Client, from, to string) []api.Event {
	t.Helper()
	var resp struct {
		Events []api.Event `json:"events"`
	}
	c.Call("GET", "/events?from="+from+"&to="+to, nil, http.StatusOK, &resp)
	return resp.Events
}

func suggest(t *testing.T, c *testutil.Client, body M) api.Suggestion {
	t.Helper()
	var resp struct {
		Suggestion api.Suggestion `json:"suggestion"`
	}
	c.Call("POST", "/suggestions", body, http.StatusCreated, &resp)
	return resp.Suggestion
}

func activities(t *testing.T, c *testutil.Client) []api.Activity {
	t.Helper()
	var resp struct {
		Activities []api.Activity `json:"activities"`
	}
	c.Call("GET", "/activity?limit=100", nil, http.StatusOK, &resp)
	return resp.Activities
}

func revision(t *testing.T, c *testutil.Client) int64 {
	t.Helper()
	var resp struct {
		Counts   api.Counts `json:"counts"`
		Revision int64      `json:"revision"`
	}
	c.Call("GET", "/counts", nil, http.StatusOK, &resp)
	return resp.Revision
}

func wantCode(t *testing.T, c *testutil.Client, method, path string, body any, status int, code string) {
	t.Helper()
	gotStatus, gotCode := c.ErrorCode(method, path, body)
	if gotStatus != status || gotCode != code {
		t.Fatalf("%s %s: got %d %q, want %d %q", method, path, gotStatus, gotCode, status, code)
	}
}

func titles(items []api.Item) []string {
	out := []string{}
	for _, it := range items {
		out = append(out, it.Title)
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

const (
	day    = "2026-10-13"
	dayUTC = "2026-10-12T16:00:00Z" // midnight in Asia/Shanghai
	endUTC = "2026-10-13T16:00:00Z"
)
