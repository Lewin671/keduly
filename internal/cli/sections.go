package cli

import (
	"net/http"

	"github.com/Lewin671/keduly/internal/api"
)

type areaResponse struct {
	Area api.Area `json:"area"`
}

type headingResponse struct {
	Heading api.Heading `json:"heading"`
}

func (a *app) areaList(args []string) error {
	fs := a.flags("area list", false)
	if _, err := a.parseN(fs, args, 0, ""); err != nil {
		return err
	}
	b, err := a.bootstrap()
	if err != nil {
		return err
	}
	if a.json {
		return a.printJSON(map[string]any{"areas": b.Areas})
	}
	projects := map[string]int{}
	for _, p := range b.Projects {
		if p.AreaID != nil {
			projects[*p.AreaID]++
		}
	}
	for _, area := range b.Areas {
		a.printf("%s  %s  (%d 个项目)\n", short(area.ID), area.Name, projects[area.ID])
	}
	if len(b.Areas) == 0 {
		a.printf("（还没有分组）\n")
	}
	return nil
}

func (a *app) areaAdd(args []string) error {
	fs := a.flags("area add", true)
	pos, err := a.parseN(fs, args, 1, "NAME")
	if err != nil {
		return err
	}
	var resp areaResponse
	if printed, _, err := a.send(http.MethodPost, "/areas", nil, map[string]any{"name": pos[0]}, &resp); err != nil || printed {
		return err
	}
	a.printf("已新建分组「%s」 %s%s\n", resp.Area.Name, short(resp.Area.ID), a.dryNote())
	return nil
}

func (a *app) areaRename(args []string) error {
	fs := a.flags("area rename", true)
	pos, err := a.parseN(fs, args, 2, "A NAME")
	if err != nil {
		return err
	}
	area, err := a.area(pos[0])
	if err != nil {
		return err
	}
	var resp areaResponse
	if printed, _, err := a.send(http.MethodPatch, "/areas/"+area.ID, nil, map[string]any{"name": pos[1]}, &resp); err != nil || printed {
		return err
	}
	a.printf("分组「%s」已改名为「%s」%s\n", area.Name, resp.Area.Name, a.dryNote())
	return nil
}

func (a *app) areaRemove(args []string) error {
	fs := a.flags("area rm", true)
	pos, err := a.parseN(fs, args, 1, "A")
	if err != nil {
		return err
	}
	area, err := a.area(pos[0])
	if err != nil {
		return err
	}
	if printed, _, err := a.send(http.MethodDelete, "/areas/"+area.ID, nil, nil, nil); err != nil || printed {
		return err
	}
	a.printf("已删除分组「%s」，其中的项目保留%s\n", area.Name, a.dryNote())
	return nil
}

func (a *app) headingList(args []string) error {
	fs := a.flags("heading list", false)
	pos, err := a.parseN(fs, args, 1, "P")
	if err != nil {
		return err
	}
	id, err := a.projectID(pos[0])
	if err != nil {
		return err
	}
	var d api.ProjectDetail
	if err := a.get("/projects/"+id, nil, &d); err != nil {
		return err
	}
	if a.json {
		return a.printJSON(map[string]any{"headings": d.Headings})
	}
	for _, h := range d.Headings {
		a.printf("%s  %s\n", short(h.ID), h.Name)
	}
	if len(d.Headings) == 0 {
		a.printf("（项目「%s」还没有分节）\n", d.Project.Name)
	}
	return nil
}

func (a *app) headingAdd(args []string) error {
	fs := a.flags("heading add", true)
	pos, err := a.parseN(fs, args, 2, "P NAME")
	if err != nil {
		return err
	}
	p, err := a.project(pos[0])
	if err != nil {
		return err
	}
	var resp headingResponse
	body := map[string]any{"name": pos[1]}
	if printed, _, err := a.send(http.MethodPost, "/projects/"+p.ID+"/headings", nil, body, &resp); err != nil || printed {
		return err
	}
	a.printf("已在项目「%s」中新建分节「%s」 %s%s\n", p.Name, resp.Heading.Name, short(resp.Heading.ID), a.dryNote())
	return nil
}

func (a *app) headingRename(args []string) error {
	fs := a.flags("heading rename", true)
	pos, err := a.parseN(fs, args, 2, "H NAME   (H: a heading ID, an ID prefix or PROJECT/NAME)")
	if err != nil {
		return err
	}
	h, err := a.heading(pos[0], "")
	if err != nil {
		return err
	}
	var resp headingResponse
	if printed, _, err := a.send(http.MethodPatch, "/headings/"+h.ID, nil, map[string]any{"name": pos[1]}, &resp); err != nil || printed {
		return err
	}
	a.printf("分节「%s」已改名为「%s」%s\n", h.Name, resp.Heading.Name, a.dryNote())
	return nil
}

func (a *app) headingRemove(args []string) error {
	fs := a.flags("heading rm", true)
	pos, err := a.parseN(fs, args, 1, "H   (H: a heading ID, an ID prefix or PROJECT/NAME)")
	if err != nil {
		return err
	}
	h, err := a.heading(pos[0], "")
	if err != nil {
		return err
	}
	if printed, _, err := a.send(http.MethodDelete, "/headings/"+h.ID, nil, nil, nil); err != nil || printed {
		return err
	}
	a.printf("已删除分节「%s」，其中的事项留在项目里%s\n", h.Name, a.dryNote())
	return nil
}
