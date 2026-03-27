package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/api/sheets/v4"
)

var _ resource.Resource = &SpreadsheetResource{}
var _ resource.ResourceWithImportState = &SpreadsheetResource{}

// SpreadsheetResource defines the resource implementation.
type SpreadsheetResource struct{}

// SpreadsheetResourceModel describes the resource data model.
type SpreadsheetResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Credentials    types.String `tfsdk:"credentials"`
	Title          types.String `tfsdk:"title"`
	Locale         types.String `tfsdk:"locale"`
	TimeZone       types.String `tfsdk:"time_zone"`
	SpreadsheetURL types.String `tfsdk:"spreadsheet_url"`
}

func NewSpreadsheetResource() resource.Resource {
	return &SpreadsheetResource{}
}

func (r *SpreadsheetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_spreadsheet"
}

func (r *SpreadsheetResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Creates and manages a Google Spreadsheet.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The spreadsheet ID assigned by Google.",
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
			"title": schema.StringAttribute{
				Description: "The title of the spreadsheet.",
				Required:    true,
			},
			"locale": schema.StringAttribute{
				Description: "The locale of the spreadsheet (e.g., 'en_US'). Defaults to the service account's locale if not set.",
				Optional:    true,
				Computed:    true,
			},
			"time_zone": schema.StringAttribute{
				Description: "The time zone of the spreadsheet (e.g., 'America/New_York'). Defaults to the service account's time zone if not set.",
				Optional:    true,
				Computed:    true,
			},
			"spreadsheet_url": schema.StringAttribute{
				Description: "The URL to access the spreadsheet in a browser.",
				Computed:    true,
			},
		},
	}
}

func (r *SpreadsheetResource) Configure(_ context.Context, _ resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	// Credentials are configured at the resource level, so no provider data is needed.
}

func (r *SpreadsheetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SpreadsheetResourceModel
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

	properties := &sheets.SpreadsheetProperties{
		Title: data.Title.ValueString(),
	}

	if !data.Locale.IsNull() && !data.Locale.IsUnknown() {
		properties.Locale = data.Locale.ValueString()
	}
	if !data.TimeZone.IsNull() && !data.TimeZone.IsUnknown() {
		properties.TimeZone = data.TimeZone.ValueString()
	}

	spreadsheet := &sheets.Spreadsheet{
		Properties: properties,
	}

	result, err := clients.SheetsService.Spreadsheets.Create(spreadsheet).Context(ctx).Do()
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Spreadsheet",
			"Could not create spreadsheet: "+err.Error(),
		)
		return
	}

	data.ID = types.StringValue(result.SpreadsheetId)
	data.Title = types.StringValue(result.Properties.Title)
	data.Locale = types.StringValue(result.Properties.Locale)
	data.TimeZone = types.StringValue(result.Properties.TimeZone)
	data.SpreadsheetURL = types.StringValue(result.SpreadsheetUrl)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SpreadsheetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SpreadsheetResourceModel
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

	result, err := clients.SheetsService.Spreadsheets.Get(data.ID.ValueString()).
		Fields("spreadsheetId,properties(title,locale,timeZone),spreadsheetUrl").
		Context(ctx).Do()
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Spreadsheet",
			"Could not read spreadsheet ID "+data.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	data.Title = types.StringValue(result.Properties.Title)
	data.Locale = types.StringValue(result.Properties.Locale)
	data.TimeZone = types.StringValue(result.Properties.TimeZone)
	data.SpreadsheetURL = types.StringValue(result.SpreadsheetUrl)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SpreadsheetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data SpreadsheetResourceModel
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

	// Build the update requests for changed properties.
	var requests []*sheets.Request

	// Update title, locale, and time zone via a single UpdateSpreadsheetProperties request.
	properties := &sheets.SpreadsheetProperties{
		Title: data.Title.ValueString(),
	}
	fields := "title"

	if !data.Locale.IsNull() && !data.Locale.IsUnknown() {
		properties.Locale = data.Locale.ValueString()
		fields += ",locale"
	}
	if !data.TimeZone.IsNull() && !data.TimeZone.IsUnknown() {
		properties.TimeZone = data.TimeZone.ValueString()
		fields += ",timeZone"
	}

	requests = append(requests, &sheets.Request{
		UpdateSpreadsheetProperties: &sheets.UpdateSpreadsheetPropertiesRequest{
			Properties: properties,
			Fields:     fields,
		},
	})

	_, err = clients.SheetsService.Spreadsheets.BatchUpdate(data.ID.ValueString(), &sheets.BatchUpdateSpreadsheetRequest{
		Requests: requests,
	}).Context(ctx).Do()
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Spreadsheet",
			"Could not update spreadsheet ID "+data.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	// Re-read to get the latest state after update.
	result, err := clients.SheetsService.Spreadsheets.Get(data.ID.ValueString()).
		Fields("spreadsheetId,properties(title,locale,timeZone),spreadsheetUrl").
		Context(ctx).Do()
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Spreadsheet After Update",
			"Could not read spreadsheet ID "+data.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	data.Title = types.StringValue(result.Properties.Title)
	data.Locale = types.StringValue(result.Properties.Locale)
	data.TimeZone = types.StringValue(result.Properties.TimeZone)
	data.SpreadsheetURL = types.StringValue(result.SpreadsheetUrl)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SpreadsheetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SpreadsheetResourceModel
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

	// The Sheets API does not support deletion. Use the Drive API to delete the file.
	err = clients.DriveService.Files.Delete(data.ID.ValueString()).Context(ctx).Do()
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Spreadsheet",
			"Could not delete spreadsheet ID "+data.ID.ValueString()+": "+err.Error(),
		)
		return
	}
}

func (r *SpreadsheetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
