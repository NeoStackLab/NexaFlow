#!/bin/sh
set -eu

fail() {
	printf 'ERROR: %s\n' "$*" >&2
	exit 1
}

provider=${1:-}
: "${BACKUP_BUCKET:?BACKUP_BUCKET is required}"
: "${BACKUP_RETENTION_DAYS:=35}"
: "${BACKUP_PREFIX:=database/}"

case "$BACKUP_RETENTION_DAYS" in
*[!0-9]* | "") fail "BACKUP_RETENTION_DAYS must be a positive integer" ;;
esac
[ "$BACKUP_RETENTION_DAYS" -ge 1 ] || fail "BACKUP_RETENTION_DAYS must be positive"

case "$provider" in
s3)
	command -v aws >/dev/null 2>&1 || fail "aws CLI is required"
	aws s3api put-bucket-versioning \
		--bucket "$BACKUP_BUCKET" \
		--versioning-configuration Status=Enabled
	status=$(aws s3api get-bucket-versioning --bucket "$BACKUP_BUCKET" --query Status --output text)
	[ "$status" = "Enabled" ] || fail "S3 bucket versioning did not become enabled"
	printf 'S3 versioning enabled for %s; configure lifecycle retention for prefix %s separately.\n' "$BACKUP_BUCKET" "$BACKUP_PREFIX"
	;;
r2)
	command -v curl >/dev/null 2>&1 || fail "curl is required"
	command -v jq >/dev/null 2>&1 || fail "jq is required"
	: "${CLOUDFLARE_ACCOUNT_ID:?CLOUDFLARE_ACCOUNT_ID is required}"
	: "${CLOUDFLARE_API_TOKEN:?CLOUDFLARE_API_TOKEN is required}"
	api="https://api.cloudflare.com/client/v4/accounts/$CLOUDFLARE_ACCOUNT_ID/r2/buckets/$BACKUP_BUCKET/lock"
	work_directory=$(mktemp -d "${TMPDIR:-/tmp}/nexaflow-r2-lock.XXXXXX")
	cleanup() {
		case "${work_directory:-}" in
		"${TMPDIR:-/tmp}"/nexaflow-r2-lock.*) rm -rf -- "$work_directory" ;;
		*) fail "refusing to remove unexpected temporary directory" ;;
		esac
	}
	trap cleanup EXIT HUP INT TERM
	curl --fail-with-body --silent --show-error "$api" \
		-H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" >"$work_directory/current.json"
	seconds=$((BACKUP_RETENTION_DAYS * 86400))
	jq \
		--arg prefix "$BACKUP_PREFIX" \
		--argjson seconds "$seconds" \
		'{rules: ((.result.rules // []) | map(select(.id != "nexaflow-backups-retention")) + [{id:"nexaflow-backups-retention",enabled:true,prefix:$prefix,condition:{type:"Age",maxAgeSeconds:$seconds}}])}' \
		"$work_directory/current.json" >"$work_directory/update.json"
	curl --fail-with-body --silent --show-error -X PUT "$api" \
		-H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" \
		-H "Content-Type: application/json" \
		--data-binary "@$work_directory/update.json" >"$work_directory/result.json"
	jq -e '.success == true' "$work_directory/result.json" >/dev/null || fail "R2 bucket lock update failed"
	printf 'R2 retention lock set for %s/%s (%s days).\n' "$BACKUP_BUCKET" "$BACKUP_PREFIX" "$BACKUP_RETENTION_DAYS"
	;;
*) fail "usage: configure-object-retention.sh <s3|r2>" ;;
esac
