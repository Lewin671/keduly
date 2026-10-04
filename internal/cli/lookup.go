package cli

import (
	"fmt"
	"net/url"
	"slices"
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

// activityID resolves a prefix among the 200 most recent entries.
func (a *app) activityID(prefix string) (string, error) {
	if len(prefix) == idLength {
		return prefix, nil
	}
	recent, err := a.activities(200)
	if err != nil {
		return "", err
	}
	ids := make([]string, 0, len(recent))
	for _, act := range recent {
		ids = append(ids, act.ID)
	}
	return pick("activity", prefix, ids)
}

// isProject reports whether ref names p: its ID, its name or an ID prefix.
func isProject(p api.Project, ref string) bool {
	return p.ID == ref || strings.EqualFold(p.Name, ref) || (len(ref) >= 4 && strings.HasPrefix(p.ID, ref))
}

// findProject resolves a project by exact name, ID or ID prefix.
func findProject(b *api.Bootstrap, ref string) (*api.Project, error) {
	var ids []string
	for i, p := range b.Projects {
		if p.ID == ref || strings.EqualFold(p.Name, ref) {
			return &b.Projects[i], nil
		}
		ids = append(ids, p.ID)
	}
	if len(ref) >= 4 {
		if id, err := pick("project", ref, ids); err == nil {
			return &b.Projects[slices.Index(ids, id)], nil
		}
	}
	return nil, fmt.Errorf("no project is named %q; see `keduly project list --archived`", ref)
}

func (a *app) project(ref string) (*api.Project, error) {
	b, err := a.bootstrap()
	if err != nil {
		return nil, err
	}
	return findProject(b, ref)
}

func (a *app) projectID(ref string) (string, error) {
	p, err := a.project(ref)
	if err != nil {
		return "", err
	}
	return p.ID, nil
}

// findArea resolves an area by exact name, ID or ID prefix; nil when none matches.
func findArea(b *api.Bootstrap, ref string) *api.Area {
	var ids []string
	for i, area := range b.Areas {
		if area.ID == ref || strings.EqualFold(area.Name, ref) {
			return &b.Areas[i]
		}
		ids = append(ids, area.ID)
	}
	if len(ref) >= 4 {
		if id, err := pick("area", ref, ids); err == nil {
			return &b.Areas[slices.Index(ids, id)]
		}
	}
	return nil
}

func (a *app) area(ref string) (*api.Area, error) {
	b, err := a.bootstrap()
	if err != nil {
		return nil, err
	}
	if area := findArea(b, ref); area != nil {
		return area, nil
	}
	return nil, fmt.Errorf("no area is named %q; see `keduly area list`", ref)
}

// findHeading resolves a heading by ID, ID prefix or PROJECT/NAME. A bare name
// is looked up among the headings of projectID, which may be empty.
func findHeading(b *api.Bootstrap, ref, projectID string) (*api.Heading, error) {
	projects := map[string]api.Project{}
	for _, p := range b.Projects {
		projects[p.ID] = p
	}
	var named, pathed []*api.Heading
	var ids []string
	for i := range b.Headings {
		h := &b.Headings[i]
		if h.ID == ref {
			return h, nil
		}
		ids = append(ids, h.ID)
		if h.ProjectID == projectID && strings.EqualFold(h.Name, ref) {
			named = append(named, h)
		}
		if n := len(ref) - len(h.Name) - 1; n > 0 && ref[n] == '/' && strings.EqualFold(ref[n+1:], h.Name) &&
			isProject(projects[h.ProjectID], ref[:n]) {
			pathed = append(pathed, h)
		}
	}
	matches := named
	if len(matches) == 0 {
		matches = pathed
	}
	switch {
	case len(matches) == 1:
		return matches[0], nil
	case len(matches) > 1:
		return nil, fmt.Errorf("%d headings are named %q; give the heading ID, see `keduly heading list %s`",
			len(matches), ref, projects[matches[0].ProjectID].Name)
	}
	if len(ref) >= 4 {
		if id, err := pick("heading", ref, ids); err == nil {
			return &b.Headings[slices.Index(ids, id)], nil
		}
	}
	if projectID == "" && !strings.Contains(ref, "/") {
		return nil, fmt.Errorf("heading %q is a name and the project is not known; give PROJECT/NAME or a heading ID", ref)
	}
	return nil, fmt.Errorf("no heading matches %q; see `keduly heading list PROJECT`", ref)
}

func (a *app) heading(ref, projectID string) (*api.Heading, error) {
	b, err := a.bootstrap()
	if err != nil {
		return nil, err
	}
	return findHeading(b, ref, projectID)
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
