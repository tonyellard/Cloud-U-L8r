# SPDX-License-Identifier: Apache-2.0

# --- SSM Parameter Store ---

resource "aws_ssm_parameter" "app_db_host" {
  name  = "/app/database/host"
  type  = "String"
  value = "localhost"
}

resource "aws_ssm_parameter" "app_db_password" {
  name  = "/app/database/password"
  type  = "SecureString"
  value = "change-me"
}

# --- Secrets Manager ---

resource "aws_secretsmanager_secret" "db_credentials" {
  name = "app/database/credentials"
}

resource "aws_secretsmanager_secret_version" "db_credentials" {
  secret_id     = aws_secretsmanager_secret.db_credentials.id
  secret_string = jsonencode({
    username = "admin"
    password = "s3cr3t"
  })
}
