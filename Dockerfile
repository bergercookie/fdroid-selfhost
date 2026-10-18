# syntax=docker/dockerfile:1
FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
COPY web ./web
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" \
    -o /out/fdroid-selfhost ./cmd/fdroid-selfhost

FROM debian:bookworm-slim
# fdroidserver comes from PyPI: Debian's package drags in a C++ toolchain,
# LLVM and scipy that `fdroid update` never uses. It does need the JDK (jar,
# jarsigner, keytool) to build and sign the index. Bump the version on purpose.
ARG FDROIDSERVER_VERSION=2.4.5
RUN apt-get update \
 && apt-get install -y --no-install-recommends python3 python3-venv apksigner aapt default-jdk-headless ca-certificates \
 && rm -rf /var/lib/apt/lists/* \
 && python3 -m venv /opt/fdroidserver \
 && /opt/fdroidserver/bin/pip install --no-cache-dir "fdroidserver==${FDROIDSERVER_VERSION}" \
 && mkdir /data && chown 1000:1000 /data
COPY --from=build /out/fdroid-selfhost /usr/local/bin/fdroid-selfhost

# Any uid can run the image (see `user:` in docker-compose.yml); it only writes to /data.
# GitPython is imported by fdroidserver but git itself is never needed here.
ENV DATA_DIR=/data HOME=/tmp PATH=/opt/fdroidserver/bin:$PATH GIT_PYTHON_REFRESH=quiet
VOLUME /data
EXPOSE 8080
USER 1000:1000
HEALTHCHECK --interval=30s --timeout=5s --start-period=60s \
    CMD ["fdroid-selfhost", "healthcheck"]
ENTRYPOINT ["fdroid-selfhost"]
