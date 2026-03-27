package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

var _ provider.Provider = &GoogleDriveSuiteProvider{}

// GoogleDriveSuiteProvider defines the provider implementation.
type GoogleDriveSuiteProvider struct {
	version string
}

// GoogleDriveSuiteProviderModel describes the provider data model.
type GoogleDriveSuiteProviderModel struct {
	Credentials types.String `tfsdk:"credentials"`
}

// GoogleDriveSuiteClients holds the initialized API clients shared with resources.
type GoogleDriveSuiteClients struct {
	SheetsService *sheets.Service
	DriveService  *drive.Service
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &GoogleDriveSuiteProvider{
			version: version,
		}
	}
}

func (p *GoogleDriveSuiteProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "googledrivesuite"
	resp.Version = p.version
}

func (p *GoogleDriveSuiteProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Terraform provider for managing Google Sheets and Google Drive resources using service accounts.",
		Attributes: map[string]schema.Attribute{
			"credentials": schema.StringAttribute{
				Description: "Service account JSON credentials. Can also be set via the GOOGLE_APPLICATION_CREDENTIALS environment variable.",
				Optional:    true,
				Sensitive:   true,
			},
		},
	}
}

func (p *GoogleDriveSuiteProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config GoogleDriveSuiteProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Resolve credentials: config value takes precedence over env var.
	var credentialsJSON string
	if config.Credentials.IsNull() || config.Credentials.IsUnknown() {
		credentialsJSON = os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	} else {
		credentialsJSON = config.Credentials.ValueString()
	}

	if credentialsJSON == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("credentials"),
			"Missing Google Credentials",
			"The provider requires Google service account credentials. Set the 'credentials' attribute in the provider block or the GOOGLE_APPLICATION_CREDENTIALS environment variable.",
		)
		return
	}

	// Initialize Google Sheets service.
	sheetsService, err := sheets.NewService(ctx, option.WithCredentialsJSON([]byte(credentialsJSON)))
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create Google Sheets Client",
			"An error occurred when creating the Google Sheets API client: "+err.Error(),
		)
		return
	}

	// Initialize Google Drive service.
	driveService, err := drive.NewService(ctx, option.WithCredentialsJSON([]byte(credentialsJSON)))
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create Google Drive Client",
			"An error occurred when creating the Google Drive API client: "+err.Error(),
		)
		return
	}

	clients := &GoogleDriveSuiteClients{
		SheetsService: sheetsService,
		DriveService:  driveService,
	}

	resp.DataSourceData = clients
	resp.ResourceData = clients
}

func (p *GoogleDriveSuiteProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewPermissionResource,
	}
}

func (p *GoogleDriveSuiteProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}
