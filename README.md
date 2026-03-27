# Terraform Provider: Google Drive Suite

A Terraform provider for managing Google Sheets and Google Drive resources using service accounts.

## Resources

| Resource | Description |
|---|---|
| `googledrivesuite_spreadsheet` | Creates and manages a Google Spreadsheet |
| `googledrivesuite_sheet` | Manages an individual sheet (tab) within a spreadsheet |
| `googledrivesuite_permission` | Manages sharing permissions on a file or spreadsheet |
| `googledrivesuite_spreadsheet_backup` | Exports a spreadsheet to a Google Cloud Storage bucket |

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://go.dev/dl/) >= 1.26 (for building from source)
- A Google Cloud service account with the Google Sheets and Google Drive APIs enabled

## Installation

Add the provider to your `required_providers` block:

```hcl
terraform {
  required_providers {
    googledrivesuite = {
      source  = "hachisieunhan/googledrivesuite"
    }
  }
}

provider "googledrivesuite" {}
```

## Authentication

Credentials are configured at the **resource level**, not the provider level. This allows different resources to authenticate with different service accounts.

Each resource accepts an optional `credentials` attribute containing service account JSON. If omitted, the `GOOGLE_APPLICATION_CREDENTIALS` environment variable is used as a fallback.

```hcl
# Option 1: Pass credentials directly
resource "googledrivesuite_spreadsheet" "example" {
  credentials = file("service-account.json")
  title       = "My Spreadsheet"
}

# Option 2: Use the environment variable
# export GOOGLE_APPLICATION_CREDENTIALS='{"type":"service_account",...}'
resource "googledrivesuite_spreadsheet" "example" {
  title = "My Spreadsheet"
}
```

Use a `locals` block to avoid repeating credentials across resources:

```hcl
locals {
  credentials = file("service-account.json")
}

resource "googledrivesuite_spreadsheet" "example" {
  credentials = local.credentials
  title       = "My Spreadsheet"
}

resource "googledrivesuite_permission" "editor" {
  credentials   = local.credentials
  file_id       = googledrivesuite_spreadsheet.example.id
  role          = "writer"
  type          = "user"
  email_address = "user@example.com"
}
```

## Usage

### Create a spreadsheet with sheets and permissions

```hcl
resource "googledrivesuite_spreadsheet" "report" {
  credentials = local.credentials
  title       = "Quarterly Report"
  locale      = "en_US"
  time_zone   = "America/New_York"
}

resource "googledrivesuite_sheet" "revenue" {
  credentials    = local.credentials
  spreadsheet_id = googledrivesuite_spreadsheet.report.id
  title          = "Revenue"
  index          = 1
  row_count      = 500
  column_count   = 20
}

resource "googledrivesuite_permission" "editor" {
  credentials   = local.credentials
  file_id       = googledrivesuite_spreadsheet.report.id
  role          = "writer"
  type          = "user"
  email_address = "finance-lead@example.com"
}
```

### Back up a spreadsheet to GCS

```hcl
resource "googledrivesuite_spreadsheet_backup" "xlsx" {
  credentials    = local.credentials
  spreadsheet_id = googledrivesuite_spreadsheet.report.id
  bucket         = "my-backups"
  object_path    = "spreadsheets/quarterly-report.xlsx"
  export_format  = "xlsx"
}
```

Supported export formats: `xlsx`, `pdf`, `csv`, `ods`, `tsv`.

### Create a service account with the Google provider

```hcl
resource "google_service_account" "drive" {
  account_id   = "drive-manager"
  display_name = "Drive Manager"
}

resource "google_service_account_key" "drive" {
  service_account_id = google_service_account.drive.name
}

resource "googledrivesuite_spreadsheet" "example" {
  credentials = base64decode(google_service_account_key.drive.private_key)
  title       = "My Spreadsheet"
}
```

See [`examples/`](examples/) for complete working configurations.

## Resource Reference

### googledrivesuite_spreadsheet

Creates and manages a Google Spreadsheet.

#### Arguments

| Attribute | Type | Required | Description |
|---|---|---|---|
| `credentials` | String | No | Service account JSON credentials (sensitive). Falls back to `GOOGLE_APPLICATION_CREDENTIALS` env var. |
| `title` | String | Yes | The title of the spreadsheet. |
| `locale` | String | No | The locale (e.g., `en_US`). Defaults to the service account's locale. |
| `time_zone` | String | No | The time zone (e.g., `America/New_York`). Defaults to the service account's time zone. |

#### Outputs

| Attribute | Type | Description |
|---|---|---|
| `id` | String | The spreadsheet ID assigned by Google. |
| `spreadsheet_url` | String | The URL to access the spreadsheet in a browser. |
| `locale` | String | The resolved locale of the spreadsheet (computed if not set). |
| `time_zone` | String | The resolved time zone of the spreadsheet (computed if not set). |

Import: `terraform import googledrivesuite_spreadsheet.example <spreadsheet_id>`

### googledrivesuite_sheet

Manages an individual sheet (tab) within a Google Spreadsheet.

#### Arguments

| Attribute | Type | Required | Description |
|---|---|---|---|
| `credentials` | String | No | Service account JSON credentials (sensitive). |
| `spreadsheet_id` | String | Yes | The ID of the parent spreadsheet. Changing this forces re-creation. |
| `title` | String | Yes | The name of the sheet tab. |
| `index` | Int64 | No | The zero-based position of the sheet. |
| `row_count` | Int64 | No | The number of rows. Defaults to 1000. |
| `column_count` | Int64 | No | The number of columns. Defaults to 26. |

#### Outputs

| Attribute | Type | Description |
|---|---|---|
| `id` | String | Composite ID in the format `spreadsheet_id/sheet_id`. |
| `sheet_id` | Int64 | The numeric sheet ID assigned by Google. |
| `index` | Int64 | The resolved zero-based position of the sheet (computed if not set). |
| `row_count` | Int64 | The resolved number of rows (computed if not set). |
| `column_count` | Int64 | The resolved number of columns (computed if not set). |

Import: `terraform import googledrivesuite_sheet.example <spreadsheet_id>/<sheet_id>`

### googledrivesuite_permission

Manages a sharing permission on a Google Drive file or spreadsheet.

#### Arguments

| Attribute | Type | Required | Description |
|---|---|---|---|
| `credentials` | String | No | Service account JSON credentials (sensitive). |
| `file_id` | String | Yes | The ID of the file or spreadsheet. Changing this forces re-creation. |
| `role` | String | Yes | The role: `owner`, `organizer`, `fileOrganizer`, `writer`, `commenter`, `reader`. |
| `type` | String | Yes | The grantee type: `user`, `group`, `domain`, `anyone`. Changing this forces re-creation. |
| `email_address` | String | No | The email address. Required when type is `user` or `group`. Changing this forces re-creation. |
| `domain` | String | No | The domain. Required when type is `domain`. Changing this forces re-creation. |
| `send_notification` | Bool | No | Whether to send a notification email. |

#### Outputs

| Attribute | Type | Description |
|---|---|---|
| `id` | String | The permission ID assigned by Google Drive. |

Import: `terraform import googledrivesuite_permission.example <file_id>/<permission_id>`

### googledrivesuite_spreadsheet_backup

Exports a Google Spreadsheet to a Google Cloud Storage bucket. The backup is created or updated on every `terraform apply`.

#### Arguments

| Attribute | Type | Required | Description |
|---|---|---|---|
| `credentials` | String | No | Service account JSON credentials (sensitive). |
| `spreadsheet_id` | String | Yes | The ID of the spreadsheet to back up. |
| `bucket` | String | Yes | The GCS bucket name. |
| `object_path` | String | Yes | The object path within the bucket. |
| `export_format` | String | No | Export format: `xlsx` (default), `pdf`, `csv`, `ods`, `tsv`. |

#### Outputs

| Attribute | Type | Description |
|---|---|---|
| `id` | String | Composite ID in the format `bucket/object_path`. |
| `export_format` | String | The resolved export format (defaults to `xlsx` if not set). |
| `gcs_object_url` | String | The full GCS URL (`gs://bucket/path`). |
| `last_backup` | String | RFC 3339 timestamp of the last successful backup. |

## Developing

### Build

```sh
go build ./...
```

### Test

```sh
go test ./... -v
```

### Local installation

Build and install the provider binary for local development:

```sh
go install .
```

Then configure a [dev override](https://developer.hashicorp.com/terraform/cli/config/config-file#development-overrides-for-provider-developers) in your `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "hachisieunhan/googledrivesuite" = "/path/to/go/bin"
  }
  direct {}
}
```

## License

See [LICENSE](LICENSE) for details.
