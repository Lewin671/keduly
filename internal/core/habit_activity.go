package core

import (
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/Lewin671/keduly/internal/api"
	"strconv"
	"strings"
)

// Habit undo uses the same compare-before-restore contract as ordinary items. Rules
// cannot be removed beneath later edits/records or applied retrospectively.
func (op *Op) restoreHabit(kind, id string, expected, target json.RawMessage) error {
	habit, key, ok := strings.Cut(id, ":")
	if !ok {
		return Conflict("invalid habit activity")
	}
	var owner string
	err := op.q.QueryRowContext(op.ctx, "SELECT user_id FROM habits WHERE id=?", habit).Scan(&owner)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil && owner != op.User.ID {
		return NotFound("habit")
	}
	exists := err == nil
	if kind == "habit-record" {
		if !exists {
			return Conflict("habit no longer exists")
		}
		var body string
		err = op.q.QueryRowContext(op.ctx, "SELECT body FROM habit_records WHERE habit_id=? AND date=?", habit, key).Scan(&body)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		var current json.RawMessage
		if err == nil {
			current = json.RawMessage(body)
		}
		if !sameState(expected, current) {
			return Conflict("habit record changed since; undo it by hand")
		}
		if target == nil {
			_, err = op.q.ExecContext(op.ctx, "DELETE FROM habit_records WHERE habit_id=? AND date=?", habit, key)
		} else {
			_, err = op.q.ExecContext(op.ctx, "INSERT INTO habit_records(habit_id,date,body) VALUES(?,?,?) ON CONFLICT(habit_id,date) DO UPDATE SET body=excluded.body", habit, key, string(target))
		}
		return err
	}
	version, err := strconv.Atoi(key)
	if err != nil {
		return Conflict("invalid rule version")
	}
	var latest int
	if exists {
		if err = op.q.QueryRowContext(op.ctx, "SELECT coalesce(max(version),0) FROM habit_rules WHERE habit_id=?", habit).Scan(&latest); err != nil {
			return err
		}
	}
	var body string
	err = op.q.QueryRowContext(op.ctx, "SELECT body FROM habit_rules WHERE habit_id=? AND version=?", habit, version).Scan(&body)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	var current json.RawMessage
	if err == nil {
		current = json.RawMessage(body)
	}
	if !sameState(expected, current) {
		return Conflict("habit rule changed since; undo it by hand")
	}
	raw := target
	if raw == nil {
		raw = expected
	}
	var rule api.Habit
	if json.Unmarshal(raw, &rule) != nil {
		return Conflict("invalid rule snapshot")
	}
	if rule.EffectiveDate < op.today() {
		return Conflict("past habit rules cannot be rewritten")
	}
	var n int
	if exists {
		if err = op.q.QueryRowContext(op.ctx, "SELECT count(*) FROM habit_records WHERE habit_id=? AND date>=?", habit, rule.EffectiveDate).Scan(&n); err != nil {
			return err
		}
	}
	if n > 0 {
		return Conflict("habit records depend on this rule; undo them first")
	}
	if target == nil {
		if latest != version {
			return Conflict("habit has later edits; undo those first")
		}
		if version == 1 {
			_, err = op.q.ExecContext(op.ctx, "DELETE FROM habits WHERE id=? AND user_id=?", habit, op.User.ID)
		} else {
			_, err = op.q.ExecContext(op.ctx, "DELETE FROM habit_rules WHERE habit_id=? AND version=?", habit, version)
		}
		return err
	}
	if latest != version-1 {
		return Conflict("habit has later edits; redo is no longer applicable")
	}
	if !exists {
		all, e := op.Habits()
		if e != nil {
			return e
		}
		if len(all) >= 100 {
			return Conflict("at most 100 habits")
		}
		if version != 1 {
			return Conflict("habit no longer exists")
		}
		if _, err = op.q.ExecContext(op.ctx, "INSERT INTO habits(id,user_id,created_at) VALUES(?,?,?)", habit, op.User.ID, op.now()); err != nil {
			return err
		}
	}
	_, err = op.q.ExecContext(op.ctx, "INSERT INTO habit_rules(habit_id,version,effective_date,body) VALUES(?,?,?,?)", habit, version, rule.EffectiveDate, string(target))
	return err
}
