package provider

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/api/drive/v3"
)

var _ resource.Resource = &PermissionResource{}
var _ resource.ResourceWithImportState = &PermissionResource{}

// PermissionResource defines the resource implementation.
type PermissionResource struct{}

// PermissionResourceModel describes the resource data model.
type PermissionResourceModel struct {
	ID               types.String `tfsdk:"id"`
	Credentials      types.String `tfsdk:"credentials"`
	FileID           types.String `tfsdk:"file_id"`
	Role             types.String `tfsdk:"role"`
	Type             types.String `tfsdk:"type"`
	EmailAddress     types.String `tfsdk:"email_address"`
	Domain           types.String `tfsdk:"domain"`
	SendNotification types.Bool   `tfsdk:"send_notification"`
}

func NewPermissionResource() resource.Resource {
	return &PermissionResource{}
}

func (r *PermissionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_permission"
}

func (r *PermissionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a permission on a Google Drive file or spreadsheet.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The permission ID assigned by Google Drive.",
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
			"file_id": schema.StringAttribute{
				Description: "The ID of the file or spreadsheet.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"role": schema.StringAttribute{
				Description: "The role granted by this permission. Valid values: owner, organizer, fileOrganizer, writer, commenter, reader.",
				Required:    true,
			},
			"type": schema.StringAttribute{
				Description: "The type of the grantee. Valid values: user, group, domain, anyone.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"email_address": schema.StringAttribute{
				Description: "The email address of the user or group. Required when type is 'user' or 'group'.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"domain": schema.StringAttribute{
				Description: "The domain to which this permission refers. Required when type is 'domain'.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"send_notification": schema.BoolAttribute{
				Description: "Whether to send a notification email when sharing. Default is true.",
				Optional:    true,
			},
		},
	}
}

func (r *PermissionResource) Configure(_ context.Context, _ resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	// Credentials are configured at the resource level, so no provider data is needed.
}

func (r *PermissionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data PermissionResourceModel
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

	permission := &drive.Permission{
		Role: data.Role.ValueString(),
		Type: data.Type.ValueString(),
	}

	if !data.EmailAddress.IsNull() && !data.EmailAddress.IsUnknown() {
		permission.EmailAddress = data.EmailAddress.ValueString()
	}
	if !data.Domain.IsNull() && !data.Domain.IsUnknown() {
		permission.Domain = data.Domain.ValueString()
	}

	createCall := clients.DriveService.Permissions.Create(data.FileID.ValueString(), permission)

	if !data.SendNotification.IsNull() && !data.SendNotification.IsUnknown() {
		createCall.SendNotificationEmail(data.SendNotification.ValueBool())
	}

	result, err := createCall.Context(ctx).Do()
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Permission",
			"Could not create permission: "+err.Error(),
		)
		return
	}

	data.ID = types.StringValue(result.Id)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PermissionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data PermissionResourceModel
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

	permission, err := clients.DriveService.Permissions.Get(
		data.FileID.ValueString(),
		data.ID.ValueString(),
	).Fields("id,role,type,emailAddress,domain").Context(ctx).Do()
	if err != nil {
		if isNotFound(err) {
			// Permission or file was deleted outside of Terraform.
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Permission",
			"Could not read permission ID "+data.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	data.Role = types.StringValue(permission.Role)
	data.Type = types.StringValue(permission.Type)

	if permission.EmailAddress != "" {
		data.EmailAddress = types.StringValue(permission.EmailAddress)
	}
	if permission.Domain != "" {
		data.Domain = types.StringValue(permission.Domain)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PermissionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data PermissionResourceModel
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

	permission := &drive.Permission{
		Role: data.Role.ValueString(),
	}

	_, err = clients.DriveService.Permissions.Update(
		data.FileID.ValueString(),
		data.ID.ValueString(),
		permission,
	).Context(ctx).Do()
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Permission",
			"Could not update permission ID "+data.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PermissionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data PermissionResourceModel
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

	err = clients.DriveService.Permissions.Delete(
		data.FileID.ValueString(),
		data.ID.ValueString(),
	).Context(ctx).Do()
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Permission",
			"Could not delete permission ID "+data.ID.ValueString()+": "+err.Error(),
		)
		return
	}
}

func (r *PermissionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import ID format: file_id/permission_id
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Expected import ID format: file_id/permission_id. Got: "+req.ID,
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("file_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}
