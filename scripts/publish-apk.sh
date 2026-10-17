#!/usr/bin/env bash
# Publish an Android app to your fdroid-selfhost repository.
#
# Copy this script into an app's repository (or call it from CI). Everything
# that differs per server lives in configuration, so nothing here is edited.
# See docs/publishing.md.
set -euo pipefail

usage() {
	cat <<'USAGE'
Usage: publish-apk.sh [--apk PATH] [--no-build] [--dry-run]

  --apk PATH   Publish this APK instead of building one
  --no-build   Don't run Gradle; use the newest APK matching APK_GLOB
  --dry-run    Show what would happen without uploading

Configuration (environment, or a .fdroid-publish.env file next to the app):
  FDROID_TARGET   Required. Where to publish:
                    https://fdroid.example.com    upload through the API
                    user@host:/path/to/data/repo  rsync over SSH
                    /path/to/data/repo            plain copy
  UPLOAD_TOKEN    Required for https:// targets (the server's UPLOAD_TOKEN)
  GRADLE_TASK     Default: assembleRelease
  APK_GLOB        Default: app/build/outputs/apk/release/*-release.apk
  SSH_OPTS        Extra ssh options for rsync targets, e.g. "-p 2222"
USAGE
}

build=1 dry=0 apk=""
while [[ $# -gt 0 ]]; do
	case "$1" in
	--apk)
		apk="${2:?--apk needs a path}"
		build=0
		shift
		;;
	--no-build) build=0 ;;
	--dry-run) dry=1 ;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		usage >&2
		exit 2
		;;
	esac
	shift
done

if [[ -f .fdroid-publish.env ]]; then
	set -a
	# shellcheck disable=SC1091
	source .fdroid-publish.env
	set +a
fi
: "${FDROID_TARGET:?FDROID_TARGET is not set (see --help)}"
GRADLE_TASK="${GRADLE_TASK:-assembleRelease}"
APK_GLOB="${APK_GLOB:-app/build/outputs/apk/release/*-release.apk}"
SSH_OPTS="${SSH_OPTS:-}"

if ((build)); then
	./gradlew "$GRADLE_TASK"
fi

if [[ -z $apk ]]; then
	# shellcheck disable=SC2086,SC2012 # APK_GLOB is meant to expand; Gradle output names are plain
	apk="$(ls -t $APK_GLOB 2>/dev/null | head -n1 || true)"
fi
[[ -f $apk ]] || {
	echo "No APK found (glob: $APK_GLOB)" >&2
	exit 1
}

# Refuse APKs that would break updates for installed copies.
if command -v apksigner >/dev/null; then
	apksigner verify "$apk" >/dev/null || {
		echo "APK signature is invalid: $apk" >&2
		exit 1
	}
else
	echo "warning: apksigner not found, skipping signature check" >&2
fi

# Name the file <applicationId>_<versionCode>.apk so releases never overwrite
# each other. fdroidserver reads the real values from the APK itself.
name="$(basename "$apk")"
if aapt="$(command -v aapt || command -v aapt2)"; then
	badging="$("$aapt" dump badging "$apk")"
	package="$(sed -n "s/^package: name='\([^']*\)'.*/\1/p" <<<"$badging")"
	code="$(sed -n "s/.*versionCode='\([^']*\)'.*/\1/p" <<<"$badging" | head -n1)"
	[[ -n $package && -n $code ]] && name="${package}_${code}.apk"
fi
echo "Publishing $apk as $name to $FDROID_TARGET"

case "$FDROID_TARGET" in
http://* | https://*)
	: "${UPLOAD_TOKEN:?UPLOAD_TOKEN is not set (needed for https targets)}"
	if ((dry)); then
		echo "[dry-run] PUT ${FDROID_TARGET%/}/api/apks/$name"
	else
		curl --fail-with-body --show-error --progress-bar \
			-H "Authorization: Bearer $UPLOAD_TOKEN" \
			-T "$apk" "${FDROID_TARGET%/}/api/apks/$name"
		echo
	fi
	;;
*:*)
	rsync_args=(-av --progress -e "ssh $SSH_OPTS")
	((dry)) && rsync_args+=(--dry-run)
	rsync "${rsync_args[@]}" "$apk" "$FDROID_TARGET/$name"
	;;
*)
	if ((dry)); then
		echo "[dry-run] cp $apk $FDROID_TARGET/$name"
	else
		cp "$apk" "$FDROID_TARGET/$name"
	fi
	;;
esac
echo "Done. The server refreshes its index within a few seconds."
