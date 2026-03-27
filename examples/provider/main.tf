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

# -------------------------------------------------------------------
# 1. Create a Google Spreadsheet
# -------------------------------------------------------------------
resource "googledrivesuite_spreadsheet" "example" {
  title     = "Quarterly Report"
  locale    = "en_US"
  time_zone = "America/New_York"
}

# -------------------------------------------------------------------
# 2. Add additional sheets (tabs) to the spreadsheet
# -------------------------------------------------------------------
resource "googledrivesuite_sheet" "revenue" {
  spreadsheet_id = googledrivesuite_spreadsheet.example.id
  title          = "Revenue"
  index          = 1
  row_count      = 500
  column_count   = 20
}

resource "googledrivesuite_sheet" "expenses" {
  spreadsheet_id = googledrivesuite_spreadsheet.example.id
  title          = "Expenses"
  index          = 2
  row_count      = 500
  column_count   = 15
}

# -------------------------------------------------------------------
# 3. Share the spreadsheet with human users and service accounts
# -------------------------------------------------------------------
resource "googledrivesuite_permission" "editor" {
  file_id       = googledrivesuite_spreadsheet.example.id
  role          = "writer"
  type          = "user"
  email_address = "finance-team-lead@example.com"

  send_notification = true
}

resource "googledrivesuite_permission" "viewer" {
  file_id       = googledrivesuite_spreadsheet.example.id
  role          = "reader"
  type          = "user"
  email_address = "auditor@example.com"

  send_notification = false
}

resource "googledrivesuite_permission" "service_account" {
  file_id       = googledrivesuite_spreadsheet.example.id
  role          = "writer"
  type          = "user"
  email_address = "data-pipeline@my-project.iam.gserviceaccount.com"

  send_notification = false
}

# -------------------------------------------------------------------
# 4. Back up the spreadsheet to Google Cloud Storage
# -------------------------------------------------------------------
resource "googledrivesuite_spreadsheet_backup" "xlsx_backup" {
  spreadsheet_id = googledrivesuite_spreadsheet.example.id
  bucket         = "my-company-backups"
  object_path    = "spreadsheets/quarterly-report.xlsx"
  export_format  = "xlsx"
}

resource "googledrivesuite_spreadsheet_backup" "pdf_backup" {
  spreadsheet_id = googledrivesuite_spreadsheet.example.id
  bucket         = "my-company-backups"
  object_path    = "spreadsheets/quarterly-report.pdf"
  export_format  = "pdf"
}

# -------------------------------------------------------------------
# Outputs
# -------------------------------------------------------------------
output "spreadsheet_url" {
  description = "URL to access the spreadsheet in a browser"
  value       = googledrivesuite_spreadsheet.example.spreadsheet_url
}

output "xlsx_backup_url" {
  description = "GCS URL of the XLSX backup"
  value       = googledrivesuite_spreadsheet_backup.xlsx_backup.gcs_object_url
}
