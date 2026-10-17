# syntax=docker/dockerfile:1
FROM golang:1.27-bookworm AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
COPY web ./web
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" \
    -o /out/fdroid-selfhost ./cmd/fdroid-selfhost

FROM debian:bookworm-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends fdroidserver apksigner aapt default-jre-headless ca-certificates \
 && rm -rf /var/lib/apt/lists/* \
 && mkdir /data && chown 1000:1000 /data
COPY --from=build /out/fdroid-selfhost /usr/local/bin/fdroid-selfhost

# Any uid can run the image (see `user:` in docker-compose.yml); it only writes to /data.
ENV DATA_DIR=/data HOME=/tmp
VOLUME /data
EXPOSE 8080
USER 1000:1000
HEALTHCHECK --interval=30s --timeout=5s --start-period=60s \
    CMD ["fdroid-selfhost", "healthcheck"]
ENTRYPOINT ["fdroid-selfhost"]
