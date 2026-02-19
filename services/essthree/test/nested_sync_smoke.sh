#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail

ENDPOINT_URL="${ENDPOINT_URL:-http://localhost:9300}"
BUCKET="${BUCKET:-sync-nested-bug-bucket}"

workdir="$(mktemp -d)"
trap 'rm -rf "${workdir}"' EXIT

mkdir -p "${workdir}/src/nested/level-one/level-two"
echo "root file" > "${workdir}/src/root.txt"
echo "level one file" > "${workdir}/src/nested/level-one/file.txt"
echo "level two file" > "${workdir}/src/nested/level-one/level-two/file.txt"

printf "Syncing local tree with nested folders to %s/%s ...\n" "$ENDPOINT_URL" "$BUCKET"
aws s3 sync "${workdir}/src" "s3://${BUCKET}/" --endpoint-url "$ENDPOINT_URL"

expected_keys=(
  "root.txt"
  "nested/level-one/file.txt"
  "nested/level-one/level-two/file.txt"
)

for key in "${expected_keys[@]}"; do
  printf "Verifying object key %s ... " "$key"
  aws s3api head-object \
    --bucket "$BUCKET" \
    --key "$key" \
    --endpoint-url "$ENDPOINT_URL" >/dev/null
  echo "OK"
done

echo "Nested sync smoke test passed."
