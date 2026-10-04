package core

import (
	"time"

	"github.com/Lewin671/keduly/internal/api"
	"github.com/Lewin671/keduly/internal/store"
)

// Colors lists the project colours in the order the contract names them.
var Colors = []string{"blue", "indigo", "orange", "teal", "green", "pink", "purple", "brown"}

func validColor(c string) bool {
	for _, known := range Colors {
		if c == known {
			return true
		}
	}
	return false
}

func areaJSON(a *store.Area) api.Area { return api.Area{ID: a.ID, Name: a.Name, Position: a.Position} }

func headingJSON(h *store.Heading) api.Heading {
	return api.Heading{ID: h.ID, ProjectID: h.ProjectID, Name: h.Name, Position: h.Position}
}

func (op *Op) nextPosition(table, where string, args ...any) (int, error) {
	var n int
	query := "SELECT coalesce(max(position), -1) + 1 FROM " + table + " WHERE user_id = ?"
	if where != "" {
		query += " AND " + where
	}
	err := op.q.QueryRowContext(op.ctx, query, append([]any{op.User.ID}, args...)...).Scan(&n)
	return n, err
}

func (op *Op) CreateArea(f Fields) (*api.Area, error) {
	a := &store.Area{ID: NewID()}
	r := newReader(f)
	if !r.has("name") {
		r.fail("name is required")
	}
	r.name("name", &a.Name, maxName)
	if err := r.done(); err != nil {
		return nil, err
	}
	n, err := store.Areas.Count(op.ctx, op.q, op.User.ID, "")
	if err != nil {
		return nil, err
	}
	if err := op.checkLimit(n, op.svc.Limits.Projects, "areas"); err != nil {
		return nil, err
	}
	if a.Position, err = op.nextPosition("areas", ""); err != nil {
		return nil, err
	}
	if err := save(op, kindArea, store.Areas, a); err != nil {
		return nil, err
	}
	_, err = op.log("area.create", "新建分组"+quote(a.Name))
	out := areaJSON(a)
	return &out, err
}

func (op *Op) UpdateArea(id string, f Fields) (*api.Area, error) {
	a, err := store.Areas.Get(op.ctx, op.q, op.User.ID, id)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, NotFound("area")
	}
	r := newReader(f)
	r.name("name", &a.Name, maxName)
	r.integer("position", &a.Position)
	if err := r.done(); err != nil {
		return nil, err
	}
	if err := save(op, kindArea, store.Areas, a); err != nil {
		return nil, err
	}
	_, err = op.log("area.update", "修改了分组"+quote(a.Name))
	out := areaJSON(a)
	return &out, err
}

// DeleteArea removes an area; its projects stay and lose their area.
func (op *Op) DeleteArea(id string) error {
	a, err := store.Areas.Get(op.ctx, op.q, op.User.ID, id)
	if err != nil {
		return err
	}
	if a == nil {
		return NotFound("area")
	}
	projects, err := store.Projects.List(op.ctx, op.q, op.User.ID, "area_id = ?", id)
	if err != nil {
		return err
	}
	for _, p := range projects {
		p.AreaID, p.UpdatedAt = nil, op.now()
		if err := save(op, kindProject, store.Projects, p); err != nil {
			return err
		}
	}
	if err := remove(op, kindArea, store.Areas, id); err != nil {
		return err
	}
	_, err = op.log("area.delete", "删除分组"+quote(a.Name))
	return err
}

// Projects lists every project with its item counts.
func (op *Op) Projects() ([]api.Project, error) {
	rows, err := store.Projects.List(op.ctx, op.q, op.User.ID, "")
	if err != nil {
		return nil, err
	}
	counts, err := op.q.QueryContext(op.ctx,
		"SELECT project_id, status, count(*) FROM items WHERE user_id = ? AND project_id IS NOT NULL GROUP BY project_id, status",
		op.User.ID)
	if err != nil {
		return nil, err
	}
	defer counts.Close()
	open, done := map[string]int{}, map[string]int{}
	for counts.Next() {
		var id, status string
		var n int
		if err := counts.Scan(&id, &status, &n); err != nil {
			return nil, err
		}
		if status == "open" {
			open[id] = n
		} else {
			done[id] = n
		}
	}
	out := make([]api.Project, 0, len(rows))
	for _, p := range rows {
		out = append(out, projectJSON(p, open[p.ID], done[p.ID]))
	}
	return out, counts.Err()
}

func projectJSON(p *store.Project, open, done int) api.Project {
	return api.Project{ID: p.ID, AreaID: p.AreaID, Name: p.Name, Color: p.Color, Notes: p.Notes,
		Position: p.Position, Archived: p.Archived, OpenCount: open, DoneCount: done,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}
}

func (op *Op) project(id string) (*store.Project, error) {
	p, err := store.Projects.Get(op.ctx, op.q, op.User.ID, id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, NotFound("project")
	}
	return p, nil
}

func (op *Op) renderProject(p *store.Project) (*api.Project, error) {
	open, err := store.Items.Count(op.ctx, op.q, op.User.ID, "project_id = ? AND status = 'open'", p.ID)
	if err != nil {
		return nil, err
	}
	done, err := store.Items.Count(op.ctx, op.q, op.User.ID, "project_id = ? AND status = 'done'", p.ID)
	if err != nil {
		return nil, err
	}
	out := projectJSON(p, open, done)
	return &out, nil
}

func (op *Op) applyProjectFields(p *store.Project, f Fields, creating bool) error {
	r := newReader(f)
	if creating && !r.has("name") {
		r.fail("name is required")
	}
	r.name("name", &p.Name, maxName)
	r.str("color", &p.Color, 20)
	r.nullStr("area_id", &p.AreaID, anyID, "an area ID")
	r.str("notes", &p.Notes, maxNotes)
	if !creating {
		r.integer("position", &p.Position)
		r.boolean("archived", &p.Archived)
	}
	if err := r.done(); err != nil {
		return err
	}
	if p.Color != "" && !validColor(p.Color) {
		return Invalid("color must be one of blue, indigo, orange, teal, green, pink, purple, brown")
	}
	if p.AreaID != nil && r.has("area_id") {
		a, err := store.Areas.Get(op.ctx, op.q, op.User.ID, *p.AreaID)
		if err != nil {
			return err
		}
		if a == nil {
			return Invalid("area_id does not name one of your areas")
		}
	}
	return nil
}

// leastUsedColor returns the colour the fewest projects use, earliest first on ties.
func (op *Op) leastUsedColor() (string, error) {
	projects, err := store.Projects.List(op.ctx, op.q, op.User.ID, "")
	if err != nil {
		return "", err
	}
	used := map[string]int{}
	for _, p := range projects {
		used[p.Color]++
	}
	best := Colors[0]
	for _, c := range Colors {
		if used[c] < used[best] {
			best = c
		}
	}
	return best, nil
}

func (op *Op) CreateProject(f Fields) (*api.Project, error) {
	n, err := store.Projects.Count(op.ctx, op.q, op.User.ID, "")
	if err != nil {
		return nil, err
	}
	if err := op.checkLimit(n, op.svc.Limits.Projects, "projects"); err != nil {
		return nil, err
	}
	p := &store.Project{ID: NewID(), CreatedAt: op.now(), UpdatedAt: op.now()}
	if err := op.applyProjectFields(p, f, true); err != nil {
		return nil, err
	}
	if p.Color == "" {
		if p.Color, err = op.leastUsedColor(); err != nil {
			return nil, err
		}
	}
	if p.Position, err = op.nextPosition("projects", ""); err != nil {
		return nil, err
	}
	if err := save(op, kindProject, store.Projects, p); err != nil {
		return nil, err
	}
	if _, err := op.log("project.create", "新建项目"+quote(p.Name)); err != nil {
		return nil, err
	}
	return op.renderProject(p)
}

func (op *Op) UpdateProject(id string, f Fields) (*api.Project, error) {
	p, err := op.project(id)
	if err != nil {
		return nil, err
	}
	before := *p
	if err := op.applyProjectFields(p, f, false); err != nil {
		return nil, err
	}
	p.UpdatedAt = op.now()
	if err := save(op, kindProject, store.Projects, p); err != nil {
		return nil, err
	}
	summary := "修改了项目" + quote(p.Name)
	switch {
	case before.Name != p.Name:
		summary = "项目" + quote(before.Name) + "改名为" + quote(p.Name)
	case !before.Archived && p.Archived:
		summary = "归档了项目" + quote(p.Name)
	case before.Archived && !p.Archived:
		summary = "恢复了项目" + quote(p.Name)
	}
	if _, err := op.log("project.update", summary); err != nil {
		return nil, err
	}
	return op.renderProject(p)
}

// DeleteProject removes a project together with its headings, items and events.
// No suggestion kind stands in for it, so a token whose deletions need the
// user's confirmation is refused outright.
func (op *Op) DeleteProject(id string) error {
	p, err := op.project(id)
	if err != nil {
		return err
	}
	if op.needsConfirmation() {
		return Forbidden("this token's deletions need the user's confirmation, and deleting a project " +
			"(with all its items and events) cannot be proposed: the user must delete it in the web app")
	}
	items, err := store.Items.List(op.ctx, op.q, op.User.ID, "project_id = ?", id)
	if err != nil {
		return err
	}
	for _, it := range items {
		if _, err := op.deleteItem(it); err != nil {
			return err
		}
	}
	events, err := store.Events.List(op.ctx, op.q, op.User.ID, "project_id = ?", id)
	if err != nil {
		return err
	}
	for _, ev := range events {
		if err := op.removeEvent(ev); err != nil {
			return err
		}
	}
	headings, err := store.Headings.List(op.ctx, op.q, op.User.ID, "project_id = ?", id)
	if err != nil {
		return err
	}
	for _, h := range headings {
		if err := remove(op, kindHeading, store.Headings, h.ID); err != nil {
			return err
		}
	}
	if err := remove(op, kindProject, store.Projects, id); err != nil {
		return err
	}
	_, err = op.log("project.delete", "删除项目"+quote(p.Name))
	return err
}

func (op *Op) headings(projectID string) ([]api.Heading, error) {
	rows, err := store.Headings.List(op.ctx, op.q, op.User.ID, "project_id = ?", projectID)
	if err != nil {
		return nil, err
	}
	out := make([]api.Heading, 0, len(rows))
	for _, h := range rows {
		out = append(out, headingJSON(h))
	}
	return out, nil
}

// ProjectDetail assembles what the project screen shows.
func (op *Op) ProjectDetail(id string) (*api.ProjectDetail, error) {
	p, err := op.project(id)
	if err != nil {
		return nil, err
	}
	project, err := op.renderProject(p)
	if err != nil {
		return nil, err
	}
	out := &api.ProjectDetail{Project: *project, UpcomingEvents: []api.Event{}}
	if out.Headings, err = op.headings(id); err != nil {
		return nil, err
	}
	entries, err := op.entries(op.Now, op.Now.Add(366*24*time.Hour), true)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		mine := e.ev.ProjectID != nil && *e.ev.ProjectID == id
		if mine && realEvents(e) && e.end.After(op.Now) && len(out.UpcomingEvents) < 3 {
			out.UpcomingEvents = append(out.UpcomingEvents, e.ev)
		}
	}
	open, err := store.Items.List(op.ctx, op.q, op.User.ID, "project_id = ? AND status = 'open'", id)
	if err != nil {
		return nil, err
	}
	for _, it := range open {
		planned, err := op.planned(it.ID)
		if err != nil {
			return nil, err
		}
		if !planned {
			out.UnplannedCount++
		}
	}
	return out, nil
}

func (op *Op) CreateHeading(projectID string, f Fields) (*api.Heading, error) {
	p, err := op.project(projectID)
	if err != nil {
		return nil, err
	}
	h := &store.Heading{ID: NewID(), ProjectID: p.ID}
	r := newReader(f)
	if !r.has("name") {
		r.fail("name is required")
	}
	r.name("name", &h.Name, maxName)
	if err := r.done(); err != nil {
		return nil, err
	}
	n, err := store.Headings.Count(op.ctx, op.q, op.User.ID, "project_id = ?", p.ID)
	if err != nil {
		return nil, err
	}
	if err := op.checkLimit(n, 200, "headings per project"); err != nil {
		return nil, err
	}
	if h.Position, err = op.nextPosition("headings", "project_id = ?", p.ID); err != nil {
		return nil, err
	}
	if err := save(op, kindHeading, store.Headings, h); err != nil {
		return nil, err
	}
	_, err = op.log("heading.create", "在项目"+quote(p.Name)+"中新建分节"+quote(h.Name))
	out := headingJSON(h)
	return &out, err
}

func (op *Op) heading(id string) (*store.Heading, error) {
	h, err := store.Headings.Get(op.ctx, op.q, op.User.ID, id)
	if err != nil {
		return nil, err
	}
	if h == nil {
		return nil, NotFound("heading")
	}
	return h, nil
}

func (op *Op) UpdateHeading(id string, f Fields) (*api.Heading, error) {
	h, err := op.heading(id)
	if err != nil {
		return nil, err
	}
	r := newReader(f)
	r.name("name", &h.Name, maxName)
	r.integer("position", &h.Position)
	if err := r.done(); err != nil {
		return nil, err
	}
	if err := save(op, kindHeading, store.Headings, h); err != nil {
		return nil, err
	}
	_, err = op.log("heading.update", "修改了分节"+quote(h.Name))
	out := headingJSON(h)
	return &out, err
}

// DeleteHeading removes a heading; its items stay in the project.
func (op *Op) DeleteHeading(id string) error {
	h, err := op.heading(id)
	if err != nil {
		return err
	}
	items, err := store.Items.List(op.ctx, op.q, op.User.ID, "heading_id = ?", id)
	if err != nil {
		return err
	}
	for _, it := range items {
		it.HeadingID, it.UpdatedAt = nil, op.now()
		if err := save(op, kindItem, store.Items, it); err != nil {
			return err
		}
	}
	if err := remove(op, kindHeading, store.Headings, id); err != nil {
		return err
	}
	_, err = op.log("heading.delete", "删除分节"+quote(h.Name))
	return err
}
