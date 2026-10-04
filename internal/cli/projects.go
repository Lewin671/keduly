package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Lewin671/keduly/internal/api"
)

type projectResponse struct {
	Project api.Project `json:"project"`
}

func areaNames(b *api.Bootstrap) map[string]string {
	names := map[string]string{}
	for _, area := range b.Areas {
		names[area.ID] = area.Name
	}
	return names
}

func projectLine(p api.Project, areas map[string]string) string {
	line := fmt.Sprintf("%s  %s  (%s · 未完成 %d · 已完成 %d", short(p.ID), p.Name, p.Color, p.OpenCount, p.DoneCount)
	if p.AreaID != nil && areas[*p.AreaID] != "" {
		line += " · " + areas[*p.AreaID]
	}
	if p.Archived {
		line += " · 已归档"
	}
	return line + ")"
}

func (a *app) projectList(args []string) error {
	fs := a.flags("project list", false)
	archived := fs.Bool("archived", false, "also list archived projects (--json always includes them)")
	if _, err := a.parseN(fs, args, 0, "[--archived]"); err != nil {
		return err
	}
	var b api.Bootstrap
	if printed, _, err := a.send(http.MethodGet, "/bootstrap", nil, nil, &b); err != nil || printed {
		return err
	}
	areas := areaNames(&b)
	hidden := 0
	for _, p := range b.Projects {
		if p.Archived && !*archived {
			hidden++
			continue
		}
		a.printf("%s\n", projectLine(p, areas))
	}
	if len(b.Projects) == 0 {
		a.printf("（还没有项目）\n")
	}
	if hidden > 0 {
		a.printf("… 另有 %d 个已归档项目，加 --archived 查看\n", hidden)
	}
	return nil
}

func (a *app) projectShow(args []string) error {
	fs := a.flags("project show", false)
	pos, err := a.parseN(fs, args, 1, "P")
	if err != nil {
		return err
	}
	b, err := a.bootstrap()
	if err != nil {
		return err
	}
	p, err := findProject(b, pos[0])
	if err != nil {
		return err
	}
	var d api.ProjectDetail
	if printed, _, err := a.send(http.MethodGet, "/projects/"+p.ID, nil, nil, &d); err != nil || printed {
		return err
	}
	if _, err := a.zone(); err != nil {
		return err
	}
	a.printf("%s\nID %s · 未安排 %d · 本周专注 %s\n", projectLine(d.Project, areaNames(b)), d.Project.ID, d.UnplannedCount, minutesLabel(d.FocusWeekMinutes))
	if d.Project.Notes != "" {
		a.printf("\n%s\n", d.Project.Notes)
	}
	if len(d.Headings) > 0 {
		a.printf("\n分组：\n")
	}
	for _, h := range d.Headings {
		a.printf("  %s  %s\n", short(h.ID), h.Name)
	}
	if len(d.UpcomingEvents) > 0 {
		a.printf("\n近期日程：\n")
	}
	for _, e := range d.UpcomingEvents {
		a.printf("  %s  %s\n", dateLabel(a.eventDay(e)), a.eventLine(e, nil))
	}
	return nil
}

// areaID finds an area by name, ID or ID prefix, creating it when it does not exist.
func (a *app) areaID(ref string) (string, error) {
	b, err := a.bootstrap()
	if err != nil {
		return "", err
	}
	if area := findArea(b, ref); area != nil {
		return area.ID, nil
	}
	if a.dryRun {
		return "", fmt.Errorf("area %q does not exist; a dry run does not create it", ref)
	}
	var resp areaResponse
	data, _, err := a.request(http.MethodPost, "/areas", nil, map[string]any{"name": ref})
	if err != nil {
		return "", err
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", err
	}
	return resp.Area.ID, nil
}

func (a *app) projectAdd(args []string) error {
	fs := a.flags("project add", true)
	color := fs.String("color", "", "blue, indigo, orange, teal, green, pink, purple or brown (default: the least used)")
	area := fs.String("area", "", "area name or ID; created when missing")
	notes := fs.String("notes", "", "notes")
	pos, err := a.parseN(fs, args, 1, "NAME [--color C] [--area A] [--notes TEXT]")
	if err != nil {
		return err
	}
	body := map[string]any{"name": pos[0]}
	if *color != "" {
		body["color"] = *color
	}
	if *notes != "" {
		body["notes"] = *notes
	}
	if *area != "" {
		id, err := a.areaID(*area)
		if err != nil {
			return err
		}
		body["area_id"] = id
	}
	var resp projectResponse
	if printed, _, err := a.send(http.MethodPost, "/projects", nil, body, &resp); err != nil || printed {
		return err
	}
	a.printf("已新建项目「%s」 %s (%s)%s\n", resp.Project.Name, short(resp.Project.ID), resp.Project.Color, a.dryNote())
	return nil
}

func (a *app) projectEdit(args []string) error {
	fs := a.flags("project edit", true)
	name := fs.String("name", "", "new name")
	color := fs.String("color", "", "blue, indigo, orange, teal, green, pink, purple or brown")
	area := fs.String("area", "", `area name or ID, created when missing; "none" takes it out of its area`)
	notes := fs.String("notes", "", "notes; an empty string clears them")
	pos, err := a.parseN(fs, args, 1, "P [--name N] [--color C] [--area A|none] [--notes TEXT]")
	if err != nil {
		return err
	}
	body := map[string]any{}
	if given(fs, "name") {
		body["name"] = *name
	}
	if given(fs, "color") {
		body["color"] = *color
	}
	if given(fs, "notes") {
		body["notes"] = *notes
	}
	if given(fs, "area") {
		body["area_id"] = nil
		if !strings.EqualFold(*area, "none") {
			if body["area_id"], err = a.areaID(*area); err != nil {
				return err
			}
		}
	}
	if len(body) == 0 {
		return usagef("nothing to change; give at least one flag")
	}
	return a.patchProject(pos[0], body, "已修改项目")
}

func (a *app) patchProject(ref string, body map[string]any, verb string) error {
	id, err := a.projectID(ref)
	if err != nil {
		return err
	}
	var resp projectResponse
	if printed, _, err := a.send(http.MethodPatch, "/projects/"+id, nil, body, &resp); err != nil || printed {
		return err
	}
	b, err := a.bootstrap()
	if err != nil {
		return err
	}
	a.printf("%s%s\n%s\n", verb, a.dryNote(), projectLine(resp.Project, areaNames(b)))
	return nil
}

func (a *app) projectArchive(archived bool) func([]string) error {
	return func(args []string) error {
		name, verb := "unarchive", "已恢复项目"
		if archived {
			name, verb = "archive", "已归档项目"
		}
		fs := a.flags("project "+name, true)
		pos, err := a.parseN(fs, args, 1, "P")
		if err != nil {
			return err
		}
		return a.patchProject(pos[0], map[string]any{"archived": archived}, verb)
	}
}

// projectRemove deletes a project with everything in it. The server refuses a
// token whose deletions need confirmation; its message says who can.
func (a *app) projectRemove(args []string) error {
	fs := a.flags("project rm", true)
	pos, err := a.parseN(fs, args, 1, "P")
	if err != nil {
		return err
	}
	p, err := a.project(pos[0])
	if err != nil {
		return err
	}
	if printed, _, err := a.send(http.MethodDelete, "/projects/"+p.ID, nil, nil, nil); err != nil || printed {
		return err
	}
	a.printf("已删除项目「%s」及其全部分组、事项和日程%s\n", p.Name, a.dryNote())
	return nil
}
