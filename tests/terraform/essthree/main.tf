# SPDX-License-Identifier: Apache-2.0
#
# Terraform test for essthree (S3 emulator)
# Validates CreateBucket, PutObject, HeadObject, and DeleteObject operations.

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
  s3_use_path_style           = true

  endpoints {
    s3 = "http://localhost:9300"
  }
}

# Create S3 bucket
resource "aws_s3_bucket" "test" {
  bucket = "terraform-test-bucket"
}

# Upload an object to the bucket
resource "aws_s3_object" "test" {
  bucket       = aws_s3_bucket.test.id
  key          = "hello.txt"
  content      = "Hello from Terraform!"
  content_type = "text/plain"
}
