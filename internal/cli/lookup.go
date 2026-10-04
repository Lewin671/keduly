package cli

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Lewin671/keduly/internal/api"
)

const idLength = 16

// pick returns the one candidate that starts with prefix.
func pick(what, prefix string, ids []string) (string, error) {
	if len(prefix) < 4 {
		return "", usagef("%s ID %q is too short; give at least 4 characters", what, prefix)
	}
	var matches []string
	for _, id := range ids {
		if strings.HasPrefix(id, prefix) {
			matches = append(matches, id)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no %s has an ID starting with %q", what, prefix)
	case 1:
		return matches[0], nil
	}
	return "", fmt.Errorf("%d %ss have an ID starting with %q; give more characters", len(matches), what, prefix)
}

func (a *app) itemID(prefix string) (string, error) {
	if len(prefix) == idLength {
		return prefix, nil
	}
	var ids []string
	cursor := ""
	for page := 0; page < 200; page++ {
		var resp struct {
			Items      []api.Item `json:"items"`
			NextCursor *string    `json:"next_cursor"`
		}
		q := url.Values{"status": {"any"}, "limit": {"200"}}
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		if err := a.get("/items", q, &resp); err != nil {
			return "", err
		}
		for _, it := range resp.Items {
			ids = append(ids, it.ID)
		}
		if resp.NextCursor == nil {
			break
		}
		cursor = *resp.NextCursor
	}
	return pick("item", prefix, ids)
}

// eventID resolves a prefix among events within 200 days of now.
func (a *app) eventID(prefix string) (string, error) {
	if len(prefix) == idLength {
		return prefix, nil
	}
	now := a.env.Now()
	events, err := a.events(now.AddDate(0, 0, -200), now.AddDate(0, 0, 200))
	if err != nil {
		return "", err
	}
	seen := map[string]bool{}
	var ids []string
	for _, e := range events {
		if e.Status != "tentative" && !seen[e.ID] {
			seen[e.ID] = true
			ids = append(ids, e.ID)
		}
	}
	return pick("event", prefix, ids)
}

func (a *app) events(from, to time.Time) ([]api.Event, error) {
	var resp struct {
		Events []api.Event `json:"events"`
	}
	err := a.get("/events", url.Values{"from": {stamp(from)}, "to": {stamp(to)}}, &resp)
	return resp.Events, err
}

func (a *app) activities(limit int) ([]api.Activity, error) {
	var resp struct {
		Activities []api.Activity `json:"activities"`
	}
	err := a.get("/activity", url.Values{"limit": {strconv.Itoa(limit)}}, &resp)
	return resp.Activities, err
}

func (a *app) bootstrap() (*api.Bootstrap, error) {
	var b api.Bootstrap
	return &b, a.get("/bootstrap", nil, &b)
}

// projectID resolves a project by exact name, ID or ID prefix.
func (a *app) projectID(ref string) (string, error) {
	b, err := a.bootstrap()
	if err != nil {
		return "", err
	}
	var ids []string
	for _, p := range b.Projects {
		if p.ID == ref || strings.EqualFold(p.Name, ref) {
			return p.ID, nil
		}
		ids = append(ids, p.ID)
	}
	if len(ref) >= 4 {
		if id, err := pick("project", ref, ids); err == nil {
			return id, nil
		}
	}
	return "", fmt.Errorf("no project is named %q; see `keduly project list`", ref)
}

// projectNames maps project IDs to names for display.
func (a *app) projectNames() map[string]string {
	names := map[string]string{}
	if b, err := a.bootstrap(); err == nil {
		for _, p := range b.Projects {
			names[p.ID] = p.Name
		}
	}
	return names
}
