# Android companion app

A minimal app (no dependencies) that you point at your server. It can:

- **Add the repository to F-Droid** with the right signing fingerprint, one tap.
- **Open the web UI** in your browser.

On first launch, enter your server URL (for example `https://fdroid.example.com`). It is remembered. F-Droid must be installed for the first button to work.

Each release attaches the APK to the [GitHub release](../../../releases). It is a normal APK, so you can also publish it through your own repo.

## Choosing the server URL

- **At runtime** (default): type it into the app.
- **At build time**: bake in a default, still editable in the app: `just apk https://fdroid.example.com`

## Building

Needs JDK 17, the Android SDK (platform 34) with `ANDROID_HOME` set, and Gradle 8.7 or newer. There is no Gradle wrapper; Android Studio or `gradle` both work.

```bash
just apk                              # -> dist/fdroid-selfhost-<version>.apk
just apk https://fdroid.example.com   # with a default server URL
```

## Signing

Without a key, builds are signed with the debug key. That is fine for trying it out but not for distribution. For real releases, provide a key through the environment (in CI: repository secrets):

| Variable | Secret in CI |
|---|---|
| `APP_KEYSTORE` (path to `.jks`) | `ANDROID_KEYSTORE_BASE64` (the file, base64 encoded) |
| `APP_KS_PASS` | `ANDROID_KEYSTORE_PASSWORD` |
| `APP_KEY_ALIAS` | `ANDROID_KEY_ALIAS` |
| `APP_KEY_PASS` | `ANDROID_KEY_PASSWORD` |

Create a key with `keytool -genkeypair -v -keystore release.jks -alias release -keyalg RSA -keysize 4096 -validity 10000`, and keep it safe: updates only work with the same key.

## Cleartext HTTP

The app allows plain `http://` servers because homelab servers often sit on a LAN or VPN without TLS. Use HTTPS where you can.
