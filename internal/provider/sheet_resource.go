package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/api/sheets/v4"
)

var _ resource.Resource = &SheetResource{}
var _ resource.ResourceWithImportState = &SheetResource{}

// SheetResource defines the resource implementation for individual sheets (tabs).
type SheetResource struct{}

// SheetResourceModel describes the resource data model.
type SheetResourceModel struct {
	ID            types.String `tfsdk:"id"`
	Credentials   types.String `tfsdk:"credentials"`
	SpreadsheetID types.String `tfsdk:"spreadsheet_id"`
	SheetID       types.Int64  `tfsdk:"sheet_id"`
	Title         types.String `tfsdk:"title"`
	Index         types.Int64  `tfsdk:"index"`
	RowCount      types.Int64  `tfsdk:"row_count"`
	ColumnCount   types.Int64  `tfsdk:"column_count"`
}

func NewSheetResource() resource.Resource {
	return &SheetResource{}
}

func (r *SheetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sheet"
}

func (r *SheetResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an individual sheet (tab) within a Google Spreadsheet.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Composite ID in the format spreadsheet_id/sheet_id.",
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
				Description: "The ID of the parent spreadsheet.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"sheet_id": schema.Int64Attribute{
				Description: "The numeric ID of the sheet assigned by Google.",
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"title": schema.StringAttribute{
				Description: "The name of the sheet tab.",
				Required:    true,
			},
			"index": schema.Int64Attribute{
				Description: "The zero-based index of the sheet within the spreadsheet.",
				Optional:    true,
				Computed:    true,
			},
			"row_count": schema.Int64Attribute{
				Description: "The number of rows in the sheet grid. Defaults to 1000.",
				Optional:    true,
				Computed:    true,
			},
			"column_count": schema.Int64Attribute{
				Description: "The number of columns in the sheet grid. Defaults to 26.",
				Optional:    true,
				Computed:    true,
			},
		},
	}
}

func (r *SheetResource) Configure(_ context.Context, _ resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	// Credentials are configured at the resource level, so no provider data is needed.
}

func (r *SheetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SheetResourceModel
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

	sheetProperties := &sheets.SheetProperties{
		Title: data.Title.ValueString(),
	}

	if !data.Index.IsNull() && !data.Index.IsUnknown() {
		sheetProperties.Index = data.Index.ValueInt64()
	}

	gridProperties := &sheets.GridProperties{}
	hasGridProps := false

	if !data.RowCount.IsNull() && !data.RowCount.IsUnknown() {
		gridProperties.RowCount = data.RowCount.ValueInt64()
		hasGridProps = true
	}
	if !data.ColumnCount.IsNull() && !data.ColumnCount.IsUnknown() {
		gridProperties.ColumnCount = data.ColumnCount.ValueInt64()
		hasGridProps = true
	}

	if hasGridProps {
		sheetProperties.GridProperties = gridProperties
	}

	addSheetRequest := &sheets.Request{
		AddSheet: &sheets.AddSheetRequest{
			Properties: sheetProperties,
		},
	}

	var result *sheets.BatchUpdateSpreadsheetResponse
	err = retryOn403(ctx, 2*time.Minute, func() error {
		var createErr error
		result, createErr = clients.SheetsService.Spreadsheets.BatchUpdate(data.SpreadsheetID.ValueString(), &sheets.BatchUpdateSpreadsheetRequest{
			Requests: []*sheets.Request{addSheetRequest},
		}).Context(ctx).Do()
		return createErr
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Sheet",
			"Could not create sheet in spreadsheet "+data.SpreadsheetID.ValueString()+": "+err.Error(),
		)
		return
	}

	// Extract the new sheet's properties from the response.
	addedSheet := result.Replies[0].AddSheet
	data.SheetID = types.Int64Value(addedSheet.Properties.SheetId)
	data.Title = types.StringValue(addedSheet.Properties.Title)
	data.Index = types.Int64Value(addedSheet.Properties.Index)
	data.ID = types.StringValue(fmt.Sprintf("%s/%d", data.SpreadsheetID.ValueString(), addedSheet.Properties.SheetId))

	if addedSheet.Properties.GridProperties != nil {
		data.RowCount = types.Int64Value(addedSheet.Properties.GridProperties.RowCount)
		data.ColumnCount = types.Int64Value(addedSheet.Properties.GridProperties.ColumnCount)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SheetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SheetResourceModel
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

	spreadsheet, err := clients.SheetsService.Spreadsheets.Get(data.SpreadsheetID.ValueString()).
		Fields("sheets(properties)").
		Context(ctx).Do()
	if err != nil {
		if isNotFound(err) {
			// Parent spreadsheet was deleted outside of Terraform.
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Sheet",
			"Could not read spreadsheet "+data.SpreadsheetID.ValueString()+": "+err.Error(),
		)
		return
	}

	// Find the sheet with the matching sheet ID.
	var found *sheets.SheetProperties
	for _, s := range spreadsheet.Sheets {
		if s.Properties.SheetId == data.SheetID.ValueInt64() {
			found = s.Properties
			break
		}
	}

	if found == nil {
		// Sheet no longer exists, remove from state.
		resp.State.RemoveResource(ctx)
		return
	}

	data.Title = types.StringValue(found.Title)
	data.Index = types.Int64Value(found.Index)
	if found.GridProperties != nil {
		data.RowCount = types.Int64Value(found.GridProperties.RowCount)
		data.ColumnCount = types.Int64Value(found.GridProperties.ColumnCount)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SheetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data SheetResourceModel
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

	properties := &sheets.SheetProperties{
		SheetId: data.SheetID.ValueInt64(),
		Title:   data.Title.ValueString(),
	}
	fields := "title"

	if !data.Index.IsNull() && !data.Index.IsUnknown() {
		properties.Index = data.Index.ValueInt64()
		fields += ",index"
	}

	gridProperties := &sheets.GridProperties{}
	hasGridProps := false

	if !data.RowCount.IsNull() && !data.RowCount.IsUnknown() {
		gridProperties.RowCount = data.RowCount.ValueInt64()
		fields += ",gridProperties.rowCount"
		hasGridProps = true
	}
	if !data.ColumnCount.IsNull() && !data.ColumnCount.IsUnknown() {
		gridProperties.ColumnCount = data.ColumnCount.ValueInt64()
		fields += ",gridProperties.columnCount"
		hasGridProps = true
	}

	if hasGridProps {
		properties.GridProperties = gridProperties
	}

	updateRequest := &sheets.Request{
		UpdateSheetProperties: &sheets.UpdateSheetPropertiesRequest{
			Properties: properties,
			Fields:     fields,
		},
	}

	_, err = clients.SheetsService.Spreadsheets.BatchUpdate(data.SpreadsheetID.ValueString(), &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{updateRequest},
	}).Context(ctx).Do()
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Sheet",
			"Could not update sheet "+data.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	// Re-read the sheet to get the latest state.
	spreadsheet, err := clients.SheetsService.Spreadsheets.Get(data.SpreadsheetID.ValueString()).
		Fields("sheets(properties)").
		Context(ctx).Do()
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Sheet After Update",
			"Could not read spreadsheet "+data.SpreadsheetID.ValueString()+": "+err.Error(),
		)
		return
	}

	for _, s := range spreadsheet.Sheets {
		if s.Properties.SheetId == data.SheetID.ValueInt64() {
			data.Title = types.StringValue(s.Properties.Title)
			data.Index = types.Int64Value(s.Properties.Index)
			if s.Properties.GridProperties != nil {
				data.RowCount = types.Int64Value(s.Properties.GridProperties.RowCount)
				data.ColumnCount = types.Int64Value(s.Properties.GridProperties.ColumnCount)
			}
			break
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SheetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SheetResourceModel
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

	deleteRequest := &sheets.Request{
		DeleteSheet: &sheets.DeleteSheetRequest{
			SheetId: data.SheetID.ValueInt64(),
		},
	}

	_, err = clients.SheetsService.Spreadsheets.BatchUpdate(data.SpreadsheetID.ValueString(), &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{deleteRequest},
	}).Context(ctx).Do()
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Sheet",
			"Could not delete sheet "+data.ID.ValueString()+": "+err.Error(),
		)
		return
	}
}

func (r *SheetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import ID format: spreadsheet_id/sheet_id
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Expected import ID format: spreadsheet_id/sheet_id. Got: "+req.ID,
		)
		return
	}

	sheetID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Sheet ID",
			"Sheet ID must be a numeric value. Got: "+parts[1],
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("spreadsheet_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("sheet_id"), sheetID)...)
}
