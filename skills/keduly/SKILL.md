---
name: keduly
description: Operate the user's Keduly calendar and task manager through the `keduly` CLI. Use when the user asks about their schedule or agenda, tasks/to-dos/items, projects, deadlines, priorities or what to do today, wants help planning a day or week or finding free time, or wants something added, captured, scheduled, moved, completed or reorganised.
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
| `keduly item show ID` | One item: notes, heading, time block |
| `keduly event list [--from D] [--to D]` | Events (default: the next 7 days) |
| `keduly project list [--archived]` · `keduly project show P` | Projects with counts · one project: notes, headings, unplanned count, next events |
| `keduly area list` · `keduly heading list P` | Areas · the headings of a project |
| `keduly suggest list [--status pending\|accepted\|rejected\|any]` | Suggestions; see what the user decided |
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
| `keduly undo [ACTIVITY]` · `keduly redo ACTIVITY` | Undo (default: your latest change) and redo |

## Only the user can

Accept or reject suggestions; create or revoke tokens and app passwords; change the account (name,
password, time zone, working hours); register; delete a project when the token needs delete
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
