terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.0"
    }
    googledrivesuite = {
      source  = "hachisieunhan/googledrivesuite"
      version = "~> 1.0"
    }
  }
}

variable "project_id" {
  description = "The GCP project ID."
  type        = string
}

provider "google" {
  project = var.project_id
}

provider "googledrivesuite" {}

# -------------------------------------------------------------------
# 1. Create a Google service account
# -------------------------------------------------------------------
resource "google_service_account" "drive" {
  account_id   = "drive-manager"
  display_name = "Drive Manager"
  description  = "Service account for managing Google Drive resources via Terraform."
}

# -------------------------------------------------------------------
# 2. Create a service account key and extract its credentials
# -------------------------------------------------------------------
resource "google_service_account_key" "drive" {
  service_account_id = google_service_account.drive.name
}

# -------------------------------------------------------------------
# 3. Enable the required Google APIs
# -------------------------------------------------------------------
resource "google_project_service" "sheets" {
  service            = "sheets.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "drive" {
  service            = "drive.googleapis.com"
  disable_on_destroy = false
}

# -------------------------------------------------------------------
# 4. Pass the key to googledrivesuite resources
# -------------------------------------------------------------------
resource "googledrivesuite_spreadsheet" "example" {
  credentials = base64decode(google_service_account_key.drive.private_key)
  title       = "Quarterly Report"
  locale      = "en_US"
  time_zone   = "America/New_York"

  depends_on = [
    google_project_service.sheets,
    google_project_service.drive,
  ]
}

resource "googledrivesuite_sheet" "summary" {
  credentials    = base64decode(google_service_account_key.drive.private_key)
  spreadsheet_id = googledrivesuite_spreadsheet.example.id
  title          = "Summary"
  index          = 1
  row_count      = 500
  column_count   = 20
}

resource "googledrivesuite_permission" "editor" {
  credentials   = base64decode(google_service_account_key.drive.private_key)
  file_id       = googledrivesuite_spreadsheet.example.id
  role          = "writer"
  type          = "user"
  email_address = "team-lead@example.com"

  send_notification = true
}

# -------------------------------------------------------------------
# Outputs
# -------------------------------------------------------------------
output "service_account_email" {
  description = "The email of the created service account"
  value       = google_service_account.drive.email
}

output "spreadsheet_url" {
  description = "URL to access the spreadsheet in a browser"
  value       = googledrivesuite_spreadsheet.example.spreadsheet_url
}
