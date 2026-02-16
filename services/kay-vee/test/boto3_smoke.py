#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
import os
import sys
import time

import boto3


def main() -> int:
    endpoint_url = os.getenv("KAY_VEE_ENDPOINT_URL", "http://localhost:9350")
    region_name = os.getenv("AWS_REGION", "us-east-1")
    access_key = os.getenv("AWS_ACCESS_KEY_ID", "test")
    secret_key = os.getenv("AWS_SECRET_ACCESS_KEY", "test")

    session = boto3.session.Session()
    ssm = session.client(
        "ssm",
        endpoint_url=endpoint_url,
        region_name=region_name,
        aws_access_key_id=access_key,
        aws_secret_access_key=secret_key,
    )
    secrets = session.client(
        "secretsmanager",
        endpoint_url=endpoint_url,
        region_name=region_name,
        aws_access_key_id=access_key,
        aws_secret_access_key=secret_key,
    )

    suffix = str(int(time.time()))
    parameter_name = f"/smoke/param/boto3/{suffix}"
    secret_name = f"smoke/secret/boto3/{suffix}"

    print(f"[boto3] endpoint: {endpoint_url}")
    print(f"[boto3] parameter: {parameter_name}")
    print(f"[boto3] secret: {secret_name}")

    ssm.put_parameter(Name=parameter_name, Type="String", Value="initial-value", Overwrite=False)
    ssm.put_parameter(Name=parameter_name, Type="String", Value="updated-value", Overwrite=True)
    parameter_value = ssm.get_parameter(Name=parameter_name)["Parameter"]["Value"]

    if parameter_value != "updated-value":
        print(f"[boto3] parameter assertion failed: expected updated-value got {parameter_value}", file=sys.stderr)
        return 1

    secrets.create_secret(Name=secret_name, SecretString="initial-secret")
    secrets.put_secret_value(SecretId=secret_name, SecretString="updated-secret")
    secret_value = secrets.get_secret_value(SecretId=secret_name)["SecretString"]

    if secret_value != "updated-secret":
        print(f"[boto3] secret assertion failed: expected updated-secret got {secret_value}", file=sys.stderr)
        return 1

    print("[boto3] smoke test passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())