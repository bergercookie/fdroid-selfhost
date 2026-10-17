# Publishing your apps

The repo only needs signed release APKs. [`scripts/publish-apk.sh`](../scripts/publish-apk.sh) builds one with Gradle and uploads it. Copy the script into your app's repository, or call it from CI.

## App signing

Two different keys are involved. Don't mix them up:

| Key | Signs | Lives |
|---|---|---|
| Repo key | the repo index | on the server (`data/keystore.p12`) |
| **App key** | each APK | with you; one per app, or shared |

- Android refuses to update an app signed with a different key, so **keep the app key forever** and back it up.
- Never publish debug builds: the debug key differs per machine, so updates would break.
- Bump `versionCode` on every release. F-Droid only offers an update when it increases. Keep `applicationId` fixed.

Gradle signing config, reading secrets from the environment:

```kotlin
signingConfigs {
    create("release") {
        storeFile = file(System.getenv("APP_KEYSTORE") ?: "release.jks")
        storePassword = System.getenv("APP_KS_PASS")
        keyAlias = System.getenv("APP_KEY_ALIAS")
        keyPassword = System.getenv("APP_KEY_PASS")
    }
}
buildTypes { release { signingConfig = signingConfigs.getByName("release") } }
```

## Using the script

```bash
cp /path/to/fdroid-selfhost/scripts/publish-apk.sh scripts/
cp /path/to/fdroid-selfhost/.fdroid-publish.env.example .fdroid-publish.env   # edit; never commit
scripts/publish-apk.sh --dry-run
scripts/publish-apk.sh
```

`FDROID_TARGET` chooses how it publishes:

| Target | Method |
|---|---|
| `https://fdroid.example.com` | API upload with `UPLOAD_TOKEN` (default choice) |
| `user@host:/srv/fdroid/data/repo` | `rsync` over SSH |
| `/srv/fdroid/data/repo` | plain copy (local disk or NFS) |

The script verifies the APK signature, names the file `<applicationId>_<versionCode>.apk` so releases never overwrite each other, then uploads. The server refreshes its index within seconds. Run `scripts/publish-apk.sh --help` for all options.

Because the script only needs `FDROID_TARGET` and `UPLOAD_TOKEN`, anyone can point it at their own server. In CI, set those as secrets; the runner must be able to reach the server (self-hosted runner or a VPN step).

## App listing

New apps get a stub in `data/metadata/<applicationId>.yml`. Edit it for a proper name, summary and description; icons and screenshots go in `data/metadata/<applicationId>/en-US/` (see the [fdroidserver docs](https://f-droid.org/docs/All_About_Descriptions_Graphics_and_Screenshots/)).
