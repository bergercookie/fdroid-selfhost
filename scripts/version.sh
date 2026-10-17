#!/usr/bin/env bash
# Version helpers used by the release workflows.
#
# Tags are written with a "v" prefix (v1.2.3, v1.3.3.post2). Versions never have
# one: that is what Docker tags and the APK use. A "v" on input is ignored.
#
#   git tag | scripts/version.sh nightly   next nightly version, e.g. 1.3.3.post2
#   scripts/version.sh code 1.3.3.post2    Android versionCode, e.g. 1030302
set -euo pipefail

# Reads tags on stdin (with or without the v prefix). A nightly bumps the minor version of the latest stable
# tag (1.2.3 -> 1.3.3) and appends .postN, where N counts the nightlies already
# released for that version.
nightly() {
	local tags latest major minor patch base last
	tags="$(sed -E 's/^v([0-9])/\1/')"
	latest="$(grep -E '^[0-9]+\.[0-9]+\.[0-9]+$' <<<"$tags" | sort -V | tail -n1 || true)"
	IFS=. read -r major minor patch <<<"${latest:-0.0.0}"
	base="${major}.$((minor + 1)).${patch}"
	last="$(grep -E "^${base//./\\.}\.post[0-9]+$" <<<"$tags" | sed 's/.*\.post//' | sort -n | tail -n1 || true)"
	echo "${base}.post$((${last:-0} + 1))"
}

# Android needs an integer that grows with every release. Each part of
# MAJOR.MINOR.PATCH[.postN] gets two digits, so 1.3.3.post2 becomes 01 03 03 02.
code() {
	local version="${1#v}" re='^([0-9]+)\.([0-9]+)\.([0-9]+)(\.post([0-9]+))?$'
	if [[ ! $version =~ $re ]]; then
		echo "invalid version: $version" >&2
		exit 1
	fi
	local parts=("${BASH_REMATCH[1]}" "${BASH_REMATCH[2]}" "${BASH_REMATCH[3]}" "${BASH_REMATCH[5]:-0}")
	local part
	for part in "${parts[@]}"; do
		if ((10#$part > 99)); then
			echo "version part too large (max 99): $version" >&2
			exit 1
		fi
	done
	echo $((((10#${parts[0]} * 100 + 10#${parts[1]}) * 100 + 10#${parts[2]}) * 100 + 10#${parts[3]}))
}

case "${1:-}" in
nightly) nightly ;;
code) code "${2:?usage: version.sh code VERSION}" ;;
*)
	echo "usage: version.sh nightly | code VERSION" >&2
	exit 2
	;;
esac
