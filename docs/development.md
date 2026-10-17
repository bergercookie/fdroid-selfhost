# Development

Everything goes through the [`justfile`](../justfile). Run `just` for the list. CI and the pre-commit hooks call the same recipes, so what passes locally passes in CI.

## Tools

Required: [`just`](https://github.com/casey/just), Go (the version in `go.mod`; newer Go downloads it automatically), Docker, `shellcheck`, [`pre-commit`](https://pre-commit.com).
For the Android app: JDK 17, the Android SDK and Gradle (see [android.md](android.md)).

The Go-based linters (golangci-lint, govulncheck, shfmt, actionlint) run through `go run` at pinned versions, so there is nothing else to install. Update a version by editing the top of the justfile.

```bash
just setup    # install the pre-commit hooks
```

## Recipes

| Recipe | What it does |
|---|---|
| `just build` | Build the server binary to `bin/` |
| `just up` | Run the server with Docker Compose |
| `just apk [url]` | Build the Android app |
| `just fmt` | Format Go and shell code |
| `just lint` | `go vet`, gofmt, golangci-lint, shellcheck, shfmt, hadolint, actionlint |
| `just test` | Go unit tests (with `-race`) and the version-script tests |
| `just vuln` | `govulncheck` |
| `just e2e` | Build the Docker image and test it end to end |
| `just ci` | `lint`, `test`, `vuln` and `e2e`: everything CI checks |

Pre-commit runs the relevant lint recipe when matching files change (`go` files run `just lint-go`, and so on).

## CI

[`ci.yml`](../.github/workflows/ci.yml) runs on pushes to `main` and on pull requests, one job per recipe: `lint`, `test`, `vuln`, `e2e`, and an `android` build.

## Releases

Tags look like `v1.2.3`; the version itself (image tags, APK) has no `v`. Images and releases are produced by [`release.yml`](../.github/workflows/release.yml).

**Stable release**: push a tag.

```bash
git tag v1.2.3 && git push origin v1.2.3
```

It runs `just ci`, builds the APK and a multi-arch (amd64/arm64) server image, pushes the image to `ghcr.io/<owner>/fdroid-selfhost` (tags `1.2.3` and `latest`; the release is named after the tag, `v1.2.3`), and creates a GitHub release with the APK attached.

**Nightly release**: [`nightly.yml`](../.github/workflows/nightly.yml) runs every night at 02:17 UTC, and on demand from the Actions tab (*Run workflow*; tick *force* to release even when nothing changed). It releases the tip of `main` unless that commit already has a tag.

The version is the next minor after the latest stable tag, with the patch number kept, plus `.postN`. The git tag is the version with a `v`:

| Latest stable tag | Nightlies (version, tag) |
|---|---|
| `v1.2.3` | `1.3.3.post1` (`v1.3.3.post1`), `1.3.3.post2`, ... |
| `v1.3.0` (after releasing) | `1.4.0.post1`, ... |

Nightlies are marked as pre-releases and the image is also tagged `nightly`. The logic lives in [`scripts/version.sh`](../scripts/version.sh) and is tested by `just test`.

Android `versionCode` is derived from the version (`1.3.3.post2` becomes `1030302`), so each part must stay below 100.

**Setup for releases** (once per repository):

1. Add the Android signing secrets from [android.md](android.md#signing). Without them, release APKs use the debug key and the workflow prints a warning.
2. After the first image push, GitHub marks the package private. Make it public or grant access under *Packages* if you want others to pull it.
3. Under *Settings → Actions → General*, allow workflows to create releases (the workflows request the permissions they need).
