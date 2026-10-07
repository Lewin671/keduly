# HTTP API

The web app and the CLI talk to the server through this JSON API. It is the contract between
the backend and the frontend: change it here first, then in the code.

## Conventions

| Topic | Rule |
|---|---|
| Base path | `/api/v1` |
| Format | JSON request and response bodies, UTF-8 |
| IDs | Opaque strings (16 lowercase base32 characters). Never parse them |
| Timestamps | RFC 3339 in UTC, e.g. `2026-10-13T07:30:00Z` |
| Dates | `YYYY-MM-DD`, interpreted in the user's time zone |
| Times of day | `HH:MM`, 24-hour |
| Durations | Integer minutes |
| Absent values | `null`, never an empty string |
| Unknown fields | Rejected with `400 invalid_request` |

### Authentication

| Client | Mechanism |
|---|---|
| Web app | Session cookie `keduly_session` (HttpOnly, SameSite=Lax, Secure over HTTPS). Every request that is not `GET` must also send the header `X-Keduly-Request: 1`; requests without it are rejected with `403 csrf` |
| CLI and agents | `Authorization: Bearer <token>`. Tokens are created in Settings and start with `kdl_` |
| CalDAV clients | HTTP Basic with the account email and an app password (a token of kind `caldav`). See [CalDAV](#caldav) |

A token has a `scope`: `read` may only call `GET` endpoints, `write` may call everything except
account, token and session management, which always require a session cookie.

### Errors

Every error has the same shape and an HTTP status that matches it:

```json
{ "error": { "code": "not_found", "message": "item not found" } }
```

| Status | Codes |
|---|---|
| 400 | `invalid_request` (malformed JSON, unknown field, failed validation; `message` names the field) |
| 401 | `unauthenticated` |
| 403 | `forbidden` (token scope too narrow), `csrf`, `registration_closed` |
| 404 | `not_found` (also returned for objects owned by another user) |
| 409 | `conflict` (e.g. email already registered, suggestion already decided) |
| 429 | `rate_limited`. `/auth/` endpoints have a small budget per address, except claiming a login request |
| 500 | `internal` |

### Pagination

List endpoints that can grow without bound take `limit` (default 50, maximum 200) and `cursor`.
They return `next_cursor`, which is `null` on the last page, and `total`, the size of the whole
result ignoring pagination. Cursors are opaque.

### Dry run

Every `POST`, `PATCH` and `DELETE` on items, events, suggestions and the focus timer accepts `?dry_run=1`.
The server validates and computes the result but writes nothing, and responds with the normal
body plus `"dry_run": true`.

### Actors

Every write is attributed to an actor, which the activity log shows:

```json
{ "kind": "user" | "agent" | "caldav" | "system", "name": "Claude Code · MacBook" }
```

`kind` is `user` for the web session, `agent` for bearer tokens (the name is the token name) and
`caldav` for CalDAV clients (the name is the app password name).

## Objects

### User

```json
{
  "id": "…", "email": "me@example.com", "name": "Me",
  "timezone": "Asia/Shanghai", "timezone_auto": false,
  "work_start": "09:00", "work_end": "18:00",
  "focus_minutes": 25, "rest_minutes": 5, "long_rest_minutes": 15, "round_size": 4,
  "created_at": "…"
}
```

`focus_minutes` is the length of one tomato, `rest_minutes` the rest after it, and
`long_rest_minutes` the rest after every `round_size`-th tomato of the day. See [Focus](#focus).

`timezone` decides what "today" is, how dates are read, and which wall-clock times the web app
shows. It is set at registration and stays put: when the web app runs on a device in another zone
it asks the user whether to switch. `timezone_auto` (default `false`) is the user's opt-in to skip
the question and always follow the device.

### Area

A group of projects in the sidebar, such as "Work" or "Personal".

```json
{ "id": "…", "name": "工作", "position": 0 }
```

### Project

A project is both a list of items and a calendar: events and items belong to at most one project
and take its colour.

```json
{
  "id": "…", "area_id": "…" | null, "name": "Keduly 开发",
  "color": "blue", "notes": "",
  "position": 0, "archived": false,
  "open_count": 7, "done_count": 3,
  "created_at": "…", "updated_at": "…"
}
```

`color` is one of `blue`, `indigo`, `orange`, `teal`, `green`, `pink`, `purple`, `brown`.

### Heading

A titled section inside a project.

```json
{ "id": "…", "project_id": "…", "name": "同步", "position": 0 }
```

### Item

Something to do. An item with no project is in the inbox.

```json
{
  "id": "…", "project_id": "…" | null, "heading_id": "…" | null,
  "title": "实现 CalDAV 同步", "notes": "",
  "estimate_minutes": 90 | null,
  "planned_date": "2026-10-13" | null,
  "evening": false,
  "due_date": "2026-10-13" | null, "due_time": "18:00" | null,
  "important": false,
  "status": "open" | "done", "completed_at": "…" | null,
  "position": 0,
  "block": { "event_id": "…", "start": "…", "end": "…" } | null,
  "suggestion": { "id": "…", "start": "…", "end": "…", "reason": "…", "actor": { … } } | null,
  "focus": { "tomatoes": 2, "minutes": 50 },
  "created_by": { … }, "created_at": "…", "updated_at": "…"
}
```

| Field | Meaning |
|---|---|
| `planned_date` | The day the user intends to do it. Items planned for today or earlier and still open show in Today |
| `evening` | Show under "This evening" in Today |
| `due_date`, `due_time` | The deadline. `due_time` requires `due_date` |
| `important` | The user's mark. Urgency is never stored: an open item is urgent when `due_date` is tomorrow or earlier |
| `block` | The confirmed time block on the calendar: the next one that has not ended, or else the most recent one. Scheduling an item sets `planned_date` to the block's day |
| `suggestion` | A pending suggestion to schedule this item, if any |
| `focus` | Time spent on the item with the focus timer: completed tomatoes, and minutes including sessions that were given up. A session still running is not counted |

### Event

Something on the calendar. A time block for an item is an event with `item_id` set.

```json
{
  "id": "…", "project_id": "…" | null, "item_id": "…" | null,
  "title": "设计评审", "notes": "", "location": "",
  "all_day": false,
  "start": "2026-10-13T02:00:00Z", "end": "2026-10-13T03:30:00Z",
  "start_date": null, "end_date": null,
  "rrule": null, "recurring": false, "instance": null,
  "status": "confirmed" | "tentative" | "leaving",
  "suggestion_id": null,
  "item_done": null,
  "readonly": false,
  "created_by": { … }, "updated_at": "…"
}
```

| Field | Meaning |
|---|---|
| `all_day` | When true, `start_date` and `end_date` are set (both inclusive) and `start`/`end` are `null` |
| `rrule` | The iCalendar recurrence rule of the series, e.g. `FREQ=WEEKLY;BYDAY=MO`. Only present on events that came from a CalDAV client or were created with one |
| `recurring`, `instance` | Listing events expands a series into instances. Each instance has the series `id`, `recurring: true` and `instance` set to that occurrence's original start |
| `status` | `confirmed` is a normal event. `tentative` does not exist yet: it is what a pending suggestion would create. `leaving` is a real event that a pending suggestion would move or delete |
| `suggestion_id` | Set when `status` is `tentative` or `leaving` |
| `item_done` | For time blocks, whether the item is done; otherwise `null` |
| `readonly` | True for instances of recurring events: they can be changed from a CalDAV client, not from the web app |

### Focus session

One stretch of the focus timer: a tomato being worked on, or a rest.

```json
{
  "id": "…", "kind": "work" | "rest",
  "item_id": "…" | null, "project_id": "…" | null, "title": "实现 CalDAV 同步",
  "start": "…", "end": "…",
  "planned_minutes": 25, "completed": true,
  "created_by": { … }
}
```

| Field | Meaning |
|---|---|
| `item_id` | The item the time counts towards; `null` is free focus. Always `null` for a rest |
| `project_id`, `title` | With an item: the item's project and title, filled in by the server; a session whose item was deleted keeps the title it had and has no project. Free focus has its own: a title that says what the time went to (empty when it was not said) and a project it is filed under (`null` for none, or when that project was deleted) |
| `end` | When the session ends. For one that is running it is the planned end, `start` plus `planned_minutes`; giving it up moves `end` to that moment |
| `completed` | The session ran its full length and its end has passed. A completed `work` session is one tomato |

### Focus

The state of the user's one timer.

```json
{
  "state": "idle" | "work" | "over" | "rest",
  "session": { … } | null,
  "tomatoes_today": 3, "minutes_today": 80,
  "round_size": 4, "round_done": 3,
  "rest_minutes": 5,
  "now": "…"
}
```

| Field | Meaning |
|---|---|
| `state` | `work` and `rest`: `session` is running. `over`: `session` is a tomato that ran out and the user has not said what next; it lasts until answered or until the day ends. `idle`: `session` is `null` |
| `tomatoes_today`, `minutes_today` | Completed tomatoes today, and minutes of work today including sessions that were given up |
| `round_done` | How many tomato marks of the current round are filled, 0 to `round_size`. It stays at `round_size` from the tomato that completes the round until the rest after it is over |
| `rest_minutes` | The length of the rest that `POST /focus/rest` would start now: `long_rest_minutes` when a round has just been completed, otherwise `rest_minutes` |
| `now` | The server's clock, so a client can count down correctly when its own clock is off |

### Suggestion

A change proposed by an agent that takes effect only when the user accepts it.

```json
{
  "id": "…", "status": "pending" | "accepted" | "rejected",
  "kind": "schedule_item" | "create_item" | "create_event" | "move_event" | "delete_event" | "delete_item",
  "actor": { … }, "reason": "今天 18:00 截止，这是下午第一个 1 小时空档。",
  "title": "写周报",
  "item_id": "…" | null, "event_id": "…" | null,
  "start": "…" | null, "end": "…" | null,
  "item": { … } | null, "event": { … } | null,
  "created_at": "…", "decided_at": "…" | null
}
```

| Kind | Required input | Effect when accepted |
|---|---|---|
| `schedule_item` | `item_id`, `start`, `end` | Creates the item's time block |
| `create_item` | `item` (fields of a new item), optional `start`, `end` | Creates the item, and its time block when a slot is given |
| `create_event` | `event` (fields of a new event) | Creates the event |
| `move_event` | `event_id`, `start`, `end` | Moves the event |
| `delete_event` | `event_id` | Deletes the event |
| `delete_item` | `item_id` | Deletes the item |

`title` is the title of the affected item or event, filled in by the server for display.

### Activity

One entry per change, newest first.

```json
{
  "id": "…", "actor": { … },
  "action": "item.create",
  "summary": "新建事项「调研 FullCalendar 授权」",
  "reason": "…" | null,
  "undoable": true, "undone": false,
  "created_at": "…"
}
```

`summary` is written by the server in Chinese and is ready to display.

### Token

```json
{
  "id": "…", "name": "Claude Code · MacBook",
  "kind": "agent" | "caldav",
  "scope": "read" | "write",
  "confirm_delete": true,
  "last_used_at": "…" | null, "created_at": "…"
}
```

The secret is returned once, in the `token` field of the creation response.

## Endpoints

Unless stated otherwise, a successful response is `200` with the object under its own name,
e.g. `{ "item": { … } }`, and lists are under the plural, e.g. `{ "items": [ … ] }`.
`DELETE` returns `204` with no body.

### Service

| Method and path | Auth | Description |
|---|---|---|
| `GET /config` | none | `{ "registration": "open" \| "closed", "version": "…" }` |
| `GET /healthz` (not under `/api/v1`) | none | `200 ok` |

### Account

| Method and path | Body | Response |
|---|---|---|
| `POST /auth/register` | `email`, `password` (at least 8 characters), `name`, `timezone` | `201 { user }`, sets the session cookie. `403 registration_closed` when sign-up is off |
| `POST /auth/login` | `email`, `password` | `{ user }`, sets the session cookie. `401 unauthenticated` on a wrong email or password |
| `POST /auth/logout` | | `204` |
| `GET /me` | | `{ user }` |
| `PATCH /me` | any of `name`, `timezone`, `timezone_auto`, `work_start`, `work_end`, `focus_minutes` (1 to 180), `rest_minutes` (1 to 60), `long_rest_minutes` (1 to 120), `round_size` (2 to 12) | `{ user }` |
| `POST /me/password` | `current`, `new` | `204`. Ends every other session |

A new account starts with no projects.

### Signing in on another device

Two ways to sign a device in without typing the password, each built around a QR code. Both last
2 minutes and work once. Only a session cookie can approve or issue: tokens get `403 forbidden`.
A request or code that is unknown, expired, used, refused or withdrawn is `404 not_found`.
Changing the password withdraws all of the account's.

**Login request**: the new device shows the QR code and a signed-in device approves it. The QR
code holds `{origin}/#approve={id}`.

| Method and path | Auth | Body | Response |
|---|---|---|---|
| `POST /auth/requests` | none | | `201 { request, pin, secret }`. `request` is `{ id, device, expires_at }`. The device shows `pin` (4 digits) beside the QR code and keeps `secret` to itself |
| `POST /auth/requests/{id}/claim` | none | `secret` | `{ "status": "pending" }` while it waits. Once approved, `{ "status": "approved", user }` and the session cookie, exactly once. Polled every 2 seconds |
| `GET /auth/requests/{id}` | session | | `{ request }`, without the pin. `device` names the asking browser and system from a fixed list, e.g. `Chrome · Mac`, and is empty when unrecognised |
| `POST /auth/requests/{id}/approve` | session | `pin` | `204`: the asking device may sign in as the caller. A wrong pin is `400 invalid_request`; the third one deletes the request |
| `DELETE /auth/requests/{id}` | session | | `204`. Refuses the request |

The id in the QR code is not a secret. The session goes to the holder of `secret`, and only after
a signed-in user typed the pin, which is on the asking device's screen and nowhere else: a link
sent to someone is not enough to be let in.

**Login code**: a signed-in device shows the QR code and the new device opens it. The QR code
holds `{origin}/#signin={code}`; the code is in the fragment so that it is never sent in a `GET`.

| Method and path | Auth | Body | Response |
|---|---|---|---|
| `POST /auth/codes` | session | | `201 { code, expires_at }`. An account has one code at a time: this replaces the previous one |
| `DELETE /auth/codes` | session | | `204`. Withdraws the code |
| `POST /auth/codes/check` | none | `code` | `{ name, email }` of the account, so the device can ask before signing in. Does not use the code |
| `POST /auth/codes/redeem` | none | `code` | `{ user }`, sets the session cookie and ends the session the device had |

A code is a credential until it is used or expires. The server stores only its hash.

### Bootstrap and change detection

| Method and path | Response |
|---|---|
| `GET /bootstrap` | `{ user, areas, projects, headings, counts, revision }` |
| `GET /counts` | `{ counts, revision }` |

`counts` is `{ "inbox": 2, "today": 6, "pending": 4 }`: open inbox items, open items in Today
(including overdue ones) and pending suggestions.

`revision` is an integer that increases on every write to the user's data, whoever made it. The
web app polls `GET /counts` every 20 seconds and reloads what it shows when `revision` changes.

### Areas, projects and headings

| Method and path | Body | Notes |
|---|---|---|
| `POST /areas` | `name` | |
| `PATCH /areas/{id}` | `name`, `position` | |
| `DELETE /areas/{id}` | | Its projects stay and lose their area |
| `POST /projects` | `name`, optional `color`, `area_id`, `notes` | `color` defaults to the least used one |
| `GET /projects/{id}` | | `{ project, headings, upcoming_events, unplanned_count, focus_week_minutes }`. `upcoming_events` is the project's next 3 events from now, time blocks excluded. `focus_week_minutes` is the focus time since Monday on the project's items and on free focus filed under the project |
| `PATCH /projects/{id}` | any of `name`, `color`, `area_id`, `notes`, `position`, `archived` | |
| `DELETE /projects/{id}` | | Also deletes its headings, items and events. `403 forbidden` for a token with `confirm_delete`: no suggestion kind stands in for this delete, so the user does it in the web app |
| `POST /projects/{id}/headings` | `name` | |
| `PATCH /headings/{id}` | `name`, `position` | |
| `DELETE /headings/{id}` | | Its items stay in the project without a heading |

### Items

| Method and path | Body or query | Response |
|---|---|---|
| `GET /items` | filters below, `limit`, `cursor` | `{ items, next_cursor, total }` |
| `POST /items` | `title`, optional `project_id`, `heading_id`, `notes`, `estimate_minutes`, `planned_date`, `evening`, `due_date`, `due_time`, `important` | `201 { item }` |
| `GET /items/{id}` | | `{ item }` |
| `PATCH /items/{id}` | any writable field, and `status` | `{ item }` |
| `DELETE /items/{id}` | | `204`, or `202 { suggestion }` when the caller is a token with `confirm_delete` |
| `POST /items/{id}/schedule` | `start`, `end` | `{ item }`. Creates the time block or moves the existing one |
| `DELETE /items/{id}/schedule` | | `{ item }`. Removes the time block |

Setting `status` to `done` records `completed_at`; setting it back to `open` clears it. Completing
the item the focus timer is on also ends the timer, as `POST /focus/stop` would.

Filters for `GET /items`, combined with AND:

| Parameter | Values |
|---|---|
| `status` | `open` (default), `done`, `any` |
| `project_id` | a project ID, or `none` for the inbox |
| `heading_id` | a heading ID, or `none` |
| `quadrant` | `do` (important, urgent), `plan` (important, not urgent), `quick` (not important, urgent), `later` (neither). Open items only |
| `q` | Case-insensitive substring of the title or notes |

Open items are ordered by `position`, then creation time. Done items are ordered by
`completed_at`, newest first.

### Views

These endpoints assemble what one screen needs in a single request.

| Method and path | Response |
|---|---|
| `GET /today` | `{ date, items, overdue, events, free_minutes, unplanned_minutes }` |
| `GET /upcoming?from=&to=` | `{ days: [ { date, events, items } ] }` |
| `GET /overview` | `{ projects: [ { project_id, total, items } ] }` |
| `GET /matrix` | `{ quadrants: { do, plan, quick, later } }` |

`GET /today`

| Field | Meaning |
|---|---|
| `items` | Open items with `planned_date` today or earlier, plus items completed today. Important items first |
| `overdue` | Open items whose `due_date` is before today and that are not already in `items` |
| `events` | Today's events, time blocks excluded, all-day ones first |
| `free_minutes` | Minutes between `work_start` and `work_end` not covered by any event or time block (pending suggestions count as covered). Overlapping events are counted once |
| `unplanned_minutes` | Work left on items in `items` that are open, not `evening`, and have neither a time block nor a pending suggestion: each item's `estimate_minutes` less its `focus.minutes`, never below zero |

`GET /upcoming` takes two dates (inclusive, at most 62 days apart) and returns only days that have
something. `items` are open items planned for that day, or due that day when they have no planned
date. `events` exclude time blocks.

`GET /overview` returns every project that has open items, each with its first 5 open items and
the `total` number of open items. Load the rest with `GET /items?project_id=…`.

`GET /matrix` returns, per quadrant, `{ total, unplanned, items }` with the first 6 items.
`unplanned` counts items that have neither a time block nor a pending suggestion. Load the rest
with `GET /items?quadrant=…`.

### Events

| Method and path | Body or query | Response |
|---|---|---|
| `GET /events?from=&to=` | two timestamps, at most 400 days apart | `{ events }` |
| `POST /events` | `title`, `start`, `end`, or `all_day: true` with `start_date`, `end_date`; optional `project_id`, `notes`, `location` | `201 { event }` |
| `GET /events/{id}` | | `{ event }` |
| `PATCH /events/{id}` | any writable field | `{ event }` |
| `DELETE /events/{id}` | | `204`, or `202 { suggestion }` when the caller is a token with `confirm_delete` |
| `GET /calendar/heat?year=2026` | | `{ days: { "2026-10-13": 7, … } }` |
| `GET /free?date=&duration=` | a date and a number of minutes | `{ slots: [ { start, end } ] }` |

`GET /events` returns everything that overlaps the range, sorted by start:

- events and time blocks, with recurring series expanded into instances;
- for each pending suggestion that touches the range, a `tentative` entry for what it would
  create (for `schedule_item`, `create_item` with a slot, `create_event` and the new position of
  `move_event`), and the affected event marked `leaving` (for `move_event` and `delete_event`).

`GET /calendar/heat` counts, per day in the user's time zone, the events and time blocks on that
day. Days with nothing are omitted.

`GET /free` returns the gaps of at least `duration` minutes between `work_start` and `work_end`
on that day.

### Focus

The focus timer is a pomodoro timer. There is one per user, and it lives on the server as
sessions with a start and an end, so every client shows the same countdown and nothing has to be
running for a tomato to complete.

| Method and path | Body or query | Response |
|---|---|---|
| `GET /focus` | | `{ focus }` |
| `POST /focus/start` | optional `item_id`; without it, optional `title` and `project_id` | `{ focus }`. Starts a tomato of `focus_minutes` on the item, or free focus without one |
| `POST /focus/stop` | | `{ focus }`. Gives up the running tomato, skips the running rest, or answers `over` with "nothing"; the state becomes `idle` |
| `POST /focus/rest` | | `{ focus }`. Starts a rest of `rest_minutes` as reported by `GET /focus`. `409 conflict` while a tomato is running |
| `GET /focus/sessions?from=&to=` | two dates (inclusive, at most 62 days apart) | `{ sessions }`: the `work` sessions that started on those days, oldest first, the running one included |
| `PATCH /focus/sessions/{id}` | any of `item_id`, `title`, `project_id` | `{ session }`. Says what a `work` session was for; see "Filing a session" |
| `GET /focus/stats` | | see below |

| Rule | Detail |
|---|---|
| Giving up | The session's `end` becomes now and it keeps its minutes, but it is not `completed` and earns no tomato. A session given up within its first minute is deleted |
| Starting while something runs | `POST /focus/start` first gives up the running tomato, or ends the running rest |
| Item | `POST /focus/start` takes an `item_id` that names an open item. Deleting an item keeps its sessions |
| Filing a session | `PATCH /focus/sessions/{id}` works on any `work` session, the running one included; it never changes when the session was or whether it is a tomato. `item_id` puts it on any of the user's items, open or done, and it then counts towards that item. `item_id: null` takes it off its item, leaving free focus with no title and no project; on a session that already is free focus it changes nothing. `title` (at most 500 characters, trimmed, may be empty) and `project_id` (a project or `null`) are for free focus only: together with an item, or on a session that has one, they are `400 invalid_request` |
| Round | The rest that follows the tomato completing a round (`tomatoes_today` divisible by `round_size`) is the long one |
| Activity | Starting and stopping the timer and filing a session are not written to the activity log; the sessions are the record |

`GET /focus/stats` covers the last 7 days in the user's time zone, today included:

```json
{
  "days": [ { "date": "2026-10-07", "tomatoes": 2, "minutes": 50,
              "projects": [ { "project_id": "…" | null, "tomatoes": 2, "minutes": 50 } ] }, … ],
  "projects": [ { "project_id": "…" | null, "tomatoes": 11, "minutes": 287 }, … ],
  "tomatoes": 22, "minutes": 562,
  "streak": 7
}
```

`days` always has 7 entries, oldest first. `projects` is ordered by minutes, most first;
`project_id` `null` gathers free focus that is not filed under a project, inbox items and deleted items. `streak` is the number of
consecutive days up to today with at least one tomato, counted as far back as it goes; a today
without a tomato yet does not break it.

### Suggestions

| Method and path | Body | Response |
|---|---|---|
| `GET /suggestions?status=pending` | `status` is `pending` (default), `accepted`, `rejected` or `any` | `{ suggestions }` |
| `POST /suggestions` | `kind`, `reason`, and the inputs that kind requires | `201 { suggestion }` |
| `POST /suggestions/{id}/accept` | | `{ suggestion, activity }` |
| `POST /suggestions/{id}/reject` | | `{ suggestion }` |
| `POST /suggestions/accept-all` | | `{ accepted, activity_ids }` |
| `DELETE /suggestions/{id}` | | `204`. Withdraws a pending suggestion |

Deciding a suggestion requires a session cookie: an agent cannot accept its own proposals.
A suggestion that can no longer be applied (its item or event is gone) is rejected with
`409 conflict` on accept and is then marked `rejected`.

Withdrawing is the proposer taking a suggestion back. A session may withdraw any pending
suggestion; a `write` token only the ones that token itself created, and gets `404 not_found` for
every other one. A suggestion that is no longer pending answers `409 conflict`. A withdrawn
suggestion is removed: it has no status of its own and is not in the activity log.

### Activity

| Method and path | Body | Response |
|---|---|---|
| `GET /activity` | `limit` (default 20), `cursor` | `{ activities, next_cursor }` |
| `POST /activity/{id}/undo` | | `{ activity }` |
| `POST /activity/{id}/redo` | | `{ activity }` |
| `POST /activity/undo` | `ids` | `{ activities }`. Undoes several entries, newest first |

Undo restores the affected objects to their state before the change; redo applies the change
again. Both fail with `409 conflict` when the objects have changed since in a way that makes the
result ambiguous.

### Tokens

Session cookie only.

| Method and path | Body | Response |
|---|---|---|
| `GET /tokens` | | `{ tokens }` |
| `POST /tokens` | `name`, `kind`, optional `scope` (default `write`), `confirm_delete` (default `true`) | `201 { token }` with the secret in `token.token` |
| `DELETE /tokens/{id}` | | `204` |

## CalDAV

The CalDAV endpoint lets the calendar apps on iPhone, Mac, Android (through DAVx⁵) and others
show and edit the same data.

| Topic | Rule |
|---|---|
| Root | `/dav/`. `/.well-known/caldav` redirects to it |
| Login | HTTP Basic: the account email and an app password created in Settings |
| Principal | `/dav/principals/{user-id}/` |
| Calendar home | `/dav/calendars/{user-id}/` |
| Calendars | One per project, at `/dav/calendars/{user-id}/{project-id}/`, with the project's name and colour. Events without a project are in `/dav/calendars/{user-id}/inbox/` |
| Objects | `VEVENT` only. One resource per event, named `{event-id}.ics`. Time blocks appear as ordinary events |
| Writes | Creating, changing and deleting events is supported. Creating or deleting calendars is not: projects are managed in the web app |
| Change detection | Each object has an ETag; each calendar has a CTag (`getctag`) that changes whenever anything in it does |

Events created by a CalDAV client keep their original iCalendar text, so properties this server
does not understand survive a round trip.
