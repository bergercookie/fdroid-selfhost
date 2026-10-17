# fdroid-selfhost

A self-hosted [F-Droid](https://f-droid.org) repository for your own Android apps, with a web UI and upload API, in a single Docker container. Built for homelabs and VPNs: nothing needs to be on the public internet.

- **Server**: a small Go service that runs [fdroidserver](https://f-droid.org/docs/Setup_an_F-Droid_App_Repo/), signs the repo index, and serves the repo.
- **Web UI**: shows the repo address and fingerprint, lists APKs, uploads and deletes them.
- **Upload API**: `PUT` an APK over HTTPS with a bearer token. No SSH needed.
- **`publish-apk.sh`**: a script your app repos call to build and publish a release.

## Quick start

```bash
git clone <this repo> && cd fdroid-selfhost
cp .env.example .env            # set PUBLIC_URL and UPLOAD_TOKEN (openssl rand -hex 24)
mkdir -p data                   # holds the signing key: back it up
docker compose up -d --build
```

Open `PUBLIC_URL` in a browser: you get the web UI. Put a TLS reverse proxy in front of port 8080 (Caddy, Traefik, `tailscale serve`, ...).

To add the repo on a phone, open the UI there and tap **Add this repository**. To publish an app, see [docs/publishing.md](docs/publishing.md).

Prefer a released image to building? Set `FDROID_IMAGE=ghcr.io/<owner>/fdroid-selfhost:<version>` in `.env` and run `docker compose pull && docker compose up -d`.

## Documentation

| | |
|---|---|
| [docs/server.md](docs/server.md) | Configuration, API, HTTPS and VPN notes, backups |
| [docs/publishing.md](docs/publishing.md) | Getting your own apps into the repo; app signing keys |
| [docs/development.md](docs/development.md) | `just` recipes, CI, and how releases are made |

## Layout

```
cmd/, internal/, web/   Go server and its embedded web UI
scripts/                publish-apk.sh (for app repos), release version helper
e2e/                    End-to-end test against the Docker image
Dockerfile, docker-compose.yml
justfile                Single entry point: build, lint, test, release checks
```

Run `just` to list everything you can do.
