#!/usr/bin/env bash
# End-to-end test: start the real container image and exercise the HTTP API.
# Usage: e2e/run.sh IMAGE
set -euo pipefail

image="${1:?usage: e2e/run.sh IMAGE}"
port="${E2E_PORT:-18080}"
base="http://127.0.0.1:${port}"
token="e2e-token"
name="fdroid-selfhost-e2e-$$"
data="$(mktemp -d)"

cleanup() {
	docker logs "$name" 2>&1 | tail -n 30 || true
	docker rm -f "$name" >/dev/null 2>&1 || true
	rm -rf "$data"
}
trap cleanup EXIT

fail() {
	echo "FAIL: $*" >&2
	exit 1
}

expect_status() {
	local want="$1" got
	shift
	got="$(curl -s -o /dev/null -w '%{http_code}' "$@")"
	[[ $got == "$want" ]] || fail "expected HTTP $want, got $got: curl $*"
	echo "ok   HTTP $want: ${*: -1}"
}

docker run -d --name "$name" --user "$(id -u):$(id -g)" \
	-e PUBLIC_URL="$base" -e UPLOAD_TOKEN="$token" \
	-p "127.0.0.1:${port}:8080" -v "$data:/data" "$image" >/dev/null

echo "Waiting for the server..."
for _ in $(seq 1 60); do
	curl -sf "$base/healthz" >/dev/null && break
	sleep 1
done
curl -sf "$base/healthz" >/dev/null || fail "server did not become healthy"

info="$(curl -sf "$base/api/info")"
grep -Eq '"fingerprint":"[0-9A-F]{64}"' <<<"$info" || fail "bad fingerprint in $info"
echo "ok   /api/info has a fingerprint"

expect_status 401 -X PUT --data-binary 'x' "$base/api/apks/a.apk"
expect_status 400 -X PUT -H "Authorization: Bearer $token" --data-binary 'not an apk' "$base/api/apks/a.apk"

echo "Waiting for fdroidserver to build the first index..."
for _ in $(seq 1 120); do
	grep -q '"state":"ok"' < <(curl -sf "$base/api/status") && break
	sleep 1
done
status="$(curl -sf "$base/api/status")"
grep -q '"state":"ok"' <<<"$status" || fail "index was not built: $status"
echo "ok   fdroid update succeeded"

expect_status 200 "$base/repo/index-v1.jar"
expect_status 200 "$base/"
echo "All end-to-end checks passed."
