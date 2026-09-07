#!/bin/sh
set -eu

fail() {
	printf 'ERROR: %s\n' "$*" >&2
	exit 1
}

command -v curl >/dev/null 2>&1 || fail "curl is required"
command -v jq >/dev/null 2>&1 || fail "jq is required"
command -v minisign >/dev/null 2>&1 || fail "minisign is required"
: "${PUBLIC_URL:?PUBLIC_URL is required}"
: "${BACKEND_IMAGE:?BACKEND_IMAGE is required}"
: "${FRONTEND_IMAGE:?FRONTEND_IMAGE is required}"
: "${BACKUP_IMAGE:?BACKUP_IMAGE is required}"
: "${RESTORE_EVIDENCE_FILE:?RESTORE_EVIDENCE_FILE is required}"
: "${RESTORE_EVIDENCE_SIGNATURE:?RESTORE_EVIDENCE_SIGNATURE is required}"
: "${BACKUP_SIGNING_PUBLIC_KEY_FILE:?BACKUP_SIGNING_PUBLIC_KEY_FILE is required}"
: "${MAX_RESTORE_EVIDENCE_AGE_HOURS:=192}"

case "$PUBLIC_URL" in
https://*) ;;
*) fail "PUBLIC_URL must use HTTPS" ;;
esac
case "$MAX_RESTORE_EVIDENCE_AGE_HOURS" in
*[!0-9]* | "" | 0) fail "MAX_RESTORE_EVIDENCE_AGE_HOURS must be a positive integer" ;;
esac

for image in "$BACKEND_IMAGE" "$FRONTEND_IMAGE" "$BACKUP_IMAGE"; do
	printf '%s' "$image" | grep -Eq '@sha256:[a-f0-9]{64}$' || fail "image is not pinned by digest: $image"
done

[ -r "$RESTORE_EVIDENCE_FILE" ] || fail "restore evidence is not readable"
[ -r "$RESTORE_EVIDENCE_SIGNATURE" ] || fail "restore evidence signature is not readable"
[ -r "$BACKUP_SIGNING_PUBLIC_KEY_FILE" ] || fail "backup signing public key is not readable"
minisign -V -q -p "$BACKUP_SIGNING_PUBLIC_KEY_FILE" -m "$RESTORE_EVIDENCE_FILE" -x "$RESTORE_EVIDENCE_SIGNATURE" || fail "restore evidence signature is invalid"
jq -e '.status == "passed" and (.completed_at | type == "string")' "$RESTORE_EVIDENCE_FILE" >/dev/null || fail "restore evidence is invalid"
completed_at=$(jq -r '.completed_at' "$RESTORE_EVIDENCE_FILE")
completed_epoch=$(date -u -d "$completed_at" +%s 2>/dev/null || fail "restore evidence timestamp is invalid")
now_epoch=$(date -u +%s)
age_hours=$(((now_epoch - completed_epoch) / 3600))
[ "$age_hours" -ge 0 ] || fail "restore evidence is dated in the future"
[ "$age_hours" -le "$MAX_RESTORE_EVIDENCE_AGE_HOURS" ] || fail "restore evidence is stale (${age_hours}h)"

if ! curl --fail --silent --show-error --max-time 10 "${PUBLIC_URL%/}/health/ready" >/dev/null; then
	[ "${INITIAL_DEPLOY_CONFIRM:-}" = "initial-deploy" ] || fail "current production readiness check failed; set INITIAL_DEPLOY_CONFIRM=initial-deploy only for first deployment"
	printf 'Current production endpoint is absent; explicit initial-deploy confirmation accepted.\n'
fi
printf 'Release preflight passed; restore evidence age=%sh.\n' "$age_hours"
