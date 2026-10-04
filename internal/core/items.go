package core

import (
	"strings"
	"time"

	"github.com/Lewin671/keduly/internal/api"
	"github.com/Lewin671/keduly/internal/store"
)

// decorations are the computed parts of items: time blocks and pending
// scheduling suggestions, loaded once per Op.
type decorations struct {
	blocks map[string][]*store.Event
	sugs   map[string]*store.Suggestion
}

func (op *Op) decorate() (*decorations, error) {
	if op.deco != nil {
		return op.deco, nil
	}
	d := &decorations{blocks: map[string][]*store.Event{}, sugs: map[string]*store.Suggestion{}}
	blocks, err := store.Events.List(op.ctx, op.q, op.User.ID, "item_id IS NOT NULL AND all_day = 0")
	if err != nil {
		return nil, err
	}
	for _, b := range blocks {
		d.blocks[*b.ItemID] = append(d.blocks[*b.ItemID], b)
	}
	sugs, err := store.Suggestions.List(op.ctx, op.q, op.User.ID, "status = 'pending' AND kind = 'schedule_item'")
	if err != nil {
		return nil, err
	}
	for _, s := range sugs {
		if _, ok := d.sugs[*s.ItemID]; !ok {
			d.sugs[*s.ItemID] = s
		}
	}
	op.deco = d
	return d, nil
}

// currentBlock picks the next block that has not ended, or else the most recent one.
func (op *Op) currentBlock(blocks []*store.Event) *store.Event {
	var next, last *store.Event
	now := op.now()
	for _, b := range blocks {
		if *b.EndAt > now {
			if next == nil || *b.StartAt < *next.StartAt {
				next = b
			}
		} else if last == nil || *b.StartAt > *last.StartAt {
			last = b
		}
	}
	if next != nil {
		return next
	}
	return last
}

func (op *Op) renderItem(it *store.Item) (api.Item, error) {
	d, err := op.decorate()
	if err != nil {
		return api.Item{}, err
	}
	out := api.Item{
		ID: it.ID, ProjectID: it.ProjectID, HeadingID: it.HeadingID, Title: it.Title, Notes: it.Notes,
		EstimateMinutes: it.EstimateMinutes, PlannedDate: it.PlannedDate, Evening: it.Evening,
		DueDate: it.DueDate, DueTime: it.DueTime, Important: it.Important,
		Status: it.Status, CompletedAt: it.CompletedAt, Position: it.Position,
		CreatedBy: api.Actor{Kind: it.CreatedByKind, Name: it.CreatedByName},
		CreatedAt: it.CreatedAt, UpdatedAt: it.UpdatedAt,
	}
	if b := op.currentBlock(d.blocks[it.ID]); b != nil {
		out.Block = &api.Block{EventID: b.ID, Start: *b.StartAt, End: *b.EndAt}
	}
	if s := d.sugs[it.ID]; s != nil {
		out.Suggestion = &api.ItemSuggestion{ID: s.ID, Start: *s.StartAt, End: *s.EndAt, Reason: s.Reason,
			Actor: api.Actor{Kind: s.ActorKind, Name: s.ActorName}}
	}
	return out, nil
}

func (op *Op) renderItems(rows []*store.Item) ([]api.Item, error) {
	out := make([]api.Item, 0, len(rows))
	for _, it := range rows {
		item, err := op.renderItem(it)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// planned reports whether an item has a time block or a pending suggestion.
func (op *Op) planned(id string) (bool, error) {
	d, err := op.decorate()
	if err != nil {
		return false, err
	}
	return len(d.blocks[id]) > 0 || d.sugs[id] != nil, nil
}

func (op *Op) item(id string) (*store.Item, error) {
	it, err := store.Items.Get(op.ctx, op.q, op.User.ID, id)
	if err != nil {
		return nil, err
	}
	if it == nil {
		return nil, NotFound("item")
	}
	return it, nil
}

func (op *Op) GetItem(id string) (*api.Item, error) {
	it, err := op.item(id)
	if err != nil {
		return nil, err
	}
	out, err := op.renderItem(it)
	return &out, err
}

// applyItemFields validates input and copies it onto the row.
func (op *Op) applyItemFields(it *store.Item, f Fields, creating bool) error {
	r := newReader(f)
	oldProject := it.ProjectID
	if creating && !r.has("title") {
		r.fail("title is required")
	}
	r.name("title", &it.Title, maxTitle)
	r.str("notes", &it.Notes, maxNotes)
	r.nullStr("project_id", &it.ProjectID, anyID, "a project ID")
	r.nullStr("heading_id", &it.HeadingID, anyID, "a heading ID")
	r.nullInt("estimate_minutes", &it.EstimateMinutes, 1, 100000)
	r.date("planned_date", &it.PlannedDate)
	r.boolean("evening", &it.Evening)
	r.date("due_date", &it.DueDate)
	r.nullStr("due_time", &it.DueTime, validClock, "a time of day (HH:MM)")
	r.boolean("important", &it.Important)
	r.integer("position", &it.Position)
	status := it.Status
	if !creating {
		r.str("status", &status, 10)
	}
	if err := r.done(); err != nil {
		return err
	}
	if status != "open" && status != "done" {
		return Invalid("status must be open or done")
	}
	if status != it.Status {
		it.Status, it.CompletedAt = status, nil
		if status == "done" {
			now := op.now()
			it.CompletedAt = &now
		}
	}
	if it.DueTime != nil && it.DueDate == nil {
		return Invalid("due_time requires due_date")
	}
	return op.checkItemPlace(it, oldProject, r)
}

// checkItemPlace validates the project and heading and keeps them consistent.
func (op *Op) checkItemPlace(it *store.Item, oldProject *string, r *reader) error {
	if it.HeadingID != nil {
		h, err := store.Headings.Get(op.ctx, op.q, op.User.ID, *it.HeadingID)
		if err != nil {
			return err
		}
		switch {
		case h == nil:
			return Invalid("heading_id does not name one of your headings")
		case !r.has("project_id") && r.has("heading_id"):
			it.ProjectID = &h.ProjectID
		case it.ProjectID == nil || *it.ProjectID != h.ProjectID:
			if r.has("heading_id") {
				return Invalid("heading_id belongs to a different project")
			}
			// The item moved to another project: it leaves its old heading.
			it.HeadingID = nil
		}
	}
	if it.ProjectID != nil && !eqStr(it.ProjectID, oldProject) {
		p, err := store.Projects.Get(op.ctx, op.q, op.User.ID, *it.ProjectID)
		if err != nil {
			return err
		}
		if p == nil {
			return Invalid("project_id does not name one of your projects")
		}
	}
	return nil
}

// createItem creates an item without logging; it returns the summary to log.
func (op *Op) createItem(f Fields) (*store.Item, string, error) {
	n, err := store.Items.Count(op.ctx, op.q, op.User.ID, "")
	if err != nil {
		return nil, "", err
	}
	if err := op.checkLimit(n, op.svc.Limits.Items, "items"); err != nil {
		return nil, "", err
	}
	it := &store.Item{
		ID: NewID(), UserID: op.User.ID, Status: "open",
		CreatedByKind: op.ID.Actor.Kind, CreatedByName: op.ID.Actor.Name,
		CreatedAt: op.now(), UpdatedAt: op.now(),
	}
	if err := op.applyItemFields(it, f, true); err != nil {
		return nil, "", err
	}
	if err := save(op, kindItem, store.Items, it); err != nil {
		return nil, "", err
	}
	return it, "新建事项" + quote(it.Title), nil
}

func (op *Op) CreateItem(f Fields) (*api.Item, error) {
	it, summary, err := op.createItem(f)
	if err != nil {
		return nil, err
	}
	if _, err := op.log("item.create", summary); err != nil {
		return nil, err
	}
	out, err := op.renderItem(it)
	return &out, err
}

func (op *Op) UpdateItem(id string, f Fields) (*api.Item, error) {
	it, err := op.item(id)
	if err != nil {
		return nil, err
	}
	before := *it
	if err := op.applyItemFields(it, f, false); err != nil {
		return nil, err
	}
	it.UpdatedAt = op.now()
	if err := save(op, kindItem, store.Items, it); err != nil {
		return nil, err
	}
	if before.Title != it.Title || !eqStr(before.ProjectID, it.ProjectID) {
		if err := op.syncBlocks(it); err != nil {
			return nil, err
		}
	}
	action, summary := "item.update", ""
	switch {
	case before.Status != it.Status && it.Status == "done":
		action, summary = "item.complete", "完成了"+quote(it.Title)
	case before.Status != it.Status:
		action, summary = "item.reopen", "重新打开"+quote(it.Title)
	default:
		summary = op.itemUpdateSummary(&before, it)
	}
	if _, err := op.log(action, summary); err != nil {
		return nil, err
	}
	out, err := op.renderItem(it)
	return &out, err
}

// syncBlocks carries an item's title and project over to its time blocks.
func (op *Op) syncBlocks(it *store.Item) error {
	blocks, err := store.Events.List(op.ctx, op.q, op.User.ID, "item_id = ?", it.ID)
	if err != nil {
		return err
	}
	for _, b := range blocks {
		b.Title, b.ProjectID = it.Title, it.ProjectID
		if err := op.saveEvent(b, false); err != nil {
			return err
		}
	}
	return nil
}

func (op *Op) deleteItem(it *store.Item) (string, error) {
	blocks, err := store.Events.List(op.ctx, op.q, op.User.ID, "item_id = ?", it.ID)
	if err != nil {
		return "", err
	}
	for _, b := range blocks {
		if err := op.removeEvent(b); err != nil {
			return "", err
		}
	}
	if err := remove(op, kindItem, store.Items, it.ID); err != nil {
		return "", err
	}
	return "删除事项" + quote(it.Title), nil
}

// DeleteItem deletes the item, or returns a delete suggestion when the
// caller's token requires confirmation.
func (op *Op) DeleteItem(id string) (*api.Suggestion, error) {
	it, err := op.item(id)
	if err != nil {
		return nil, err
	}
	if op.needsConfirmation() {
		return op.suggest(&store.Suggestion{Kind: "delete_item", ItemID: &it.ID, Title: it.Title})
	}
	summary, err := op.deleteItem(it)
	if err != nil {
		return nil, err
	}
	_, err = op.log("item.delete", summary)
	return nil, err
}

// readSlot reads the start and end of a time slot.
func readSlot(f Fields, required bool) (start, end *string, err error) {
	r := newReader(f)
	r.instant("start", &start)
	r.instant("end", &end)
	if err := r.done(); err != nil {
		return nil, nil, err
	}
	if start == nil && end == nil && !required {
		return nil, nil, nil
	}
	if start == nil || end == nil {
		return nil, nil, Invalid("start and end are required")
	}
	return start, end, checkSlot(*start, *end)
}

// scheduleItem creates the item's time block or moves the existing one.
func (op *Op) scheduleItem(it *store.Item, start, end string) (string, error) {
	d, err := op.decorate()
	if err != nil {
		return "", err
	}
	block := op.currentBlock(d.blocks[it.ID])
	var summary string
	if block == nil {
		if block, err = op.newEvent(); err != nil {
			return "", err
		}
		block.ItemID, block.ProjectID, block.Title = &it.ID, it.ProjectID, it.Title
		block.StartAt, block.EndAt = &start, &end
		summary = quote(it.Title) + "排到 " + op.whenLabel(store.ParseTime(start))
	} else {
		before := *block
		block.StartAt, block.EndAt = &start, &end
		summary = op.movedSummary(it.Title, &before, block)
	}
	if err := op.saveEvent(block, false); err != nil {
		return "", err
	}
	return summary, op.syncBlockItem(block)
}

func (op *Op) ScheduleItem(id string, f Fields) (*api.Item, error) {
	it, err := op.item(id)
	if err != nil {
		return nil, err
	}
	start, end, err := readSlot(f, true)
	if err != nil {
		return nil, err
	}
	summary, err := op.scheduleItem(it, *start, *end)
	if err != nil {
		return nil, err
	}
	if _, err := op.log("item.schedule", summary); err != nil {
		return nil, err
	}
	return op.GetItem(id)
}

func (op *Op) UnscheduleItem(id string) (*api.Item, error) {
	it, err := op.item(id)
	if err != nil {
		return nil, err
	}
	blocks, err := store.Events.List(op.ctx, op.q, op.User.ID, "item_id = ?", it.ID)
	if err != nil {
		return nil, err
	}
	for _, b := range blocks {
		if err := op.removeEvent(b); err != nil {
			return nil, err
		}
	}
	if len(blocks) > 0 {
		if _, err := op.log("item.unschedule", "取消了"+quote(it.Title)+"的时间安排"); err != nil {
			return nil, err
		}
	}
	out, err := op.renderItem(it)
	return &out, err
}

// ItemFilter holds the query parameters of GET /items.
type ItemFilter struct {
	Status    string
	ProjectID string
	HeadingID string
	Quadrant  string
	Q         string
}

// urgentSQL is true for items due tomorrow or earlier; its one argument is tomorrow's date.
const urgentSQL = "(due_date IS NOT NULL AND due_date <= ?)"

func (op *Op) tomorrow() string {
	return op.Now.In(op.Loc).AddDate(0, 0, 1).Format(dateLayout)
}

// quadrantSQL returns the condition for one quadrant of open items.
func (op *Op) quadrantSQL(quadrant string) (string, []any, error) {
	urgent := []any{op.tomorrow()}
	switch quadrant {
	case "do":
		return "status = 'open' AND important = 1 AND " + urgentSQL, urgent, nil
	case "plan":
		return "status = 'open' AND important = 1 AND NOT " + urgentSQL, urgent, nil
	case "quick":
		return "status = 'open' AND important = 0 AND " + urgentSQL, urgent, nil
	case "later":
		return "status = 'open' AND important = 0 AND NOT " + urgentSQL, urgent, nil
	}
	return "", nil, Invalid("quadrant must be do, plan, quick or later")
}

func (f ItemFilter) where(op *Op) (string, []any, error) {
	var conds []string
	var args []any
	add := func(cond string, a ...any) {
		conds = append(conds, cond)
		args = append(args, a...)
	}
	switch f.Status {
	case "", "open":
		add("status = 'open'")
	case "done":
		add("status = 'done'")
	case "any":
	default:
		return "", nil, Invalid("status must be open, done or any")
	}
	switch f.ProjectID {
	case "":
	case "none":
		add("project_id IS NULL")
	default:
		add("project_id = ?", f.ProjectID)
	}
	switch f.HeadingID {
	case "":
	case "none":
		add("heading_id IS NULL")
	default:
		add("heading_id = ?", f.HeadingID)
	}
	if f.Quadrant != "" {
		cond, a, err := op.quadrantSQL(f.Quadrant)
		if err != nil {
			return "", nil, err
		}
		add(cond, a...)
	}
	if f.Q != "" {
		pattern := "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(strings.ToLower(f.Q)) + "%"
		add(`(lower(title) LIKE ? ESCAPE '\' OR lower(notes) LIKE ? ESCAPE '\')`, pattern, pattern)
	}
	if len(conds) == 0 {
		return "1 = 1", args, nil
	}
	return strings.Join(conds, " AND "), args, nil
}

// itemOrder puts open items first by position, then done items newest first.
const itemOrder = "status DESC, CASE WHEN status = 'done' THEN completed_at END DESC, position, created_at, rowid"

func (op *Op) queryItems(where string, args []any, limit, offset int) ([]*store.Item, error) {
	query := store.Items.SelectSQL() + " WHERE user_id = ? AND (" + where + ") ORDER BY " + itemOrder
	all := append([]any{op.User.ID}, args...)
	if limit > 0 {
		query += " LIMIT ? OFFSET ?"
		all = append(all, limit, offset)
	}
	return store.Items.Query(op.ctx, op.q, query, all...)
}

func (op *Op) ListItems(f ItemFilter, p Page) ([]api.Item, *string, int, error) {
	where, args, err := f.where(op)
	if err != nil {
		return nil, nil, 0, err
	}
	total, err := store.Items.Count(op.ctx, op.q, op.User.ID, where, args...)
	if err != nil {
		return nil, nil, 0, err
	}
	rows, err := op.queryItems(where, args, p.Limit, p.Offset)
	if err != nil {
		return nil, nil, 0, err
	}
	items, err := op.renderItems(rows)
	return items, p.next(total), total, err
}

// localDayRange returns the instants bounding a local date.
func (op *Op) localDayRange(date string) (time.Time, time.Time) {
	start := dayStart(date, op.Loc)
	return start, start.AddDate(0, 0, 1)
}
