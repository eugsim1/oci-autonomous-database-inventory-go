#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
project_dir="$(cd -- "${script_dir}/.." && pwd)"

image="${APP_IMAGE:-localhost/oci-adb-inventory:2.4.0}"
output_dir="${OUTPUT_DIR:-${project_dir}/reports}"

command -v podman >/dev/null 2>&1 || {
  echo "error: podman is not installed or not in PATH" >&2
  exit 1
}
mkdir -p -- "${output_dir}"

exec podman run --rm \
  --network host \
  --userns=keep-id \
  --user "$(id -u):$(id -g)" \
  --read-only \
  --security-opt=no-new-privileges \
  --cap-drop=ALL \
  --volume "${output_dir}:/reports:Z" \
  "${image}" \
  --auth instance_principal \
  --output-dir /reports \
  "$@"
