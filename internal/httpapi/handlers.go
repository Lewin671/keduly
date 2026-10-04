package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/Lewin671/keduly/internal/api"
	"github.com/Lewin671/keduly/internal/core"
)

func (s *Server) routes() {
	s.handle("GET /config", public, s.config)

	s.handle("POST /auth/register", public, s.register)
	s.handle("POST /auth/login", public, s.login)
	s.handle("POST /auth/logout", public, s.logout)
	s.handle("GET /me", member, s.me)
	s.handle("PATCH /me", sessionOnly, s.updateMe)
	s.handle("POST /me/password", sessionOnly, s.changePassword)

	s.handle("GET /bootstrap", member, s.bootstrap)
	s.handle("GET /counts", member, s.counts)

	s.handle("POST /areas", member, s.createArea)
	s.handle("PATCH /areas/{id}", member, s.updateArea)
	s.handle("DELETE /areas/{id}", member, s.deleteArea)
	s.handle("POST /projects", member, s.createProject)
	s.handle("GET /projects/{id}", member, s.getProject)
	s.handle("PATCH /projects/{id}", member, s.updateProject)
	s.handle("DELETE /projects/{id}", member, s.deleteProject)
	s.handle("POST /projects/{id}/headings", member, s.createHeading)
	s.handle("PATCH /headings/{id}", member, s.updateHeading)
	s.handle("DELETE /headings/{id}", member, s.deleteHeading)

	s.handle("GET /items", member, s.listItems)
	s.handle("POST /items", member, s.createItem)
	s.handle("GET /items/{id}", member, s.getItem)
	s.handle("PATCH /items/{id}", member, s.updateItem)
	s.handle("DELETE /items/{id}", member, s.deleteItem)
	s.handle("POST /items/{id}/schedule", member, s.scheduleItem)
	s.handle("DELETE /items/{id}/schedule", member, s.unscheduleItem)

	s.handle("GET /today", member, s.today)
	s.handle("GET /upcoming", member, s.upcoming)
	s.handle("GET /overview", member, s.overview)
	s.handle("GET /matrix", member, s.matrix)

	s.handle("GET /events", member, s.listEvents)
	s.handle("POST /events", member, s.createEvent)
	s.handle("GET /events/{id}", member, s.getEvent)
	s.handle("PATCH /events/{id}", member, s.updateEvent)
	s.handle("DELETE /events/{id}", member, s.deleteEvent)
	s.handle("GET /calendar/heat", member, s.heat)
	s.handle("GET /free", member, s.free)

	s.handle("GET /focus", member, s.focus)
	s.handle("POST /focus/start", member, s.startFocus)
	s.handle("POST /focus/stop", member, s.stopFocus)
	s.handle("POST /focus/rest", member, s.restFocus)
	s.handle("GET /focus/sessions", member, s.focusSessions)
	s.handle("GET /focus/stats", member, s.focusStats)

	s.handle("GET /suggestions", member, s.listSuggestions)
	s.handle("POST /suggestions", member, s.createSuggestion)
	s.handle("POST /suggestions/accept-all", sessionOnly, s.acceptAll)
	s.handle("POST /suggestions/{id}/accept", sessionOnly, s.acceptSuggestion)
	s.handle("POST /suggestions/{id}/reject", sessionOnly, s.rejectSuggestion)
	s.handle("DELETE /suggestions/{id}", member, s.withdrawSuggestion)

	s.handle("GET /activity", member, s.listActivity)
	s.handle("POST /activity/undo", member, s.undoMany)
	s.handle("POST /activity/{id}/undo", member, s.undo)
	s.handle("POST /activity/{id}/redo", member, s.redo)

	s.handle("GET /tokens", sessionOnly, s.listTokens)
	s.handle("POST /tokens", sessionOnly, s.createToken)
	s.handle("DELETE /tokens/{id}", sessionOnly, s.deleteToken)
}

func (s *Server) config(c *call) error {
	return c.ok(obj{"registration": s.svc.Registration, "version": s.svc.Version})
}

func (s *Server) register(c *call) error {
	user, secret, err := s.svc.Register(c.r.Context(), c.fields)
	if err != nil {
		return err
	}
	s.setCookie(c.w, c.r, secret)
	writeJSON(c.w, http.StatusCreated, obj{"user": user})
	return nil
}

func (s *Server) login(c *call) error {
	user, secret, err := s.svc.Login(c.r.Context(), c.fields)
	if err != nil {
		return err
	}
	s.setCookie(c.w, c.r, secret)
	return c.ok(obj{"user": user})
}

func (s *Server) logout(c *call) error {
	if cookie, err := c.r.Cookie(cookieName); err == nil && cookie.Value != "" {
		if err := s.svc.Logout(c.r.Context(), cookie.Value); err != nil {
			return err
		}
	}
	s.setCookie(c.w, c.r, "")
	c.w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) me(c *call) error {
	return c.ok(obj{"user": s.read(c).Me(), "actor": c.id.Actor})
}

func (s *Server) updateMe(c *call) error {
	return s.write(c, http.StatusOK, func(op *core.Op) (obj, error) {
		user, err := op.UpdateMe(c.fields)
		return obj{"user": user}, err
	})
}

func (s *Server) changePassword(c *call) error {
	return s.write(c, 0, func(op *core.Op) (obj, error) { return nil, op.ChangePassword(c.fields) })
}

func (s *Server) bootstrap(c *call) error {
	b, err := s.read(c).Bootstrap()
	if err != nil {
		return err
	}
	return c.ok(b)
}

func (s *Server) counts(c *call) error {
	op := s.read(c)
	counts, err := op.Counts()
	if err != nil {
		return err
	}
	revision, err := op.Revision()
	if err != nil {
		return err
	}
	return c.ok(obj{"counts": counts, "revision": revision})
}

func (s *Server) createArea(c *call) error {
	return s.write(c, http.StatusCreated, func(op *core.Op) (obj, error) {
		area, err := op.CreateArea(c.fields)
		return obj{"area": area}, err
	})
}

func (s *Server) updateArea(c *call) error {
	return s.write(c, http.StatusOK, func(op *core.Op) (obj, error) {
		area, err := op.UpdateArea(c.pathID(), c.fields)
		return obj{"area": area}, err
	})
}

func (s *Server) deleteArea(c *call) error {
	return s.write(c, 0, func(op *core.Op) (obj, error) { return nil, op.DeleteArea(c.pathID()) })
}

func (s *Server) createProject(c *call) error {
	return s.write(c, http.StatusCreated, func(op *core.Op) (obj, error) {
		project, err := op.CreateProject(c.fields)
		return obj{"project": project}, err
	})
}

func (s *Server) getProject(c *call) error {
	detail, err := s.read(c).ProjectDetail(c.pathID())
	if err != nil {
		return err
	}
	return c.ok(detail)
}

func (s *Server) updateProject(c *call) error {
	return s.write(c, http.StatusOK, func(op *core.Op) (obj, error) {
		project, err := op.UpdateProject(c.pathID(), c.fields)
		return obj{"project": project}, err
	})
}

func (s *Server) deleteProject(c *call) error {
	return s.write(c, 0, func(op *core.Op) (obj, error) { return nil, op.DeleteProject(c.pathID()) })
}

func (s *Server) createHeading(c *call) error {
	return s.write(c, http.StatusCreated, func(op *core.Op) (obj, error) {
		heading, err := op.CreateHeading(c.pathID(), c.fields)
		return obj{"heading": heading}, err
	})
}

func (s *Server) updateHeading(c *call) error {
	return s.write(c, http.StatusOK, func(op *core.Op) (obj, error) {
		heading, err := op.UpdateHeading(c.pathID(), c.fields)
		return obj{"heading": heading}, err
	})
}

func (s *Server) deleteHeading(c *call) error {
	return s.write(c, 0, func(op *core.Op) (obj, error) { return nil, op.DeleteHeading(c.pathID()) })
}

func (s *Server) listItems(c *call) error {
	page, err := core.ParsePage(c.query("limit"), c.query("cursor"), 50)
	if err != nil {
		return err
	}
	filter := core.ItemFilter{Status: c.query("status"), ProjectID: c.query("project_id"),
		HeadingID: c.query("heading_id"), Quadrant: c.query("quadrant"), Q: c.query("q")}
	items, next, total, err := s.read(c).ListItems(filter, page)
	if err != nil {
		return err
	}
	return c.ok(obj{"items": items, "next_cursor": next, "total": total})
}

func (s *Server) createItem(c *call) error {
	return s.write(c, http.StatusCreated, func(op *core.Op) (obj, error) {
		item, err := op.CreateItem(c.fields)
		return obj{"item": item}, err
	})
}

func (s *Server) getItem(c *call) error {
	item, err := s.read(c).GetItem(c.pathID())
	if err != nil {
		return err
	}
	return c.ok(obj{"item": item})
}

func (s *Server) updateItem(c *call) error {
	return s.write(c, http.StatusOK, func(op *core.Op) (obj, error) {
		item, err := op.UpdateItem(c.pathID(), c.fields)
		return obj{"item": item}, err
	})
}

// deleted answers a delete: nothing, or 202 with the suggestion that stands
// in for it when the token needs the user's confirmation.
func deleted(suggestion any, isNil bool, err error) (obj, error) {
	if err != nil || isNil {
		return nil, err
	}
	return obj{"suggestion": suggestion, "_status": http.StatusAccepted}, nil
}

func (s *Server) deleteItem(c *call) error {
	return s.write(c, 0, func(op *core.Op) (obj, error) {
		suggestion, err := op.DeleteItem(c.pathID())
		return deleted(suggestion, suggestion == nil, err)
	})
}

func (s *Server) scheduleItem(c *call) error {
	return s.write(c, http.StatusOK, func(op *core.Op) (obj, error) {
		item, err := op.ScheduleItem(c.pathID(), c.fields)
		return obj{"item": item}, err
	})
}

func (s *Server) unscheduleItem(c *call) error {
	return s.write(c, http.StatusOK, func(op *core.Op) (obj, error) {
		item, err := op.UnscheduleItem(c.pathID())
		return obj{"item": item}, err
	})
}

func (s *Server) today(c *call) error {
	today, err := s.read(c).Today()
	if err != nil {
		return err
	}
	return c.ok(today)
}

func (s *Server) upcoming(c *call) error {
	days, err := s.read(c).Upcoming(c.query("from"), c.query("to"))
	if err != nil {
		return err
	}
	return c.ok(obj{"days": days})
}

func (s *Server) overview(c *call) error {
	projects, err := s.read(c).Overview()
	if err != nil {
		return err
	}
	return c.ok(obj{"projects": projects})
}

func (s *Server) matrix(c *call) error {
	quadrants, err := s.read(c).Matrix()
	if err != nil {
		return err
	}
	return c.ok(obj{"quadrants": quadrants})
}

func (s *Server) listEvents(c *call) error {
	from, to, err := core.ParseRange(c.query("from"), c.query("to"))
	if err != nil {
		return err
	}
	events, err := s.read(c).ListEvents(from, to)
	if err != nil {
		return err
	}
	return c.ok(obj{"events": events})
}

func (s *Server) createEvent(c *call) error {
	return s.write(c, http.StatusCreated, func(op *core.Op) (obj, error) {
		event, err := op.CreateEvent(c.fields)
		return obj{"event": event}, err
	})
}

func (s *Server) getEvent(c *call) error {
	event, err := s.read(c).GetEvent(c.pathID())
	if err != nil {
		return err
	}
	return c.ok(obj{"event": event})
}

func (s *Server) updateEvent(c *call) error {
	return s.write(c, http.StatusOK, func(op *core.Op) (obj, error) {
		event, err := op.UpdateEvent(c.pathID(), c.fields)
		return obj{"event": event}, err
	})
}

func (s *Server) deleteEvent(c *call) error {
	return s.write(c, 0, func(op *core.Op) (obj, error) {
		suggestion, err := op.DeleteEvent(c.pathID())
		return deleted(suggestion, suggestion == nil, err)
	})
}

func (s *Server) heat(c *call) error {
	days, err := s.read(c).Heat(c.query("year"))
	if err != nil {
		return err
	}
	return c.ok(obj{"days": days})
}

func (s *Server) free(c *call) error {
	slots, err := s.read(c).Free(c.query("date"), c.query("duration"))
	if err != nil {
		return err
	}
	return c.ok(obj{"slots": slots})
}

func (s *Server) focus(c *call) error {
	focus, err := s.read(c).Focus()
	if err != nil {
		return err
	}
	return c.ok(obj{"focus": focus})
}

// focusWrite runs one of the timer's commands and answers with its new state.
func (s *Server) focusWrite(c *call, fn func(*core.Op, core.Fields) (*api.Focus, error)) error {
	return s.write(c, http.StatusOK, func(op *core.Op) (obj, error) {
		focus, err := fn(op, c.fields)
		return obj{"focus": focus}, err
	})
}

func (s *Server) startFocus(c *call) error { return s.focusWrite(c, (*core.Op).StartFocus) }
func (s *Server) stopFocus(c *call) error  { return s.focusWrite(c, (*core.Op).StopFocus) }
func (s *Server) restFocus(c *call) error  { return s.focusWrite(c, (*core.Op).RestFocus) }

func (s *Server) focusSessions(c *call) error {
	sessions, err := s.read(c).FocusSessions(c.query("from"), c.query("to"))
	if err != nil {
		return err
	}
	return c.ok(obj{"sessions": sessions})
}

func (s *Server) focusStats(c *call) error {
	stats, err := s.read(c).FocusStats()
	if err != nil {
		return err
	}
	return c.ok(stats)
}

func (s *Server) listSuggestions(c *call) error {
	suggestions, err := s.read(c).ListSuggestions(c.query("status"))
	if err != nil {
		return err
	}
	return c.ok(obj{"suggestions": suggestions})
}

func (s *Server) createSuggestion(c *call) error {
	return s.write(c, http.StatusCreated, func(op *core.Op) (obj, error) {
		suggestion, err := op.CreateSuggestion(c.fields)
		return obj{"suggestion": suggestion}, err
	})
}

func (s *Server) acceptSuggestion(c *call) error {
	return s.write(c, http.StatusOK, func(op *core.Op) (obj, error) {
		suggestion, activity, err := op.AcceptSuggestion(c.pathID())
		return obj{"suggestion": suggestion, "activity": activity}, err
	})
}

func (s *Server) rejectSuggestion(c *call) error {
	return s.write(c, http.StatusOK, func(op *core.Op) (obj, error) {
		suggestion, err := op.RejectSuggestion(c.pathID())
		return obj{"suggestion": suggestion}, err
	})
}

func (s *Server) withdrawSuggestion(c *call) error {
	return s.write(c, 0, func(op *core.Op) (obj, error) { return nil, op.WithdrawSuggestion(c.pathID()) })
}

func (s *Server) acceptAll(c *call) error {
	return s.write(c, http.StatusOK, func(op *core.Op) (obj, error) {
		accepted, ids, err := op.AcceptAll()
		return obj{"accepted": accepted, "activity_ids": ids}, err
	})
}

func (s *Server) listActivity(c *call) error {
	page, err := core.ParsePage(c.query("limit"), c.query("cursor"), 20)
	if err != nil {
		return err
	}
	activities, next, err := s.read(c).ListActivity(page)
	if err != nil {
		return err
	}
	return c.ok(obj{"activities": activities, "next_cursor": next})
}

func (s *Server) undo(c *call) error {
	return s.write(c, http.StatusOK, func(op *core.Op) (obj, error) {
		activity, err := op.Undo(c.pathID())
		return obj{"activity": activity}, err
	})
}

func (s *Server) redo(c *call) error {
	return s.write(c, http.StatusOK, func(op *core.Op) (obj, error) {
		activity, err := op.Redo(c.pathID())
		return obj{"activity": activity}, err
	})
}

func (s *Server) undoMany(c *call) error {
	var ids []string
	raw, ok := c.fields["ids"]
	if !ok || json.Unmarshal(raw, &ids) != nil || len(c.fields) != 1 {
		return core.Invalid("ids must be the only field: a list of activity IDs")
	}
	return s.write(c, http.StatusOK, func(op *core.Op) (obj, error) {
		activities, err := op.UndoMany(ids)
		return obj{"activities": activities}, err
	})
}

func (s *Server) listTokens(c *call) error {
	tokens, err := s.read(c).Tokens()
	if err != nil {
		return err
	}
	return c.ok(obj{"tokens": tokens})
}

func (s *Server) createToken(c *call) error {
	return s.write(c, http.StatusCreated, func(op *core.Op) (obj, error) {
		token, err := op.CreateToken(c.fields)
		return obj{"token": token}, err
	})
}

func (s *Server) deleteToken(c *call) error {
	return s.write(c, 0, func(op *core.Op) (obj, error) { return nil, op.DeleteToken(c.pathID()) })
}
