package httpapi

import (
	"github.com/Lewin671/keduly/internal/api"
	"github.com/Lewin671/keduly/internal/core"
	"net/http"
)

func (s *Server) listHabits(c *call) error {
	v, e := s.read(c).Habits()
	if e != nil {
		return e
	}
	return c.ok(obj{"habits": v})
}
func (s *Server) createHabit(c *call) error {
	return s.write(c, http.StatusCreated, func(op *core.Op) (obj, error) { v, e := op.SaveHabit("", c.fields); return obj{"habit": v}, e })
}
func (s *Server) updateHabit(c *call) error {
	return s.write(c, http.StatusOK, func(op *core.Op) (obj, error) {
		v, e := op.SaveHabit(c.r.PathValue("id"), c.fields)
		return obj{"habit": v}, e
	})
}
func (s *Server) habitOccurrences(c *call) error {
	q := c.r.URL.Query()
	var v []api.HabitOccurrence
	var e error
	switch q.Get("basis") {
	case "", "logical":
		v, e = s.read(c).HabitOccurrences(q.Get("from"), q.Get("to"))
	case "plan":
		v, e = s.read(c).PlannedHabitOccurrences(q.Get("from"), q.Get("to"))
	default:
		return core.Invalid("basis must be logical or plan")
	}
	if e != nil {
		return e
	}
	if id := q.Get("habit_id"); id != "" {
		filtered := v[:0]
		for _, o := range v {
			if o.HabitID == id {
				filtered = append(filtered, o)
			}
		}
		v = filtered
	}
	return c.ok(obj{"occurrences": v})
}
func (s *Server) updateHabitOccurrence(c *call) error {
	return s.write(c, http.StatusOK, func(op *core.Op) (obj, error) {
		v, e := op.UpdateHabitOccurrence(c.r.PathValue("id"), c.r.PathValue("date"), c.fields)
		return obj{"occurrence": v}, e
	})
}
