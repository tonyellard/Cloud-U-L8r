#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail

ENDPOINT_URL="${KAY_VEE_ENDPOINT_URL:-http://localhost:9350}"
AWS_REGION="${AWS_REGION:-us-east-1}"
AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID:-test}"
AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY:-test}"

export AWS_REGION AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY

suffix="$(date +%s)"
parameter_name="/smoke/param/${suffix}"
secret_name="smoke/secret/${suffix}"

echo "[aws-cli] endpoint: ${ENDPOINT_URL}"
echo "[aws-cli] parameter: ${parameter_name}"
echo "[aws-cli] secret: ${secret_name}"

aws --endpoint-url "${ENDPOINT_URL}" ssm put-parameter \
  --name "${parameter_name}" \
  --type "String" \
  --value "initial-value" >/dev/null

aws --endpoint-url "${ENDPOINT_URL}" ssm put-parameter \
  --name "${parameter_name}" \
  --type "String" \
  --value "updated-value" \
  --overwrite >/dev/null

parameter_value="$(aws --endpoint-url "${ENDPOINT_URL}" ssm get-parameter --name "${parameter_name}" --query 'Parameter.Value' --output text)"

if [[ "${parameter_value}" != "updated-value" ]]; then
  echo "[aws-cli] parameter assertion failed: expected updated-value got ${parameter_value}" >&2
  exit 1
fi

aws --endpoint-url "${ENDPOINT_URL}" secretsmanager create-secret \
  --name "${secret_name}" \
  --secret-string "initial-secret" >/dev/null

aws --endpoint-url "${ENDPOINT_URL}" secretsmanager put-secret-value \
  --secret-id "${secret_name}" \
  --secret-string "updated-secret" >/dev/null

secret_value="$(aws --endpoint-url "${ENDPOINT_URL}" secretsmanager get-secret-value --secret-id "${secret_name}" --query 'SecretString' --output text)"

if [[ "${secret_value}" != "updated-secret" ]]; then
  echo "[aws-cli] secret assertion failed: expected updated-secret got ${secret_value}" >&2
  exit 1
fi

echo "[aws-cli] smoke test passed"