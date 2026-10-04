# The image holds nothing but the static binary. Build it first:
#   GOOS=linux GOARCH=amd64 scripts/build.sh
#   docker build -f deploy/Dockerfile -t keduly .
# The binary embeds the web app, the time zone database and its SQLite engine,
# so it needs no shell, no CA bundle and no zoneinfo at runtime.
FROM scratch
COPY bin/keduly /keduly
EXPOSE 8080
VOLUME /data
ENTRYPOINT ["/keduly", "serve", "--addr", "0.0.0.0:8080", "--data", "/data"]
