# Keduly

A calendar and task manager built for working with AI agents.

- **Projects organise everything.** A project is both a list of items and a calendar colour, so the
  calendar shows where your time goes.
- **One timeline.** Events and the time blocks of your items share the same calendar: day, week,
  month and year.
- **Agents propose, you decide.** An agent reads and writes through the command line, but its
  scheduling suggestions appear as tentative entries that take effect only when you accept them.
  Deletes by an agent wait for your consent.
- **Every change is traceable.** The activity log records who changed what and why, and each
  change can be undone.
- **A pomodoro timer that knows your items.** Focus on an item one tomato at a time; the time
  actually spent shows next to its estimate, and statistics show where the week went.
- **Important and urgent.** Mark what is important; urgency follows from the deadline. A
  four-quadrant list sorts everything open.
- **Works with the calendar apps you already use** through CalDAV: iPhone, Mac, Android with
  DAVx⁵, Thunderbird.
- **One binary.** A Go server with the web app, SQLite and the time zone database inside. No other
  services to run.

The interface is in Chinese for now.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/Lewin671/keduly/main/install.sh | sh
```

This downloads the `keduly` binary for macOS or Linux from the
[latest release](https://github.com/Lewin671/keduly/releases/latest), verifies its checksum, puts
it in `~/.local/bin`, and installs the agent skill for Claude Code. The one binary is both the
server and the command line. Run the line again to upgrade. The variables at the top of
[install.sh](install.sh) choose another version, directory or agent.

To build from source instead you need Go 1.25 or later, and Node with `pnpm` for the web app:
`scripts/build.sh` writes `bin/keduly`.

## Run it

```sh
keduly serve --addr 127.0.0.1:8080 --data ./data
```

Open <http://127.0.0.1:8080> and create an account. [docs/deploy.md](docs/deploy.md) covers running
it on a server behind HTTPS, the container image, configuration and backups.

## Use it from an agent

Create a token in Settings → AI 凭证, then on the machine where the agent runs:

```sh
keduly login --server https://keduly.example.com      # paste the token when asked
keduly agenda                                          # what is on today
keduly free --date tomorrow --duration 60m             # find a gap
keduly item add "Prepare the demo" --project "Client" --estimate 1h --due wednesday --important
keduly suggest schedule <item-id> --start "14:00" --duration 1h --reason "First free hour before the deadline"
keduly undo
```

Every command takes `--json`; every command that changes something takes `--dry-run` and
`--reason`. `keduly help` lists them all.

[skills/keduly/SKILL.md](skills/keduly/SKILL.md) teaches an agent when and how to use the CLI. The
binary carries the skill that matches it: `keduly skill install` writes it into Claude Code's
skills directory, and `--dir` names the skills directory of another agent. The install script
already does this.

## Use it from a calendar app

Create an app password in Settings → 系统日历 and add a CalDAV account in your calendar app with
the server address shown there, your email and that password. Each project appears as a calendar.

## Documentation

| Document | Contents |
|---|---|
| [docs/design/DESIGN.md](docs/design/DESIGN.md) | What the product is and why it looks the way it does |
| [docs/design/mockup.html](docs/design/mockup.html) | The clickable design mockup |
| [docs/api.md](docs/api.md) | The HTTP API and the CalDAV mapping |
| [docs/deploy.md](docs/deploy.md) | Self-hosting |
| [AGENTS.md](AGENTS.md) | Layout, rules and commands for contributors, human or not |

## Development

```sh
go run ./cmd/keduly serve --data ./data --addr 127.0.0.1:8080   # server
cd web && pnpm install && pnpm dev                              # web app with hot reload
scripts/check.sh                                                # formatting, vet, all tests
```

## Status

Early. The server, web app, CLI and CalDAV endpoint work end to end and are covered by tests, but
CalDAV has so far been exercised with a client library rather than with every calendar app, and
the phone layout is basic.

## License

[AGPL-3.0](LICENSE)
