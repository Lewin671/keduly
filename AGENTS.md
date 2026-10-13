# Agent instructions

Keduly is a calendar and task manager with a first-class interface for AI agents: a Go server
(JSON API and CalDAV), a web app, a CLI, and native iOS and Android apps.

## Layout

| Path | Contents |
|---|---|
| `cmd/keduly/` | The single binary: `keduly serve` runs the server, every other subcommand is the CLI |
| `internal/` | Server, storage, CalDAV and CLI packages |
| `internal/webui/` | Embeds the built web app (`dist/`) into the binary |
| `web/` | Web app source (TypeScript, Preact, Vite) |
| `ios/` | iOS app source (Swift, SwiftUI). `project.yml` is the project: XcodeGen writes the Xcode project from it, which is not committed |
| `android/` | Android app source (Kotlin, Jetpack Compose), a Gradle project with its wrapper |
| `skills/keduly/` | The skill that teaches an agent to use the CLI; embedded in the binary for `keduly skill install` |
| `docs/api.md` | The HTTP API. It is the contract between server and web app: update it first |
| `docs/design/` | `DESIGN.md`, `mockup.html` (web app) and `mobile.html` (phone apps), the approved design. UI work follows the mockups |
| `deploy/` | Example container and reverse proxy files |
| `scripts/` | `check.sh` runs every check of the server, the web app and the CLI; `check-ios.sh` and `check-android.sh` build and test the phone apps; `seed-demo.py` fills a development server with demo data; `build.sh` builds the release binary; `release.sh` builds the archives of a release |
| `install.sh` | What users pipe into `sh`: downloads a release and installs the binary; the skill is a separate `keduly skill install` |

## Rules

- Everything in the repository is in English: code, comments, documentation, commit messages.
  The user interface is in Chinese; its strings live in `web/src/i18n/`,
  `ios/Keduly/Resources/zh-Hans.lproj/` and `android/app/src/main/res/values/strings.xml`.
- Never commit anything that identifies a deployment or a person: host names, IP addresses,
  server paths, account names, email addresses, tokens. Use `example.com` and placeholders.
  Deployment-specific files belong outside the repository or under the ignored `deploy/local/`.
- UI changes start in `docs/design/mockup.html`, then in `web/`. Phone app changes start in
  `docs/design/mobile.html`, and land in `ios/` and `android/` together: the two apps do the same
  things with the same words.
- Every capability ships on both surfaces. A feature is not done until a person can use it in the
  web app **and** an agent can use it through the CLI, with `skills/keduly/SKILL.md` updated to
  describe it. The order is: `docs/api.md`, server, web app, CLI, skill. The only exceptions are
  the actions reserved for the person, listed under "Only the user can" in the skill (deciding
  suggestions, tokens, account settings); adding to that list is a product decision, not a shortcut.
- No compatibility code. The project is iterating fast and the server, the web app and the CLI are
  always released together, so do not keep old fields, old endpoints, fallbacks or version checks
  for clients that are behind. Change the contract and every caller in the same commit. Database
  migrations are the one exception: they carry existing data forward.
- The phone apps are clients of `docs/api.md` like the web app, and follow the same rule on
  compatibility.
- Run `scripts/check.sh` before committing. It must pass. A change under `ios/` or `android/`
  also runs that platform's check script.
- The server has no runtime dependencies besides its SQLite file. Keep it that way.

## Commands

| Task | Command |
|---|---|
| Run every check | `scripts/check.sh` |
| Build and test the iOS app (needs Xcode and XcodeGen) | `scripts/check-ios.sh` |
| Build and test the Android app (needs JDK 17 and the Android SDK) | `scripts/check-android.sh` |
| Fill a development server with demo data | `scripts/seed-demo.py http://127.0.0.1:8080` (the server must run with `--registration open`) |
| Build the release binary (web app embedded) | `scripts/build.sh` |
| Publish a release | `git tag v0.2.0 && git push origin v0.2.0`; the workflow in `.github/workflows/release.yml` builds and uploads it |
| Run the server for development | `go run ./cmd/keduly serve --data ./data --addr 127.0.0.1:8080` |
| Run the web app with hot reload | `cd web && pnpm dev` (proxies `/api` and `/dav` to `127.0.0.1:8080`) |
