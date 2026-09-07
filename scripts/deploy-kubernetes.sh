#!/bin/sh
set -eu

fail() {
	printf 'ERROR: %s\n' "$*" >&2
	exit 1
}

command -v kubectl >/dev/null 2>&1 || fail "kubectl is required"
command -v kustomize >/dev/null 2>&1 || fail "kustomize is required"
: "${BACKEND_IMAGE:?BACKEND_IMAGE is required}"
: "${FRONTEND_IMAGE:?FRONTEND_IMAGE is required}"
: "${BACKUP_IMAGE:?BACKUP_IMAGE is required}"
: "${PUBLIC_URL:?PUBLIC_URL is required}"

script_directory=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repository_root=$(CDPATH= cd -- "$script_directory/.." && pwd)
sh "$repository_root/scripts/release-preflight.sh"

render_directory=$(mktemp -d "${TMPDIR:-/tmp}/nexaflow-kustomize.XXXXXX")
cleanup() {
	case "${render_directory:-}" in
	"${TMPDIR:-/tmp}"/nexaflow-kustomize.*) rm -rf -- "$render_directory" ;;
	*) fail "refusing to remove unexpected render directory" ;;
	esac
}
trap cleanup EXIT HUP INT TERM
cp -R "$repository_root/deploy/kubernetes/." "$render_directory/"

(
	cd "$render_directory"
	kustomize edit set image \
		ghcr.io/neostacklab/nexaflow-backend="$BACKEND_IMAGE" \
		ghcr.io/neostacklab/nexaflow-frontend="$FRONTEND_IMAGE" \
		ghcr.io/neostacklab/nexaflow-backup="$BACKUP_IMAGE"
)

kubectl apply --server-side --dry-run=server -k "$render_directory" >/dev/null
kubectl apply --server-side -k "$render_directory"
kubectl -n nexaflow rollout status deployment/nexaflow-backend --timeout=10m
kubectl -n nexaflow rollout status deployment/nexaflow-frontend --timeout=10m
kubectl -n nexaflow wait --for=condition=available deployment/nexaflow-backend deployment/nexaflow-frontend --timeout=2m
curl --fail --silent --show-error --max-time 15 "${PUBLIC_URL%/}/health/ready" >/dev/null
printf 'NexaFlow rollout completed and readiness is healthy.\n'
