#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
project_dir="$(cd -- "${script_dir}/.." && pwd)"

image="${APP_IMAGE:-localhost/oci-adb-inventory:2.5.0}"
output_dir="${OUTPUT_DIR:-${project_dir}/reports}"
oci_config_dir="${OCI_CONFIG_DIR:-${HOME}/.oci}"
oci_profile="${OCI_PROFILE:-DEFAULT}"

command -v podman >/dev/null 2>&1 || {
  echo "error: podman is not installed or not in PATH" >&2
  exit 1
}
[[ -r "${oci_config_dir}/config" ]] || {
  echo "error: ${oci_config_dir}/config is not readable" >&2
  exit 1
}
mkdir -p -- "${output_dir}"

exec podman run --rm \
  --userns=keep-id \
  --user "$(id -u):$(id -g)" \
  --read-only \
  --security-opt=no-new-privileges \
  --cap-drop=ALL \
  --env "HOME=${HOME}" \
  --volume "${oci_config_dir}:${HOME}/.oci:ro,Z" \
  --volume "${output_dir}:/reports:Z" \
  "${image}" \
  --auth api_key \
  --config-file "${HOME}/.oci/config" \
  --profile "${oci_profile}" \
  --output-dir /reports \
  "$@"
