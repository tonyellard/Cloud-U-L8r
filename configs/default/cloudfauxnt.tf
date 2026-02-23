# SPDX-License-Identifier: Apache-2.0
#
# CloudFauxnt origins are managed via admin API (no AWS provider resource).

resource "terraform_data" "cloudfauxnt_origin_s3" {
  input = {
    endpoint = var.cloudfauxnt_endpoint
    origin = jsonencode({
      name               = "s3"
      url                = "http://essthree:9300"
      path_patterns      = ["/test-bucket/*"]
      require_signature  = false
      default_root_object = "index.html"
    })
  }

  provisioner "local-exec" {
    command = <<-EOT
      curl -sf -X POST "${self.input.endpoint}/admin/api/origins" \
        -H "Content-Type: application/json" \
        -d '${self.input.origin}'
    EOT
  }
}
