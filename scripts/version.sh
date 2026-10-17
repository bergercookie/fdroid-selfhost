#!/usr/bin/env bash
# Version helpers used by the release workflows.
#
# Tags are written with a "v" prefix (v1.2.3, v1.3.3.post2). Versions never have
# one: that is what Docker image tags use. A "v" on input is ignored.
#
#   git tag | scripts/version.sh nightly   next nightly version, e.g. 1.3.3.post2
set -euo pipefail

# Reads tags on stdin (with or without the v prefix). A nightly bumps the minor
# version of the latest stable tag (1.2.3 -> 1.3.3) and appends .postN, where N
# counts the nightlies already released for that version.
nightly() {
	local tags latest major minor patch base last
	tags="$(sed -E 's/^v([0-9])/\1/')"
	latest="$(grep -E '^[0-9]+\.[0-9]+\.[0-9]+$' <<<"$tags" | sort -V | tail -n1 || true)"
	IFS=. read -r major minor patch <<<"${latest:-0.0.0}"
	base="${major}.$((minor + 1)).${patch}"
	last="$(grep -E "^${base//./\\.}\.post[0-9]+$" <<<"$tags" | sed 's/.*\.post//' | sort -n | tail -n1 || true)"
	echo "${base}.post$((${last:-0} + 1))"
}

case "${1:-}" in
nightly) nightly ;;
*)
	echo "usage: git tag | version.sh nightly" >&2
	exit 2
	;;
esac
