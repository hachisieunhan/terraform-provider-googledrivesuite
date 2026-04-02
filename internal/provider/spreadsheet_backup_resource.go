package provider

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &SpreadsheetBackupResource{}
var _ resource.ResourceWithImportState = &SpreadsheetBackupResource{}

// SpreadsheetBackupResource defines the resource implementation.
type SpreadsheetBackupResource struct{}

// SpreadsheetBackupResourceModel describes the resource data model.
type SpreadsheetBackupResourceModel struct {
	ID            types.String `tfsdk:"id"`
	Credentials   types.String `tfsdk:"credentials"`
	SpreadsheetID types.String `tfsdk:"spreadsheet_id"`
	Bucket        types.String `tfsdk:"bucket"`
	ObjectPath    types.String `tfsdk:"object_path"`
	ExportFormat  types.String `tfsdk:"export_format"`
	GCSObjectURL  types.String `tfsdk:"gcs_object_url"`
	LastBackup    types.String `tfsdk:"last_backup"`
}

// exportFormatMIMEMap maps user-friendly format names to Google Drive export MIME types.
var exportFormatMIMEMap = map[string]struct {
	mimeType  string
	extension string
}{
	"xlsx": {"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", ".xlsx"},
	"pdf":  {"application/pdf", ".pdf"},
	"csv":  {"text/csv", ".csv"},
	"ods":  {"application/vnd.oasis.opendocument.spreadsheet", ".ods"},
	"tsv":  {"text/tab-separated-values", ".tsv"},
}

func NewSpreadsheetBackupResource() resource.Resource {
	return &SpreadsheetBackupResource{}
}

func (r *SpreadsheetBackupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_spreadsheet_backup"
}

func (r *SpreadsheetBackupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Exports a Google Spreadsheet to a Google Cloud Storage bucket. The backup is created or updated whenever Terraform applies.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Composite ID in the format bucket/object_path.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"credentials": schema.StringAttribute{
				Description: "Service account JSON credentials. Can also be set via the GOOGLE_APPLICATION_CREDENTIALS environment variable.",
				Optional:    true,
				Sensitive:   true,
			},
			"spreadsheet_id": schema.StringAttribute{
				Description: "The ID of the Google Spreadsheet to back up.",
				Required:    true,
			},
			"bucket": schema.StringAttribute{
				Description: "The name of the GCS bucket to store the backup.",
				Required:    true,
			},
			"object_path": schema.StringAttribute{
				Description: "The object path (key) within the GCS bucket. The export format extension will be appended automatically if not included.",
				Required:    true,
			},
			"export_format": schema.StringAttribute{
				Description: "The format to export. Valid values: xlsx (default), pdf, csv, ods, tsv.",
				Optional:    true,
				Computed:    true,
			},
			"gcs_object_url": schema.StringAttribute{
				Description: "The full GCS URL of the backup object (gs://bucket/path).",
				Computed:    true,
			},
			"last_backup": schema.StringAttribute{
				Description: "The RFC3339 timestamp of the last successful backup.",
				Computed:    true,
			},
		},
	}
}

func (r *SpreadsheetBackupResource) Configure(_ context.Context, _ resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	// Credentials are configured at the resource level, so no provider data is needed.
}

func (r *SpreadsheetBackupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SpreadsheetBackupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	credentialsJSON, err := resolveCredentials(data.Credentials)
	if err != nil {
		resp.Diagnostics.AddError("Missing Google Credentials", err.Error())
		return
	}

	clients, err := newClients(ctx, credentialsJSON)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Create API Clients", err.Error())
		return
	}

	// Default export format to xlsx.
	exportFormat := "xlsx"
	if !data.ExportFormat.IsNull() && !data.ExportFormat.IsUnknown() {
		exportFormat = data.ExportFormat.ValueString()
	}

	formatInfo, ok := exportFormatMIMEMap[exportFormat]
	if !ok {
		resp.Diagnostics.AddError(
			"Invalid Export Format",
			fmt.Sprintf("Unsupported export format: %s. Valid values: xlsx, pdf, csv, ods, tsv.", exportFormat),
		)
		return
	}

	// Perform the export and upload with retry for API propagation delays.
	objectPath := data.ObjectPath.ValueString()
	err = retryOn403(ctx, 2*time.Minute, func() error {
		return exportAndUpload(ctx, clients, data.SpreadsheetID.ValueString(), data.Bucket.ValueString(), objectPath, formatInfo.mimeType)
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Backup",
			"Could not export and upload spreadsheet: "+err.Error(),
		)
		return
	}

	data.ExportFormat = types.StringValue(exportFormat)
	data.ID = types.StringValue(fmt.Sprintf("%s/%s", data.Bucket.ValueString(), objectPath))
	data.GCSObjectURL = types.StringValue(fmt.Sprintf("gs://%s/%s", data.Bucket.ValueString(), objectPath))
	data.LastBackup = types.StringValue(time.Now().UTC().Format(time.RFC3339))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SpreadsheetBackupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SpreadsheetBackupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	credentialsJSON, err := resolveCredentials(data.Credentials)
	if err != nil {
		resp.Diagnostics.AddError("Missing Google Credentials", err.Error())
		return
	}

	clients, err := newClients(ctx, credentialsJSON)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Create API Clients", err.Error())
		return
	}

	// Verify the GCS object still exists.
	bucket := data.Bucket.ValueString()
	objectPath := data.ObjectPath.ValueString()

	attrs, err := clients.StorageClient.Bucket(bucket).Object(objectPath).Attrs(ctx)
	if err != nil {
		if err == storage.ErrObjectNotExist {
			// Object was deleted outside of Terraform.
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Backup Object",
			"Could not read GCS object "+objectPath+": "+err.Error(),
		)
		return
	}

	data.LastBackup = types.StringValue(attrs.Updated.UTC().Format(time.RFC3339))
	data.GCSObjectURL = types.StringValue(fmt.Sprintf("gs://%s/%s", bucket, objectPath))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SpreadsheetBackupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data SpreadsheetBackupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	credentialsJSON, err := resolveCredentials(data.Credentials)
	if err != nil {
		resp.Diagnostics.AddError("Missing Google Credentials", err.Error())
		return
	}

	clients, err := newClients(ctx, credentialsJSON)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Create API Clients", err.Error())
		return
	}

	exportFormat := "xlsx"
	if !data.ExportFormat.IsNull() && !data.ExportFormat.IsUnknown() {
		exportFormat = data.ExportFormat.ValueString()
	}

	formatInfo, ok := exportFormatMIMEMap[exportFormat]
	if !ok {
		resp.Diagnostics.AddError(
			"Invalid Export Format",
			fmt.Sprintf("Unsupported export format: %s. Valid values: xlsx, pdf, csv, ods, tsv.", exportFormat),
		)
		return
	}

	objectPath := data.ObjectPath.ValueString()
	err = retryOn403(ctx, 2*time.Minute, func() error {
		return exportAndUpload(ctx, clients, data.SpreadsheetID.ValueString(), data.Bucket.ValueString(), objectPath, formatInfo.mimeType)
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Backup",
			"Could not export and upload spreadsheet: "+err.Error(),
		)
		return
	}

	data.ExportFormat = types.StringValue(exportFormat)
	data.ID = types.StringValue(fmt.Sprintf("%s/%s", data.Bucket.ValueString(), objectPath))
	data.GCSObjectURL = types.StringValue(fmt.Sprintf("gs://%s/%s", data.Bucket.ValueString(), objectPath))
	data.LastBackup = types.StringValue(time.Now().UTC().Format(time.RFC3339))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SpreadsheetBackupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SpreadsheetBackupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	credentialsJSON, err := resolveCredentials(data.Credentials)
	if err != nil {
		resp.Diagnostics.AddError("Missing Google Credentials", err.Error())
		return
	}

	clients, err := newClients(ctx, credentialsJSON)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Create API Clients", err.Error())
		return
	}

	// Delete the GCS object.
	err = clients.StorageClient.Bucket(data.Bucket.ValueString()).Object(data.ObjectPath.ValueString()).Delete(ctx)
	if err != nil && err != storage.ErrObjectNotExist {
		resp.Diagnostics.AddError(
			"Error Deleting Backup Object",
			"Could not delete GCS object: "+err.Error(),
		)
		return
	}
}

func (r *SpreadsheetBackupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import ID format: spreadsheet_id|bucket|object_path
	// Using '|' as delimiter because '/' appears in object_path.
	parts := strings.SplitN(req.ID, "|", 3)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Expected import ID format: spreadsheet_id|bucket|object_path. Got: "+req.ID,
		)
		return
	}

	spreadsheetID := parts[0]
	bucket := parts[1]
	objectPath := parts[2]

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), fmt.Sprintf("%s/%s", bucket, objectPath))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("spreadsheet_id"), spreadsheetID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("bucket"), bucket)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("object_path"), objectPath)...)
}

// exportAndUpload exports a spreadsheet from Google Drive and uploads it to GCS.
func exportAndUpload(ctx context.Context, clients *GoogleDriveSuiteClients, spreadsheetID, bucket, objectPath, mimeType string) error {
	// Export the spreadsheet using the Drive API.
	exportResp, err := clients.DriveService.Files.Export(spreadsheetID, mimeType).Context(ctx).Download()
	if err != nil {
		return fmt.Errorf("failed to export spreadsheet %s: %w", spreadsheetID, err)
	}
	defer exportResp.Body.Close()

	// Upload to GCS.
	writer := clients.StorageClient.Bucket(bucket).Object(objectPath).NewWriter(ctx)
	writer.ContentType = mimeType

	if _, err := io.Copy(writer, exportResp.Body); err != nil {
		writer.Close()
		return fmt.Errorf("failed to upload to GCS gs://%s/%s: %w", bucket, objectPath, err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to finalize GCS upload gs://%s/%s: %w", bucket, objectPath, err)
	}

	return nil
}
