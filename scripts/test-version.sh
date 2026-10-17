#!/usr/bin/env bash
# Tests for version.sh. Run with `just test`.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")"
failures=0

check() {
	local name="$1" want="$2" got="$3"
	if [[ $got == "$want" ]]; then
		echo "ok   $name"
	else
		echo "FAIL $name: got '$got', want '$want'"
		failures=$((failures + 1))
	fi
}

nightly() { printf '%s\n' "$@" | ./version.sh nightly; }

check "no tags at all" "0.1.0.post1" "$(nightly)"
check "bumps minor, keeps patch" "1.3.3.post1" "$(nightly 1.2.3)"
check "second nightly of the day" "1.3.3.post2" "$(nightly 1.2.3 1.3.3.post1)"
check "counts past 9" "1.3.3.post10" "$(nightly 1.2.3 1.3.3.post9 1.3.3.post1)"
check "uses the highest stable tag" "1.11.0.post1" "$(nightly 1.2.0 1.10.0 1.9.5)"
check "stable release resets the counter" "1.4.0.post1" "$(nightly 1.2.3 1.3.0 1.3.3.post1)"
check "ignores unrelated tags" "1.3.3.post1" "$(nightly 1.2.3 v9.9.9 foo 2.0.0-rc1)"

check "code stable" "1030300" "$(./version.sh code 1.3.3)"
check "code nightly" "1030302" "$(./version.sh code 1.3.3.post2)"
check "code leading zeros" "1080900" "$(./version.sh code 1.08.09)"
if ./version.sh code 1.2 2>/dev/null; then
	check "code rejects malformed versions" "failure" "success"
else
	check "code rejects malformed versions" "failure" "failure"
fi

exit $((failures > 0))
