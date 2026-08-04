#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
project_dir="${script_dir}"
binary_path="${project_dir}/bin/oci-adb-inventory"
run_tests="${RUN_TESTS:-1}"

for command_name in go make; do
  command -v "${command_name}" >/dev/null 2>&1 || {
    echo "error: ${command_name} is required and was not found in PATH" >&2
    exit 1
  }
done

case "${binary_path}" in
  "${project_dir}/bin/oci-adb-inventory") ;;
  *)
    echo "error: refusing to remove an unexpected binary path: ${binary_path}" >&2
    exit 1
    ;;
esac

cd -- "${project_dir}"

printf 'Project: %s\n' "${project_dir}"
printf 'Target:  %s\n' "${binary_path}"
go version

echo 'Removing the previous binary, if present...'
make --no-print-directory clean
if [[ -e "${binary_path}" || -L "${binary_path}" ]]; then
  echo "error: previous binary was not removed: ${binary_path}" >&2
  exit 1
fi

echo 'Downloading pinned Go modules...'
make --no-print-directory deps

case "${run_tests}" in
  1|true|TRUE|yes|YES)
    echo 'Running tests and go vet...'
    make --no-print-directory check
    ;;
  0|false|FALSE|no|NO)
    echo 'Skipping tests because RUN_TESTS=0.'
    ;;
  *)
    echo "error: RUN_TESTS must be 1/true/yes or 0/false/no" >&2
    exit 2
    ;;
esac

echo 'Building a new static Linux binary...'
make --no-print-directory build CGO_ENABLED=0

if [[ ! -x "${binary_path}" ]]; then
  echo "error: build completed without an executable binary: ${binary_path}" >&2
  exit 1
fi

echo 'Build completed successfully.'
"${binary_path}" --version
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum -- "${binary_path}"
elif command -v shasum >/dev/null 2>&1; then
  shasum -a 256 -- "${binary_path}"
else
  echo 'warning: sha256sum/shasum is unavailable; checksum not printed' >&2
fi
