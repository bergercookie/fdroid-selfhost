# Server

One container runs the Go server, which also drives `fdroid update`. It watches the repo folder and regenerates the signed index a few seconds after any APK is added, replaced or removed, whether via the API, `rsync` or a plain copy.

## Configuration

All settings are environment variables (set them in `.env` for Docker Compose).

| Variable | Default | Meaning |
|---|---|---|
| `PUBLIC_URL` | `http://localhost:8080` | How devices reach the server, no trailing slash. The repo address is `PUBLIC_URL/repo`. **Changing it later means re-adding the repo on every device.** |
| `UPLOAD_TOKEN` | *(empty)* | Bearer token for uploads and deletes. If empty, writes are disabled. |
| `REPO_NAME` | `My F-Droid Repo` | Shown in F-Droid. |
| `REPO_DESCRIPTION` | `Self-hosted F-Droid repository` | Shown in F-Droid. |
| `KEYSTORE_PASS` | generated | Password of the signing key. If unset, one is generated into `data/keystore.pass`. |
| `DATA_DIR` | `/data` | Where state lives (see below). |
| `LISTEN_ADDR` | `:8080` | Listen address inside the container. |
| `POLL_INTERVAL` | `5s` | How often the repo folder is checked for changes. |
| `MAX_UPLOAD_MB` | `500` | Upload size limit. |

## Data directory

```
data/
├── keystore.p12    repo signing key, created on first start
├── keystore.pass   its password (unless you set KEYSTORE_PASS)
├── config.yml      fdroidserver config, regenerated on every start
├── repo/           APKs and the generated index. This is what clients download.
└── metadata/       optional per-app descriptions and icons (fdroidserver format)
```

**Back up `keystore.p12` and its password.** Lose them and clients will reject your repo; everyone must remove and re-add it. APKs and the index can be regenerated, the key cannot. Never commit `data/`.

Only `repo/` is served. The key and config are never exposed.

## API

| Method and path | Auth | |
|---|---|---|
| `GET /` | | Web UI |
| `GET /repo/...` | | The F-Droid repo (what clients use) |
| `GET /api/info` | | Repo name, address, fingerprint, F-Droid link, server version |
| `GET /api/status` | | Result of the last index update |
| `GET /api/apks` | | List APKs |
| `PUT /api/apks/{name}.apk` | token | Upload an APK (replaces one with the same name) |
| `DELETE /api/apks/{name}.apk` | token | Delete an APK |
| `GET /healthz` | | Liveness |

Authenticate with `Authorization: Bearer <UPLOAD_TOKEN>`:

```bash
curl -H "Authorization: Bearer $UPLOAD_TOKEN" -T app.apk https://fdroid.example.com/api/apks/com.example.app_42.apk
```

Uploads must be zip files containing `AndroidManifest.xml`; anything else is rejected with `400`.

## HTTPS, VPN and Android

- F-Droid expects HTTPS for anything beyond a trusted LAN. The container serves plain HTTP; put a reverse proxy in front.
- **Tailscale**: `tailscale serve --https=443 localhost:8080` gives a valid certificate, with no public exposure.
- **WireGuard / LAN**: use Caddy or Traefik with a DNS-challenge certificate for a real domain pointing at a private IP. Avoid a private CA: Android apps ignore user-installed CAs by default.
- The server must be reachable from the phone when F-Droid refreshes, so connect the VPN first. Installed apps keep working when it isn't.
- Everyone with the upload token can add or delete APKs. Treat it like a password, and keep the upload path on the VPN too.

## Troubleshooting

- **Index not updating**: check the UI (it shows the last update result) or `docker compose logs fdroid`.
- **`permission denied` on `data/`**: create `data` yourself before the first start and set `PUID`/`PGID` in `.env` to your user (`id -u`, `id -g`).
- **Update fails for one APK**: an APK that is unsigned or corrupt makes `fdroid update` fail; delete it and the index rebuilds.
