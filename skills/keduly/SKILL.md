---
name: keduly
description: Operate the user's Keduly calendar and task manager through the `keduly` CLI. Use when the user asks about their schedule or agenda, tasks/to-dos/items, projects, deadlines, priorities or what to do today, wants help planning a day or week or finding free time, wants something added, captured, scheduled, moved, completed or reorganised, or asks about their pomodoro (focus) timer, tomatoes, or where their time went.
---

# Keduly

Everything goes through the `keduly` CLI. Start with `keduly whoami`: it confirms the login and
shows the account's time zone and working hours. If it reports "not signed in", ask the user for
the server URL and a token (web app, Settings), then `keduly login --server URL --token TOKEN`.

## Data model

- **Area → project → heading → item.** An area groups projects ("Work"). A project is a list of
  items and a calendar. A heading is a section of a project. An item is something to do; an item
  without a project is in the inbox.
- **Item dates**: `--when` the day to do it, `--due` the deadline, `--estimate` how long it takes.
- **Event**: something on the calendar, timed or all-day. A **time block** is an event that
  reserves time for one item; scheduling an item creates it and sets `--when` to that day.
- **Important** is the user's mark. **Urgent** is derived, never set: an open item that is overdue
  or due today or tomorrow. Quadrants: `do` important and urgent, `plan` important only, `quick`
  urgent only, `later` neither.
- **Focus**: a pomodoro timer, one per account. A **tomato** is one work period run to its end
  (`keduly whoami` shows its length, 25 minutes by default); giving it up keeps the minutes but
  earns no tomato. A rest follows each tomato and a long rest every fourth. Each tomato counts
  towards one item or is free focus, which may carry a title and a project of its own. What a
  session was for can be corrected afterwards. Tomatoes are what was actually done, next to
  `--estimate`.
- **Suggestion**: a change you propose. It shows on the user's calendar as tentative and happens
  only when the user accepts it.
- **Activity log**: every change, who made it and why. Most entries can be undone and redone.

## Operating rules

1. **Look before you write** (`agenda`, `free`, `item list --query`): no duplicates, no clashes.
2. **Suggest, do not impose.** Use `keduly suggest …` for anything that rearranges the user's time
   (scheduling, moving, cancelling) and for anything they did not explicitly ask for. Write
   directly only what the user asked for, then say what changed and what waits for them.
3. **Always pass `--reason "…"`**: one sentence the user reads next to the change or suggestion.
4. **Never delete unless asked.** With most tokens `item rm` and `event rm` only create a suggestion
   the user must confirm; say so. `project rm` is refused for those tokens: tell the user to delete
   the project in the web app, or offer `project archive`.
5. **`--dry-run` when unsure**: it validates and shows the result without writing.
6. **`--json` when you parse output**; the default text is for people.
7. **Fix your own mistakes**: `keduly undo` reverts your latest change, `keduly redo ID` applies it
   again, `keduly suggest withdraw ID` takes back a suggestion you no longer stand behind.

## Reading

| Command | Shows |
|---|---|
| `keduly agenda [--date D] [--days N]` | Per day: events, time blocks, pending suggestions (待定 = would be created, 待调整 = would move or go), planned items |
| `keduly free --date D --duration 60m` | Free slots within working hours |
| `keduly item list` | Today: planned and overdue items, free and unplanned minutes |
| `keduly item list --view inbox\|upcoming\|matrix\|done\|all` | `matrix` = the four quadrants; `upcoming` = the next 14 days |
| `keduly item list [--project P] [--heading H] [--query TEXT] [--status open\|done\|any] [--quadrant do\|plan\|quick\|later]` | Filtered items (open ones unless `--status`); filters combine |
| `keduly item show ID` | One item: notes, heading, time block, tomatoes and time spent (番茄 2 · 已用 50m) |
| `keduly event list [--from D] [--to D]` | Events (default: the next 7 days) |
| `keduly project list [--archived]` · `keduly project show P` | Projects with counts · one project: notes, headings, unplanned count, focus time this week, next events |
| `keduly area list` · `keduly heading list P` | Areas · the headings of a project |
| `keduly suggest list [--status pending\|accepted\|rejected\|any]` | Suggestions; see what the user decided |
| `keduly focus status` | The timer: idle, working (on what, time left), a tomato that ran out and waits for the user, or resting; today's tomatoes and the round |
| `keduly focus log [--from D] [--to D]` | Focus sessions (default: today's): ID, time, length, tomato or given up (未完成), item or free focus (自由专注, or its title), project |
| `keduly focus stats` | The last 7 days: tomatoes and time per day and per project, and the streak of days with a tomato |
| `keduly activity [--limit N]` | Recent changes with their IDs, authors and reasons |

## Suggesting (the default for the user's time)

| Command | Proposes |
|---|---|
| `keduly suggest schedule ITEM --start T --duration 1h --reason "…"` | A time block for an item |
| `keduly suggest add-item TITLE [item flags] [--start T --duration D] --reason "…"` | A new item, optionally with a time block |
| `keduly suggest add-event TITLE --start T --duration D --reason "…"` | A new event (`--all-day --start D [--end D]` too) |
| `keduly suggest move EVENT --start T [--duration D] --reason "…"` | Moving a timed event |
| `keduly suggest delete-item ID --reason "…"` · `keduly suggest delete-event ID --reason "…"` | Deleting |
| `keduly suggest withdraw SUGGESTION` | Takes back a pending suggestion that this token made |

## Writing (what the user asked for)

| Command | Does |
|---|---|
| `keduly item add TITLE [--project P] [--heading H] [--when D] [--due D] [--estimate 30m] [--important] [--evening] [--notes TEXT]` | Creates an item. `--due "fri 18:00"` adds a time. `--heading` is a name within `--project`, or `PROJECT/NAME` |
| `keduly item edit ID [same flags] [--title T] [--no-important] [--no-evening]` | Changes an item. `none` clears `--project`, `--heading`, `--when`, `--due`, `--estimate` |
| `keduly item done ID` · `reopen ID` · `rm ID` | Completes, reopens, deletes |
| `keduly item schedule ID --start T (--end T \| --duration 1h)` · `keduly item unschedule ID` | Sets, moves or removes the item's time block |
| `keduly event add TITLE --start T (--end T \| --duration 1h) [--project P] [--notes TEXT] [--location L]` | Creates a timed event |
| `keduly event add TITLE --all-day --start D [--end D]` | Creates an all-day event; `--end` is the last day |
| `keduly event edit ID [--title T] [--start T] [--end T \| --duration D] [--project P] [--notes TEXT] [--location L]` | Changes an event. All-day events take dates. `--all-day` / `--timed --start T --duration D` switch kind |
| `keduly event move ID --start T` · `keduly event rm ID` | Moves keeping the length · deletes |
| `keduly project add NAME [--color C] [--area A] [--notes TEXT]` | Creates a project; a missing area is created |
| `keduly project edit P [--name N] [--color C] [--area A\|none] [--notes TEXT]` | Changes a project |
| `keduly project archive P` · `unarchive P` · `rm P` | Hides, restores, deletes with all its items and events |
| `keduly area add NAME` · `rename A NAME` · `rm A` | Areas; removing one keeps its projects |
| `keduly heading add P NAME` · `rename H NAME` · `rm H` | Headings; removing one keeps its items |
| `keduly focus start [ITEM]` | Starts a tomato on the item, or free focus without one (`--title T --project P` name and file it from the start). Gives up a tomato that is running |
| `keduly focus stop` | Gives up the running tomato, skips the rest, or dismisses a tomato that ran out |
| `keduly focus rest` | Starts the rest after a tomato (the long one after every fourth) |
| `keduly focus edit SESSION [--item ITEM\|none] [--project P\|none] [--title T]` | Says what a session was for. `--item` moves its time and tomato to an item (done items too); `--item none` makes it free focus. `--project` and `--title` are for free focus only. SESSION is an ID or prefix from `focus log` |
| `keduly undo [ACTIVITY]` · `keduly redo ACTIVITY` | Undo (default: your latest change) and redo |

The timer is the user's attention: start, stop or rest it only when they ask. It is not in the
activity log and `undo` does not apply to it; neither does it apply to `focus edit`, so file a
session only where the user said what it was, and repeat the command to put it back. `keduly item done ID` also ends a tomato running on
that item.

## Only the user can

Accept or reject suggestions; create or revoke tokens and app passwords; change the account (name,
password, time zone, working hours, the lengths of a tomato and of the rests); register; sign a device in or approve
one that asks to be (the QR codes); delete a project when the token needs delete
confirmation. Do not look for a way around these: tell the user to do it in the web app.

## Conventions

- **Dates**: `today`, `tomorrow`, a weekday (`fri` = the coming Friday, today included), `YYYY-MM-DD`.
  **Times**: `HH:MM` (today), `"tomorrow 14:00"`, `"YYYY-MM-DD HH:MM"`. **Durations**: `30m`, `1h30m`.
- All dates and times are in the account's time zone (`keduly whoami`), not the machine's. JSON
  timestamps are UTC.
- **IDs** are 16 characters; lists show the first 8 and any unique prefix of 4 or more works.
  P and A take a name, an ID or a prefix; H takes an ID, a prefix or `PROJECT/NAME`.
- **Pagination**: `item list` prints 50 items (`--limit` up to 200) and, when there are more, a last
  line with the `--cursor` to pass for the next page. `--all` fetches every page (it stops at 2,000
  and prints the cursor). With `--json`, read `next_cursor` (`null` on the last page) and `total`.
- **Errors** print `keduly: message (code)` and exit 1; a wrong command line exits 2.
  `invalid_request`: a bad value, the message names it. `unauthenticated`: the token is wrong or
  revoked; ask the user. `forbidden`: the token is read-only, or the action is the user's.
  `not_found`: a wrong ID, or a suggestion another token made. `conflict`: the suggestion is
  already decided, or the data changed since; read again. `rate_limited`: wait a minute.
- Recurring events come from the user's calendar apps and are read-only here.
- `keduly help` lists every command; `keduly <command> --help` describes its flags.

## Example: plan the day

```sh
keduly agenda                                  # what is fixed today, what is already proposed
keduly item list                               # today's items, overdue ones, free vs unplanned time
keduly item list --view matrix                 # priorities: do first, then plan
keduly free --date today --duration 30m        # the gaps
keduly suggest schedule ab12cd34 --start 14:00 --duration 90m \
  --reason "Important and due today; 14:00 is the first gap that fits its 90 minutes."
```

Place `do` items first, then `plan`; give each its estimate, never overlap an event or another
proposal, and stop when the day is full: say what did not fit instead of squeezing it in. Then
summarise what you proposed and that it waits for the user to accept.

## Example: check the estimates against what happened

```sh
keduly focus stats                             # tomatoes and time per day and per project
keduly focus log --from 2026-10-12 --to today  # each session, with its ID and item
keduly focus edit 3f9a1c2e --project Keduly --title "读 CalDAV 的 RFC"  # "that free tomato was RFC reading"
keduly focus edit 3f9a1c2e --item 8b21d0aa     # or: it was really work on this item
keduly item list --status any --query sync     # 预计 1h30m · 番茄 5 · 已用 2h5m: it took longer
```

Use the gap between estimate and time spent when you plan: if items of a kind keep taking longer,
say so and propose larger slots, rather than silently padding.

## Example: capture what the user mentions

"Remind me to renew the passport before the 20th, and the sync bug belongs to the Keduly project."

```sh
keduly item list --query passport --status any # already there?
keduly project list ; keduly heading list Keduly
keduly item add "Renew passport" --due 2026-10-20 --reason "User asked to be reminded."
keduly item add "Fix sync bug" --project Keduly --heading Bugs --reason "User mentioned it."
```

When no project clearly fits, leave the item in the inbox rather than inventing one. Set `--due`
only for real deadlines and `--important` only when the user says so.
