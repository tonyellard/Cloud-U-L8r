# SPDX-License-Identifier: Apache-2.0
#
# Terraform test for kay-vee (SSM Parameter Store + Secrets Manager emulator)
# Validates PutParameter, GetParameter, CreateSecret, and GetSecretValue operations.

terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "5.82.2"
    }
  }
}

provider "aws" {
  region                      = "us-east-1"
  access_key                  = "test"
  secret_key                  = "test"
  skip_credentials_validation = true
  skip_metadata_api_check     = true
  skip_requesting_account_id  = true

  endpoints {
    ssm            = "http://localhost:9350"
    secretsmanager = "http://localhost:9350"
  }
}

# Create an SSM String parameter
resource "aws_ssm_parameter" "string_param" {
  name  = "/terraform/test/string-param"
  type  = "String"
  value = "hello-from-terraform"
}

# Create an SSM SecureString parameter
resource "aws_ssm_parameter" "secure_param" {
  name  = "/terraform/test/secure-param"
  type  = "SecureString"
  value = "super-secret-value"
}

# Create a Secrets Manager secret
resource "aws_secretsmanager_secret" "test" {
  name = "terraform/test-secret"
}

# Set the secret value
resource "aws_secretsmanager_secret_version" "test" {
  secret_id     = aws_secretsmanager_secret.test.id
  secret_string = "{\"username\":\"admin\",\"password\":\"s3cr3t\"}"
}
