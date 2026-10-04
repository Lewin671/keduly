---
name: keduly
description: Read and change the user's Keduly calendar and tasks through the `keduly` CLI. Use when the user asks about their schedule or agenda, items/tasks/to-dos, projects, deadlines, what to do today, finding free time, or wants something added, scheduled, moved, completed or planned.
---

# Keduly

Keduly holds the user's items (things to do), events, and projects. An item can have a time block
on the calendar. You work through the `keduly` CLI.

## Before you start

```sh
keduly whoami        # confirms the login, shows the account's time zone and working hours
```

If it reports "not signed in", ask the user for the server URL and a token (created in the web app
under Settings), then run `keduly login --server URL --token TOKEN`.

## Operating rules

1. **Look before you write.** Run `keduly agenda`, `keduly free` or `keduly item list` first, so
   you do not create duplicates or double-book the user.
2. **Propose, do not impose.** Use `keduly suggest …` for anything that changes how the user's
   time is laid out (scheduling, moving events) and for anything they did not explicitly ask for.
   A suggestion shows up on their calendar as a tentative entry that they accept or reject.
   Write directly (`item add`, `item edit`, `item schedule`, `event add`, …) only for exactly what
   the user explicitly asked you to do.
3. **Always pass `--reason "…"`** on every change and suggestion: one short sentence the user will
   read in their activity log, e.g. `--reason "Due today; this is the first free hour."`
4. **Never delete unless asked.** `item rm` and `event rm` are for explicit requests only. With most
   tokens a delete becomes a suggestion the user must confirm; say so when that happens.
5. **Use `--dry-run` when unsure.** It validates and shows the result without writing anything.
6. **Use `--json` when you parse output.** Without it the output is compact text for people.
7. If you made a mistake, `keduly undo` reverts your most recent change.

## Dates, times, IDs

- Dates: `today`, `tomorrow`, a weekday name (`fri` = the coming Friday, today included), `YYYY-MM-DD`.
- Times: `HH:MM` (today), `"YYYY-MM-DD HH:MM"`, `"tomorrow 14:00"`. Durations: `30m`, `1h30m`, `2h`.
- Everything is in the account's time zone (see `whoami`), not the machine's.
- IDs are 16 characters; any unique prefix of 4 or more works. Lists show the first 8.

## Reading

| Command | Purpose |
|---|---|
| `keduly agenda [--date D] [--days N]` | Events, time blocks, pending suggestions and planned items, day by day |
| `keduly free --date D --duration 60m` | Free slots within working hours, long enough for the duration |
| `keduly item list [--view today\|inbox\|upcoming\|all\|matrix\|done] [--project P] [--limit N]` | Items; `matrix` is the important/urgent grid |
| `keduly item show ID` | One item with its notes |
| `keduly event list [--from D] [--to D]` | Events in a range (default: the next 7 days) |
| `keduly project list` | Projects with their open and done counts |
| `keduly suggest list` | Suggestions waiting for the user |
| `keduly activity [--limit N]` | Recent changes: who, what, why |

## Suggesting (preferred for scheduling)

| Command | Purpose |
|---|---|
| `keduly suggest schedule ITEM_ID --start T --duration 1h --reason "…"` | Propose a time block for an existing item |
| `keduly suggest add-item TITLE [item flags] [--start T --duration D] --reason "…"` | Propose a new item, optionally with a time block |
| `keduly suggest add-event TITLE --start T --duration D --reason "…"` | Propose a new event |
| `keduly suggest move EVENT_ID --start T [--duration D] --reason "…"` | Propose moving an event |

## Writing directly (only what the user asked for)

| Command | Purpose |
|---|---|
| `keduly item add TITLE [--project P] [--when D] [--due D] [--estimate 30m] [--important] [--notes …]` | Create an item. `--when` is the day to do it, `--due` the deadline (`"fri 18:00"` adds a time) |
| `keduly item edit ID [same flags, --title …]` | Change an item; `none` clears `--when`, `--due`, `--estimate`, `--project` |
| `keduly item done ID` / `keduly item reopen ID` | Complete or reopen |
| `keduly item schedule ID --start T (--end T \| --duration 1h)` | Create or move the item's time block |
| `keduly item unschedule ID` | Remove the time block |
| `keduly item rm ID` | Delete (only when asked) |
| `keduly event add TITLE --start T (--end T \| --duration 1h) [--all-day] [--project P]` | Create an event |
| `keduly event edit ID [--title …] [--start T] [--end T \| --duration D]` | Change an event |
| `keduly event move ID --start T` | Move an event, keeping its length |
| `keduly event rm ID` | Delete (only when asked) |
| `keduly project add NAME [--color C] [--area A]` | Create a project |
| `keduly undo [ACTIVITY_ID]` | Undo a change; without an ID, your most recent one |

## A typical flow

The user says: "Find time for the report this week."

```sh
keduly item list --view all --json            # find the item and its estimate
keduly agenda --days 5                        # see the week
keduly free --date thu --duration 90m         # find a slot
keduly suggest schedule ab12cd34 --start "thu 14:00" --duration 90m \
  --reason "Due Friday; Thursday afternoon is the first 90-minute gap."
```

Then tell the user what you proposed and that it is waiting for them to accept.

## Notes

- Recurring events come from the user's calendar apps and are read-only here.
- Errors print as `keduly: message (code)` with a non-zero exit status. `forbidden` means the token
  is read-only; `not_found` usually means a wrong ID.
