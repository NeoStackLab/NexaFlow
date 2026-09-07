#!/bin/sh
set -eu

umask 077

log() {
	printf '%s %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*" >&2
}

fail() {
	log "ERROR: $*"
	exit 1
}

require_command() {
	command -v "$1" >/dev/null 2>&1 || fail "required command is missing: $1"
}

require_positive_integer() {
	value=$1
	name=$2
	case "$value" in
	*[!0-9]* | "") fail "$name must be a positive integer" ;;
	esac
	[ "$value" -ge 1 ] || fail "$name must be a positive integer"
}

read_secret() {
	variable_name=$1
	file_variable_name=$2
	eval "value=\${$variable_name:-}"
	eval "file_path=\${$file_variable_name:-}"
	if [ -n "$file_path" ]; then
		[ -r "$file_path" ] || fail "$file_variable_name is not readable"
		awk 'NR > 1 { exit 1 } END { if (NR == 0) exit 1 }' "$file_path" || fail "$file_variable_name must contain exactly one line"
		value=$(tr -d '\r\n' <"$file_path")
	fi
	[ -n "$value" ] || fail "$variable_name or $file_variable_name is required"
	export "$variable_name=$value"
}

prepare_database_environment() {
	: "${PGHOST:?PGHOST is required}"
	: "${PGPORT:=5432}"
	: "${PGDATABASE:?PGDATABASE is required}"
	: "${PGUSER:?PGUSER is required}"
	read_secret PGPASSWORD PGPASSWORD_FILE
}

prepare_backup_store() {
	if [ -n "${BACKUP_S3_ACCESS_KEY_ID_FILE:-}" ]; then
		read_secret AWS_ACCESS_KEY_ID BACKUP_S3_ACCESS_KEY_ID_FILE
		read_secret AWS_SECRET_ACCESS_KEY BACKUP_S3_SECRET_ACCESS_KEY_FILE
	fi
	export AWS_DEFAULT_REGION="${BACKUP_S3_REGION:-us-east-1}"
}

aws_copy() {
	source_path=$1
	destination_path=$2
	if [ -n "${BACKUP_S3_ENDPOINT:-}" ]; then
		aws --endpoint-url "$BACKUP_S3_ENDPOINT" s3 cp --only-show-errors "$source_path" "$destination_path"
	else
		aws s3 cp --only-show-errors "$source_path" "$destination_path"
	fi
}

aws_api() {
	if [ -n "${BACKUP_S3_ENDPOINT:-}" ]; then
		aws --endpoint-url "$BACKUP_S3_ENDPOINT" s3api "$@"
	else
		aws s3api "$@"
	fi
}

fetch() {
	source_path=$1
	destination_path=$2
	case "$source_path" in
	s3://*)
		prepare_backup_store
		aws_copy "$source_path" "$destination_path"
		;;
	file://*) cp "${source_path#file://}" "$destination_path" ;;
	*) cp "$source_path" "$destination_path" ;;
	esac
}

new_work_directory() {
	work_directory=$(mktemp -d "${TMPDIR:-/tmp}/nexaflow-recovery.XXXXXX")
	trap cleanup_work_directory EXIT HUP INT TERM
}

cleanup_work_directory() {
	case "${work_directory:-}" in
	"${TMPDIR:-/tmp}"/nexaflow-recovery.*) rm -rf -- "$work_directory" ;;
	"") ;;
	*) log "refusing to remove unexpected work directory: $work_directory" ;;
	esac
}

create_backup() {
	require_command pg_dump
	require_command pg_restore
	require_command age
	require_command sha256sum
	require_command jq
	require_command minisign
	prepare_database_environment
	read_secret AGE_RECIPIENT AGE_RECIPIENT_FILE
	retention_days=${BACKUP_RETENTION_DAYS:-35}
	require_positive_integer "$retention_days" BACKUP_RETENTION_DAYS
	: "${BACKUP_SIGNING_PRIVATE_KEY_FILE:?BACKUP_SIGNING_PRIVATE_KEY_FILE is required}"
	[ -r "$BACKUP_SIGNING_PRIVATE_KEY_FILE" ] || fail "backup signing key is not readable"
	new_work_directory

	timestamp=$(date -u +%Y%m%dT%H%M%SZ)
	archive_name="nexaflow-${PGDATABASE}-${timestamp}.dump.age"
	dump_path="$work_directory/database.dump"
	encrypted_path="$work_directory/$archive_name"
	manifest_path="$work_directory/$archive_name.json"
	signature_path="$manifest_path.minisig"

	log "creating PostgreSQL custom-format archive"
	pg_dump --format=custom --compress=9 --no-owner --no-acl --file="$dump_path"
	pg_restore --list "$dump_path" >/dev/null
	age --recipient "$AGE_RECIPIENT" --output "$encrypted_path" "$dump_path"
	rm -f "$dump_path"

	checksum=$(sha256sum "$encrypted_path" | awk '{print $1}')
	size_bytes=$(wc -c <"$encrypted_path" | tr -d ' ')
	postgres_version=$(psql --no-psqlrc --tuples-only --no-align --command='SHOW server_version;' | head -n 1)

	if [ -n "${BACKUP_S3_URI:-}" ]; then
		require_command aws
		prepare_backup_store
		base_uri=${BACKUP_S3_URI%/}
		archive_uri="$base_uri/$timestamp/$archive_name"
		manifest_uri="$archive_uri.json"
	else
		: "${BACKUP_LOCAL_DIR:?BACKUP_S3_URI or BACKUP_LOCAL_DIR is required}"
		mkdir -p "$BACKUP_LOCAL_DIR/$timestamp"
		archive_uri="file://$BACKUP_LOCAL_DIR/$timestamp/$archive_name"
		manifest_uri="$archive_uri.json"
	fi

	jq -n \
		--arg created_at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
		--arg database "$PGDATABASE" \
		--arg postgres_version "$postgres_version" \
		--arg application_version "${NEXAFLOW_VERSION:-unknown}" \
		--argjson retention_days "$retention_days" \
		--arg archive_uri "$archive_uri" \
		--arg sha256 "$checksum" \
		--argjson size_bytes "$size_bytes" \
		'{schema_version:1,created_at:$created_at,database:$database,postgres_version:$postgres_version,application_version:$application_version,archive_uri:$archive_uri,sha256:$sha256,size_bytes:$size_bytes,encryption:"age",retention_days:$retention_days}' \
		>"$manifest_path"
	minisign -S -q -s "$BACKUP_SIGNING_PRIVATE_KEY_FILE" -m "$manifest_path" -x "$signature_path"

	if [ -n "${BACKUP_S3_URI:-}" ]; then
		aws_copy "$encrypted_path" "$archive_uri"
		aws_copy "$signature_path" "$manifest_uri.minisig"
		aws_copy "$manifest_path" "$manifest_uri"
	else
		cp "$encrypted_path" "${archive_uri#file://}"
		cp "$signature_path" "${manifest_uri#file://}.minisig"
		cp "$manifest_path" "${manifest_uri#file://}"
	fi

	log "backup verified and stored"
	printf 'archive_uri=%s\nmanifest_uri=%s\nsha256=%s\n' "$archive_uri" "$manifest_uri" "$checksum"
}

load_archive() {
	manifest_source=$1
	require_command age
	require_command jq
	require_command sha256sum
	require_command pg_restore
	require_command minisign
	: "${AGE_IDENTITY_FILE:?AGE_IDENTITY_FILE is required}"
	: "${BACKUP_SIGNING_PUBLIC_KEY_FILE:?BACKUP_SIGNING_PUBLIC_KEY_FILE is required}"
	[ -r "$AGE_IDENTITY_FILE" ] || fail "age identity file is not readable"
	[ -r "$BACKUP_SIGNING_PUBLIC_KEY_FILE" ] || fail "backup signing public key is not readable"
	new_work_directory

	manifest_path="$work_directory/manifest.json"
	signature_path="$work_directory/manifest.json.minisig"
	encrypted_path="$work_directory/database.dump.age"
	dump_path="$work_directory/database.dump"
	fetch "$manifest_source" "$manifest_path"
	fetch "$manifest_source.minisig" "$signature_path"
	minisign -V -q -p "$BACKUP_SIGNING_PUBLIC_KEY_FILE" -m "$manifest_path" -x "$signature_path" || fail "backup manifest signature is invalid"
	jq -e '.schema_version == 1 and .encryption == "age" and (.archive_uri | type == "string") and (.sha256 | test("^[a-f0-9]{64}$"))' "$manifest_path" >/dev/null || fail "backup manifest is invalid"
	archive_uri=$(jq -r '.archive_uri' "$manifest_path")
	expected_checksum=$(jq -r '.sha256' "$manifest_path")
	fetch "$archive_uri" "$encrypted_path"
	actual_checksum=$(sha256sum "$encrypted_path" | awk '{print $1}')
	[ "$actual_checksum" = "$expected_checksum" ] || fail "encrypted archive checksum mismatch"
	age --decrypt --identity "$AGE_IDENTITY_FILE" --output "$dump_path" "$encrypted_path"
	pg_restore --list "$dump_path" >/dev/null || fail "PostgreSQL archive table of contents is invalid"
}

create_restore_database() {
	target_database=$1
	case "$target_database" in
	*[!A-Za-z0-9_]*) fail "restore target contains invalid characters" ;;
	esac
	read_secret PGADMIN_PASSWORD PGADMIN_PASSWORD_FILE
	: "${PGADMIN_USER:?PGADMIN_USER is required}"
	if PGPASSWORD="$PGADMIN_PASSWORD" psql --no-psqlrc --username "$PGADMIN_USER" --dbname postgres --tuples-only --no-align --command="SELECT 1 FROM pg_database WHERE datname = '$target_database'" | grep -q 1; then
		fail "restore target database already exists: $target_database"
	fi
	PGPASSWORD="$PGADMIN_PASSWORD" createdb --username "$PGADMIN_USER" --owner "$PGUSER" --template template0 "$target_database"
}

drop_restore_database() {
	target_database=$1
	PGPASSWORD="$PGADMIN_PASSWORD" dropdb --username "$PGADMIN_USER" --if-exists --force "$target_database"
}

restore_into() {
	target_database=$1
	PGPASSWORD="$PGPASSWORD" pg_restore --no-owner --no-acl --exit-on-error --dbname "$target_database" "$dump_path"
}

restore_drill() {
	[ "$#" -eq 1 ] || fail "usage: backup restore-drill <manifest-uri>"
	prepare_database_environment
	load_archive "$1"
	target_database="nexaflow_restore_$(date -u +%Y%m%d%H%M%S)_$$"
	started_epoch=$(date +%s)
	create_restore_database "$target_database"
	trap 'drop_restore_database "$target_database" >/dev/null 2>&1 || true; cleanup_work_directory' EXIT HUP INT TERM
	restore_into "$target_database"

	migration_count=$(PGPASSWORD="$PGADMIN_PASSWORD" psql --no-psqlrc --username "$PGADMIN_USER" --dbname "$target_database" --tuples-only --no-align --command='SELECT count(*) FROM schema_migrations;')
	tenant_count=$(PGPASSWORD="$PGADMIN_PASSWORD" psql --no-psqlrc --username "$PGADMIN_USER" --dbname "$target_database" --tuples-only --no-align --command='SELECT count(*) FROM tenants;')
	require_positive_integer "$migration_count" migration_count
	require_positive_integer "$tenant_count" tenant_count
	completed_epoch=$(date +%s)
	evidence_directory=${BACKUP_EVIDENCE_DIR:-/tmp}
	mkdir -p "$evidence_directory"
	evidence_path="$evidence_directory/restore-drill-$(date -u +%Y%m%dT%H%M%SZ).json"
	jq -n \
		--arg completed_at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
		--arg manifest_uri "$1" \
		--arg target_database "$target_database" \
		--argjson duration_seconds "$((completed_epoch - started_epoch))" \
		--argjson migration_count "$migration_count" \
		--argjson tenant_count "$tenant_count" \
		'{status:"passed",completed_at:$completed_at,manifest_uri:$manifest_uri,target_database:$target_database,duration_seconds:$duration_seconds,migration_count:$migration_count,tenant_count:$tenant_count}' \
		>"$evidence_path"
	drop_restore_database "$target_database"
	trap cleanup_work_directory EXIT HUP INT TERM

	if [ -n "${BACKUP_EVIDENCE_S3_URI:-}" ]; then
		: "${BACKUP_SIGNING_PRIVATE_KEY_FILE:?BACKUP_SIGNING_PRIVATE_KEY_FILE is required to publish evidence}"
		evidence_signature="$evidence_path.minisig"
		minisign -S -q -s "$BACKUP_SIGNING_PRIVATE_KEY_FILE" -m "$evidence_path" -x "$evidence_signature"
		evidence_uri="${BACKUP_EVIDENCE_S3_URI%/}/$(basename "$evidence_path")"
		prepare_backup_store
		aws_copy "$evidence_signature" "$evidence_uri.minisig"
		aws_copy "$evidence_path" "$evidence_uri"
		printf 'evidence_uri=%s\n' "$evidence_uri"
	fi
	log "restore drill passed"
	printf 'evidence=%s\n' "$evidence_path"
}

restore_backup() {
	[ "$#" -eq 2 ] || fail "usage: backup restore <manifest-uri> <new-database-name>"
	manifest_source=$1
	target_database=$2
	case "$target_database" in
	nexaflow_restore_* | nexaflow_recovery_*) ;;
	*) fail "restore target must start with nexaflow_restore_ or nexaflow_recovery_" ;;
	esac
	[ "${RESTORE_CONFIRM:-}" = "restore:$target_database" ] || fail "set RESTORE_CONFIRM=restore:$target_database"
	prepare_database_environment
	load_archive "$manifest_source"
	create_restore_database "$target_database"
	trap 'drop_restore_database "$target_database" >/dev/null 2>&1 || true; cleanup_work_directory' EXIT HUP INT TERM
	if ! restore_into "$target_database"; then
		log "restore failed; dropping incomplete target"
		drop_restore_database "$target_database"
		exit 1
	fi
	trap cleanup_work_directory EXIT HUP INT TERM
	log "restore completed into isolated database $target_database"
}

latest_manifest_uri() {
	require_command aws
	require_command jq
	: "${BACKUP_S3_URI:?BACKUP_S3_URI is required}"
	prepare_backup_store
	without_scheme=${BACKUP_S3_URI#s3://}
	bucket=${without_scheme%%/*}
	if [ "$bucket" = "$without_scheme" ]; then
		prefix=""
	else
		prefix=${without_scheme#*/}
		prefix=${prefix%/}/
	fi
	listing=$(mktemp "${TMPDIR:-/tmp}/nexaflow-backups.XXXXXX")
	case "$listing" in
	"${TMPDIR:-/tmp}"/nexaflow-backups.*) ;;
	*) fail "unexpected backup listing path" ;;
	esac
	aws_api list-objects-v2 --bucket "$bucket" --prefix "$prefix" >"$listing"
	key=$(jq -r '[.Contents[]? | select(.Key | endswith(".dump.age.json"))] | sort_by(.LastModified) | last | .Key // empty' "$listing")
	rm -f "$listing"
	[ -n "$key" ] || fail "no backup manifest found under $BACKUP_S3_URI"
	printf 's3://%s/%s\n' "$bucket" "$key"
}

restore_latest_drill() {
	[ "$#" -eq 0 ] || fail "usage: backup restore-latest-drill"
	manifest_source=$(latest_manifest_uri)
	restore_drill "$manifest_source"
}

show_help() {
	cat <<'EOF'
Usage:
  backup create
  backup restore-drill <manifest-uri>
  backup restore-latest-drill
  backup restore <manifest-uri> <new-database-name>

Backups are PostgreSQL custom archives encrypted to an age recipient. Restore
commands verify the encrypted checksum and archive contents before creating a
new, empty database. They never overwrite the live database.
EOF
}

case "${1:-help}" in
create)
	shift
	create_backup "$@"
	;;
restore-drill)
	shift
	restore_drill "$@"
	;;
restore-latest-drill)
	shift
	restore_latest_drill "$@"
	;;
restore)
	shift
	restore_backup "$@"
	;;
help | -h | --help) show_help ;;
*) fail "unknown command: $1" ;;
esac
