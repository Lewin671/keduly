# Self-hosting

Keduly is one static binary and one directory of data. There is nothing else to install.

## Build

Each [release](https://github.com/Lewin671/keduly/releases) has ready-made binaries for macOS and
Linux, which `install.sh` in the repository root downloads. To build your own:

```sh
scripts/build.sh                             # bin/keduly for this machine
GOOS=linux GOARCH=amd64 scripts/build.sh     # cross-compile for a server
```

The script builds the web app when `web/` is present (it needs `pnpm`), embeds it, and compiles
with `CGO_ENABLED=0`. The result has no runtime dependencies: SQLite, the time zone database and
the web app are inside the binary.

## Run

```sh
keduly serve --addr 127.0.0.1:8080 --data /var/lib/keduly
```

| Flag | Environment | Default | Meaning |
|---|---|---|---|
| `--addr` | `KEDULY_ADDR` | `127.0.0.1:8080` | Address to listen on |
| `--data` | `KEDULY_DATA` | `./data` | Directory that holds `keduly.db` |
| `--registration` | `KEDULY_REGISTRATION` | `open` | `open` lets anyone sign up; `closed` turns sign-up off |
| `--base-url` | `KEDULY_BASE_URL` | none | Public URL. When it starts with `https://`, session cookies are marked `Secure` |

Open the address in a browser, create your account, then set `KEDULY_REGISTRATION=closed` and
restart if the server is only for you.

The server stops cleanly on `SIGINT` and `SIGTERM`, and logs one JSON line per request to standard
output.

## HTTPS

Run Keduly behind a reverse proxy that terminates HTTPS and forwards **every** path, including
`/dav/` and `/.well-known/caldav`, which calendar apps use. The proxy must pass the client address
in `X-Forwarded-For` and the scheme in `X-Forwarded-Proto`; Caddy and nginx's usual proxy settings
do. Keduly trusts these headers only from loopback and private addresses, so do not expose its
port directly to the internet next to the proxy.

`deploy/Caddyfile.example` is a complete Caddy configuration:

```
keduly.example.com {
	encode zstd gzip
	reverse_proxy 127.0.0.1:8080
}
```

## Container

`deploy/Dockerfile` wraps a prebuilt Linux binary in an empty image:

```sh
GOOS=linux GOARCH=amd64 scripts/build.sh
docker build -f deploy/Dockerfile -t keduly .
docker run -d -p 127.0.0.1:8080:8080 -v keduly-data:/data keduly
```

The server runs as user 65534 and writes only to `/data`. A named volume gets the right owner by
itself; a host directory mounted there must be owned by 65534 (`chown 65534:65534 <dir>`).

`deploy/compose.example.yaml` runs it together with Caddy, both with a read-only root file system
and no capabilities beyond what Caddy needs to bind ports 80 and 443. Keep your own copies under
`deploy/local/`, which git ignores.

## Calendar apps

Create an app password under Settings, then add a CalDAV account in the calendar app with:

| Field | Value |
|---|---|
| Server | `keduly.example.com` |
| User name | your account email |
| Password | the app password (`kdl_…`) |

## Backup

Everything is in the data directory. With the server stopped, copy the directory. With the server
running, take a consistent copy with SQLite's backup command instead of copying the files:

```sh
sqlite3 /var/lib/keduly/keduly.db ".backup '/backups/keduly.db'"
```

To restore, stop the server, put the copy back as `keduly.db` (remove any `keduly.db-wal` and
`keduly.db-shm` next to it) and start the server.

## Limits

Requests are rate-limited per client address (10 a minute on sign-in and sign-up, 300 a minute on
the rest of the API, 600 a minute on CalDAV), request bodies are capped at 1 MiB, and an account
can hold at most 20,000 items, 20,000 events, 200 projects and 50 tokens.
