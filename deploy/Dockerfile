# The image holds nothing but the static binary and an empty data directory.
# Build the binary first:
#   GOOS=linux GOARCH=amd64 scripts/build.sh
#   docker build -f deploy/Dockerfile -t keduly .
# The binary embeds the web app, the time zone database and its SQLite engine,
# so it needs no shell, no CA bundle and no zoneinfo at runtime.
#
# The server runs as an unprivileged user. An empty image cannot create a
# directory, so /data comes from a throwaway stage: a new volume mounted there
# inherits its owner. A bind-mounted directory has to be owned by 65534 on the host.
FROM busybox AS data
RUN install -d -o 65534 -g 65534 -m 700 /data

FROM scratch
COPY bin/keduly /keduly
COPY --from=data --chown=65534:65534 --chmod=700 /data /data
USER 65534:65534
EXPOSE 8080
VOLUME /data
ENTRYPOINT ["/keduly", "serve", "--addr", "0.0.0.0:8080", "--data", "/data"]
