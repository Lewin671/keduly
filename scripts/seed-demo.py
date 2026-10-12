#!/usr/bin/env python3
"""Fills a development server with made-up data for trying the apps by hand.

    go run ./cmd/keduly serve --data "$(mktemp -d)" --addr 127.0.0.1:8080 --registration open
    scripts/seed-demo.py http://127.0.0.1:8080

It registers demo@example.com (password "demo-password") with projects, items, events and one
pending suggestion, all dated around today. Run it once per data directory.
With --empty it only registers empty@example.com, an account with nothing in it.
"""
import datetime
import http.cookiejar
import json
import sys
import urllib.request


class Client:
    def __init__(self, base):
        self.base = base.rstrip("/") + "/api/v1"
        self.opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
        self.token = None

    def call(self, method, path, body=None, token=False):
        headers = {"Content-Type": "application/json", "X-Keduly-Request": "1"}
        if token:
            headers["Authorization"] = "Bearer " + self.token
        data = json.dumps(body).encode() if body is not None else (b"{}" if method != "GET" else None)
        req = urllib.request.Request(self.base + path, data=data, method=method, headers=headers)
        opener = urllib.request.build_opener() if token else self.opener
        try:
            with opener.open(req) as res:
                raw = res.read()
        except urllib.error.HTTPError as err:
            sys.exit(f"{method} {path}: {err.code} {err.read().decode()}")
        return json.loads(raw) if raw else None


def main():
    args = [a for a in sys.argv[1:] if not a.startswith("--")]
    if len(args) != 1:
        sys.exit(__doc__)
    c = Client(args[0])
    tz = "Asia/Shanghai"
    if "--empty" in sys.argv:
        c.call("POST", "/auth/register", {"email": "empty@example.com", "password": "demo-password", "name": "Empty", "timezone": tz})
        print("registered empty@example.com / demo-password")
        return
    c.call("POST", "/auth/register", {"email": "demo@example.com", "password": "demo-password", "name": "Demo", "timezone": tz})

    zone = datetime.timezone(datetime.timedelta(hours=8))
    today = datetime.datetime.now(zone).date()

    def day(offset):
        return (today + datetime.timedelta(days=offset)).isoformat()

    def at(offset, clock):
        h, m = map(int, clock.split(":"))
        local = datetime.datetime.combine(today + datetime.timedelta(days=offset), datetime.time(h, m), zone)
        return local.astimezone(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")

    work = c.call("POST", "/areas", {"name": "工作"})["area"]["id"]
    home = c.call("POST", "/areas", {"name": "个人"})["area"]["id"]

    def project(name, color, area, notes=""):
        return c.call("POST", "/projects", {"name": name, "color": color, "area_id": area, "notes": notes})["project"]["id"]

    dev = project("Keduly 移动端开发与跨平台日历同步体验改进", "blue", work, "给 AI 用的日历和事项。这个月把手机端做出来，先 iOS。")
    team = project("团队事务", "indigo", work)
    read = project("阅读与学习", "purple", home)
    life = project("生活", "green", home)
    design = c.call("POST", f"/projects/{dev}/headings", {"name": "设计"})["heading"]["id"]
    build = c.call("POST", f"/projects/{dev}/headings", {"name": "开发"})["heading"]["id"]

    def item(title, **fields):
        return c.call("POST", "/items", {"title": title, **fields})["item"]["id"]

    main_item = item("完成移动端交互设计", project_id=dev, heading_id=design, estimate_minutes=90, important=True,
                     due_date=day(0), planned_date=day(0), notes="先把事项、日历、专注三个栏定下来，再补边界场景。")
    sync = item("检查日历同步问题", project_id=dev, heading_id=build, estimate_minutes=50, important=True, due_date=day(-2))
    item("整理下一版的开发计划", project_id=dev, heading_id=design, estimate_minutes=60, important=True, due_date=day(7), planned_date=day(2))
    item("读完这一章并整理笔记", project_id=read, estimate_minutes=50, important=True, planned_date=day(0))
    item("预约下周的洗牙", project_id=life, estimate_minutes=15, due_date=day(1), planned_date=day(0))
    item("给自行车打气")
    item("续订域名", project_id=life, estimate_minutes=10, planned_date=day(0), evening=True)
    item("为安卓和 iOS 手机检查多时区、跨天日程以及夏令时切换后实际显示是否一致", project_id=dev, heading_id=build,
         estimate_minutes=120, important=True, due_date=day(-12), planned_date=day(0))
    item("回复三封积压的客户邮件", project_id=team, estimate_minutes=30, important=True, due_date=day(0), planned_date=day(0))
    feedback = item("整理项目的反馈并更新设计稿", project_id=dev, heading_id=design, estimate_minutes=50, important=True, due_date=day(0), planned_date=day(0))
    item("准备周一例会的材料", project_id=team, estimate_minutes=45, important=True, due_date=day(1), planned_date=day(0))
    item("修复登录失效后草稿丢失", project_id=dev, heading_id=build, estimate_minutes=60, important=True, due_date=day(1), planned_date=day(1))
    item("确认发布说明", project_id=dev, heading_id=build, estimate_minutes=20, important=True, due_date=day(0), planned_date=day(0))
    item("核对上个月的报销", project_id=team, important=True, due_date=day(1), planned_date=day(0))
    item("写读书笔记：第三部分", project_id=read, estimate_minutes=90, important=True, planned_date=day(3))
    item("给新成员写入门文档", project_id=team, estimate_minutes=120, important=True, due_date=day(9))
    item("交水电费", project_id=life, estimate_minutes=5, due_date=day(0), planned_date=day(0))
    item("取快递", project_id=life, due_date=day(1), planned_date=day(0), evening=True)
    item("清理下载文件夹")
    item("看看新出的键盘")
    done = item("画出三个栏的草图", project_id=dev, heading_id=design, estimate_minutes=30)
    c.call("PATCH", f"/items/{done}", {"status": "done"})
    c.call("POST", f"/items/{sync}/schedule", {"start": at(1, "09:00"), "end": at(1, "09:50")})
    c.call("POST", f"/items/{feedback}/schedule", {"start": at(0, "16:00"), "end": at(0, "16:50")})

    def event(title, project_id, start, end, offset=0):
        c.call("POST", "/events", {"title": title, "project_id": project_id, "start": at(offset, start), "end": at(offset, end)})

    c.call("POST", "/events", {"title": "两天的线下活动", "project_id": life, "all_day": True, "start_date": day(0), "end_date": day(1)})
    event("凌晨维护", dev, "01:00", "02:00")
    event("晨间散步", life, "08:00", "08:30")
    event("产品讨论", dev, "10:00", "11:00")
    event("同步故障复现和跨天事件完整检查", dev, "10:00", "11:30")
    event("阅读组讨论", read, "10:15", "11:00")
    event("快速同步", team, "10:30", "10:45")
    event("供应商电话", team, "10:40", "11:40")
    event("回复邮件", team, "13:00", "13:15")
    event("阅读", read, "20:00", "20:45")
    c.call("POST", "/events", {"title": "夜间发布", "project_id": dev, "start": at(0, "22:30"), "end": at(1, "00:30")})
    for offset, clock, title, pid in [(1, "10:00", "周会", team), (2, "14:00", "设计评审", dev), (4, "19:00", "看牙", life), (6, "09:30", "一对一", team), (-3, "15:00", "读书会", read)]:
        h, m = map(int, clock.split(":"))
        event(title, pid, clock, f"{h + 1:02d}:{m:02d}", offset)

    c.token = c.call("POST", "/tokens", {"name": "Claude Code · demo", "kind": "agent"})["token"]["token"]
    c.call("POST", "/suggestions", {"kind": "schedule_item", "item_id": main_item, "start": at(0, "14:00"), "end": at(0, "15:30"),
                                     "reason": "上午有产品讨论，下午是今天最长的一段空档。"}, token=True)
    print("registered demo@example.com / demo-password")


if __name__ == "__main__":
    main()
