import Foundation

/// JSON as the server sends it, written from docs/api.md.
enum Fixtures {
    static let actor = #"{ "kind": "agent", "name": "Claude Code · MacBook" }"#

    static let user = """
    { "user": {
      "id": "aaaaaaaaaaaaaaaa", "email": "me@example.com", "name": "Me",
      "timezone": "Asia/Shanghai", "timezone_auto": false,
      "work_start": "09:00", "work_end": "18:00",
      "focus_minutes": 25, "rest_minutes": 5, "long_rest_minutes": 15, "round_size": 4,
      "created_at": "2026-10-01T00:00:00Z"
    } }
    """

    static let area = #"{ "area": { "id": "bbbbbbbbbbbbbbbb", "name": "工作", "position": 0 } }"#

    static let project = """
    { "project": {
      "id": "cccccccccccccccc", "area_id": null, "name": "Keduly 开发",
      "color": "blue", "notes": "",
      "position": 0, "archived": false,
      "open_count": 7, "done_count": 3,
      "created_at": "2026-10-01T00:00:00Z", "updated_at": "2026-10-02T00:00:00Z"
    } }
    """

    static let heading = #"{ "heading": { "id": "dddddddddddddddd", "project_id": "cccccccccccccccc", "name": "同步", "position": 0 } }"#

    static let itemBody = """
    {
      "id": "eeeeeeeeeeeeeeee", "project_id": "cccccccccccccccc", "heading_id": null,
      "title": "实现 CalDAV 同步", "notes": "",
      "estimate_minutes": 90,
      "planned_date": "2026-10-13",
      "evening": false,
      "due_date": "2026-10-13", "due_time": "18:00",
      "important": true,
      "status": "open", "completed_at": null,
      "position": 0,
      "block": { "event_id": "ffffffffffffffff", "start": "2026-10-13T02:00:00Z", "end": "2026-10-13T03:30:00Z" },
      "suggestion": { "id": "gggggggggggggggg", "start": "2026-10-13T06:00:00Z", "end": "2026-10-13T07:30:00Z", "reason": "下午最长的空档。", "actor": \(actor) },
      "focus": { "tomatoes": 2, "minutes": 50 },
      "created_by": { "kind": "user", "name": "Me" }, "created_at": "2026-10-10T08:00:00Z", "updated_at": "2026-10-11T08:00:00.123Z"
    }
    """

    static let item = #"{ "item": \#(itemBody) }"#

    /// The same item with everything optional absent.
    static let bareItem = """
    { "item": {
      "id": "eeeeeeeeeeeeeeee", "project_id": null, "heading_id": null,
      "title": "给自行车打气", "notes": "",
      "estimate_minutes": null, "planned_date": null, "evening": false,
      "due_date": null, "due_time": null, "important": false,
      "status": "done", "completed_at": "2026-10-11T08:00:00Z", "position": 3,
      "block": null, "suggestion": null,
      "focus": { "tomatoes": 0, "minutes": 0 },
      "created_by": { "kind": "user", "name": "Me" }, "created_at": "2026-10-10T08:00:00Z", "updated_at": "2026-10-11T08:00:00Z"
    } }
    """

    static let eventBody = """
    {
      "id": "ffffffffffffffff", "project_id": "cccccccccccccccc", "item_id": null,
      "title": "设计评审", "notes": "", "location": "",
      "all_day": false,
      "start": "2026-10-13T02:00:00Z", "end": "2026-10-13T03:30:00Z",
      "start_date": null, "end_date": null,
      "rrule": null, "recurring": false, "instance": null,
      "status": "confirmed",
      "suggestion_id": null,
      "item_done": null,
      "readonly": false,
      "created_by": { "kind": "caldav", "name": "iPhone" }, "updated_at": "2026-10-11T08:00:00Z"
    }
    """

    static let event = #"{ "event": \#(eventBody) }"#

    static let allDayEvent = """
    { "event": {
      "id": "hhhhhhhhhhhhhhhh", "project_id": null, "item_id": null,
      "title": "两天的线下活动", "notes": "", "location": "",
      "all_day": true,
      "start": null, "end": null,
      "start_date": "2026-10-11", "end_date": "2026-10-12",
      "rrule": null, "recurring": false, "instance": null,
      "status": "confirmed", "suggestion_id": null, "item_done": null, "readonly": false,
      "created_by": { "kind": "user", "name": "Me" }, "updated_at": "2026-10-11T08:00:00Z"
    } }
    """

    /// Two instances of one series, and the two halves of a pending move.
    static let events = """
    { "events": [
      { "id": "iiiiiiiiiiiiiiii", "project_id": null, "item_id": null, "title": "周会", "notes": "", "location": "",
        "all_day": false, "start": "2026-10-12T02:00:00Z", "end": "2026-10-12T03:00:00Z", "start_date": null, "end_date": null,
        "rrule": "FREQ=WEEKLY;BYDAY=MO", "recurring": true, "instance": "2026-10-12T02:00:00Z",
        "status": "confirmed", "suggestion_id": null, "item_done": null, "readonly": true,
        "created_by": { "kind": "caldav", "name": "iPhone" }, "updated_at": "2026-10-01T00:00:00Z" },
      { "id": "iiiiiiiiiiiiiiii", "project_id": null, "item_id": null, "title": "周会", "notes": "", "location": "",
        "all_day": false, "start": "2026-10-19T02:00:00Z", "end": "2026-10-19T03:00:00Z", "start_date": null, "end_date": null,
        "rrule": "FREQ=WEEKLY;BYDAY=MO", "recurring": true, "instance": "2026-10-19T02:00:00Z",
        "status": "confirmed", "suggestion_id": null, "item_done": null, "readonly": true,
        "created_by": { "kind": "caldav", "name": "iPhone" }, "updated_at": "2026-10-01T00:00:00Z" },
      { "id": "ffffffffffffffff", "project_id": null, "item_id": null, "title": "设计评审", "notes": "", "location": "",
        "all_day": false, "start": "2026-10-13T02:00:00Z", "end": "2026-10-13T03:30:00Z", "start_date": null, "end_date": null,
        "rrule": null, "recurring": false, "instance": null,
        "status": "leaving", "suggestion_id": "gggggggggggggggg", "item_done": null, "readonly": false,
        "created_by": { "kind": "user", "name": "Me" }, "updated_at": "2026-10-01T00:00:00Z" },
      { "id": "ffffffffffffffff", "project_id": null, "item_id": "eeeeeeeeeeeeeeee", "title": "设计评审", "notes": "", "location": "",
        "all_day": false, "start": "2026-10-13T06:00:00Z", "end": "2026-10-13T07:30:00Z", "start_date": null, "end_date": null,
        "rrule": null, "recurring": false, "instance": null,
        "status": "tentative", "suggestion_id": "gggggggggggggggg", "item_done": false, "readonly": false,
        "created_by": { "kind": "user", "name": "Me" }, "updated_at": "2026-10-01T00:00:00Z" }
    ] }
    """

    static let sessionBody = """
    {
      "id": "jjjjjjjjjjjjjjjj", "kind": "work",
      "item_id": "eeeeeeeeeeeeeeee", "project_id": "cccccccccccccccc", "title": "实现 CalDAV 同步",
      "start": "2026-10-13T02:00:00Z", "end": "2026-10-13T02:25:00Z",
      "planned_minutes": 25, "completed": true,
      "created_by": { "kind": "user", "name": "Me" }
    }
    """

    static let session = #"{ "session": \#(sessionBody) }"#

    static let focus = """
    { "focus": {
      "state": "work",
      "session": \(sessionBody),
      "tomatoes_today": 3, "minutes_today": 80,
      "round_size": 4, "round_done": 3,
      "rest_minutes": 5,
      "now": "2026-10-13T02:06:18Z"
    } }
    """

    static let idleFocus = """
    { "focus": { "state": "idle", "session": null, "tomatoes_today": 0, "minutes_today": 0,
      "round_size": 6, "round_done": 0, "rest_minutes": 10, "now": "2026-10-13T02:06:18Z" } }
    """

    static let suggestions = """
    { "suggestions": [
      { "id": "gggggggggggggggg", "status": "pending", "kind": "schedule_item",
        "actor": \(actor), "reason": "今天 18:00 截止，这是下午第一个 1 小时空档。",
        "title": "写周报", "item_id": "eeeeeeeeeeeeeeee", "event_id": null,
        "start": "2026-10-13T06:00:00Z", "end": "2026-10-13T07:00:00Z",
        "item": null, "event": null,
        "created_at": "2026-10-13T00:12:00Z", "decided_at": null },
      { "id": "kkkkkkkkkkkkkkkk", "status": "accepted", "kind": "create_item",
        "actor": \(actor), "reason": "",
        "title": "订机票", "item_id": null, "event_id": null, "start": null, "end": null,
        "item": { "title": "订机票", "project_id": "cccccccccccccccc", "estimate_minutes": 20, "due_date": "2026-10-20", "important": true }, "event": null,
        "created_at": "2026-10-13T00:12:00Z", "decided_at": "2026-10-13T00:20:00Z" },
      { "id": "llllllllllllllll", "status": "rejected", "kind": "create_event",
        "actor": \(actor), "reason": "",
        "title": "体检", "item_id": null, "event_id": null, "start": null, "end": null,
        "item": null, "event": { "title": "体检", "all_day": true, "start_date": "2026-10-20", "end_date": "2026-10-20" },
        "created_at": "2026-10-13T00:12:00Z", "decided_at": "2026-10-13T00:20:00Z" },
      { "id": "mmmmmmmmmmmmmmmm", "status": "pending", "kind": "move_event",
        "actor": \(actor), "reason": "", "title": "设计评审", "item_id": null, "event_id": "ffffffffffffffff",
        "start": "2026-10-13T06:00:00Z", "end": "2026-10-13T07:30:00Z", "item": null, "event": null,
        "created_at": "2026-10-13T00:12:00Z", "decided_at": null },
      { "id": "nnnnnnnnnnnnnnnn", "status": "pending", "kind": "delete_event",
        "actor": \(actor), "reason": "", "title": "设计评审", "item_id": null, "event_id": "ffffffffffffffff",
        "start": null, "end": null, "item": null, "event": null,
        "created_at": "2026-10-13T00:12:00Z", "decided_at": null },
      { "id": "oooooooooooooooo", "status": "pending", "kind": "delete_item",
        "actor": \(actor), "reason": "", "title": "写周报", "item_id": "eeeeeeeeeeeeeeee", "event_id": null,
        "start": null, "end": null, "item": null, "event": null,
        "created_at": "2026-10-13T00:12:00Z", "decided_at": null }
    ] }
    """

    static let activity = """
    { "activities": [
      { "id": "pppppppppppppppp", "actor": \(actor), "action": "item.create",
        "summary": "新建事项「调研 FullCalendar 授权」", "reason": "为下周的评审做准备",
        "undoable": true, "undone": false, "created_at": "2026-10-13T00:12:00Z" },
      { "id": "qqqqqqqqqqqqqqqq", "actor": { "kind": "system", "name": "" }, "action": "item.update",
        "summary": "完成「写周报」", "reason": null,
        "undoable": false, "undone": true, "created_at": "2026-10-12T00:12:00Z" }
    ], "next_cursor": "abc" }
    """

    static let tokens = """
    { "tokens": [
      { "id": "rrrrrrrrrrrrrrrr", "name": "Claude Code · MacBook", "kind": "agent", "scope": "write",
        "confirm_delete": true, "last_used_at": null, "created_at": "2026-10-13T00:12:00Z" },
      { "id": "ssssssssssssssss", "name": "iPhone 日历", "kind": "caldav", "scope": "read",
        "confirm_delete": false, "last_used_at": "2026-10-13T00:12:00Z", "created_at": "2026-10-13T00:12:00Z" }
    ] }
    """

    static let config = #"{ "registration": "open", "version": "0.3.0" }"#

    static let bootstrap = """
    { "user": \(userBody), "areas": [ { "id": "bbbbbbbbbbbbbbbb", "name": "工作", "position": 0 } ],
      "projects": [], "headings": [], "counts": { "inbox": 2, "today": 6, "pending": 4 }, "revision": 41 }
    """

    static let userBody = """
    { "id": "aaaaaaaaaaaaaaaa", "email": "me@example.com", "name": "Me", "timezone": "Asia/Shanghai", "timezone_auto": true,
      "work_start": "09:00", "work_end": "18:00", "focus_minutes": 30, "rest_minutes": 5, "long_rest_minutes": 15, "round_size": 4,
      "created_at": "2026-10-01T00:00:00Z" }
    """

    static let counts = #"{ "counts": { "inbox": 2, "today": 6, "pending": 4 }, "revision": 42 }"#

    static let itemPage = #"{ "items": [ \#(itemBody) ], "next_cursor": null, "total": 1 }"#

    static let projectDetail = """
    { "project": { "id": "cccccccccccccccc", "area_id": "bbbbbbbbbbbbbbbb", "name": "Keduly 开发", "color": "brown", "notes": "备注",
        "position": 0, "archived": false, "open_count": 7, "done_count": 3,
        "created_at": "2026-10-01T00:00:00Z", "updated_at": "2026-10-02T00:00:00Z" },
      "headings": [ { "id": "dddddddddddddddd", "project_id": "cccccccccccccccc", "name": "同步", "position": 0 } ],
      "habits": [], "upcoming_events": [ \(eventBody) ], "unplanned_count": 4, "focus_week_minutes": 300 }
    """

    static let today = """
    { "date": "2026-10-13", "items": [ \(itemBody) ], "habits": [], "overdue": [], "events": [ \(eventBody) ],
      "free_minutes": 255, "unplanned_minutes": 40 }
    """

    static let upcoming = #"{ "days": [ { "date": "2026-10-14", "habits": [], "events": [ \#(eventBody) ], "items": [ \#(itemBody) ] } ] }"#

    static let overview = #"{ "projects": [ { "project_id": "cccccccccccccccc", "total": 9, "items": [ \#(itemBody) ] } ] }"#

    static let matrix = """
    { "quadrants": {
      "do": { "total": 8, "unplanned": 7, "items": [ \(itemBody) ] },
      "plan": { "total": 3, "unplanned": 2, "items": [] },
      "quick": { "total": 1, "unplanned": 1, "items": [] },
      "later": { "total": 0, "unplanned": 0, "items": [] }
    } }
    """

    static let slots = #"{ "slots": [ { "start": "2026-10-13T06:00:00Z", "end": "2026-10-13T10:00:00Z" } ] }"#

    static let stats = """
    {
      "days": [ { "date": "2026-10-07", "tomatoes": 2, "minutes": 50,
                  "projects": [ { "project_id": "cccccccccccccccc", "tomatoes": 2, "minutes": 50 } ] },
                { "date": "2026-10-08", "tomatoes": 0, "minutes": 0, "projects": [] } ],
      "projects": [ { "project_id": "cccccccccccccccc", "tomatoes": 11, "minutes": 287 }, { "project_id": null, "tomatoes": 1, "minutes": 30 } ],
      "tomatoes": 22, "minutes": 562,
      "streak": 7
    }
    """

    static let failure = #"{ "error": { "code": "invalid_request", "message": "title is required" } }"#
}
