# SPDX-License-Identifier: Apache-2.0
#
# Default Cloud-U-L8r config — provisions baseline resources against local emulators.

terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "5.82.2"
    }
  }
}

provider "aws" {
  region                      = var.region
  access_key                  = "test"
  secret_key                  = "test"
  skip_credentials_validation = true
  skip_metadata_api_check     = true
  skip_requesting_account_id  = true
  s3_use_path_style           = true

  endpoints {
    s3             = var.s3_endpoint
    sqs            = var.sqs_endpoint
    sns            = var.sns_endpoint
    ssm            = var.ssm_endpoint
    secretsmanager = var.ssm_endpoint
    eventbridge    = var.eventbridge_endpoint
    scheduler      = var.scheduler_endpoint
    pipes          = var.pipes_endpoint
  }
}
