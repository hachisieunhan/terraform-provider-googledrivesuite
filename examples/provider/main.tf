terraform {
  required_providers {
    googledrivesuite = {
      source  = "hachisieunhan/googledrivesuite"
      version = "~> 1.0"
    }
  }
}

provider "googledrivesuite" {
  # Option 1: Inline credentials (not recommended for production)
  # credentials = file("service-account.json")

  # Option 2: Set the GOOGLE_APPLICATION_CREDENTIALS environment variable
  # export GOOGLE_APPLICATION_CREDENTIALS='{"type":"service_account",...}'
}

# Example: Grant read access to a spreadsheet
resource "googledrivesuite_permission" "viewer" {
  file_id       = "your-spreadsheet-id"
  role          = "reader"
  type          = "user"
  email_address = "viewer@example.com"

  send_notification = false
}
