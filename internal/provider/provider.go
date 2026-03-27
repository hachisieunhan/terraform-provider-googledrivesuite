package provider

import (
	"context"
	"fmt"
	"os"

	"errors"
	"net/http"

	"cloud.google.com/go/storage"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

var _ provider.Provider = &GoogleDriveSuiteProvider{}

// GoogleDriveSuiteProvider defines the provider implementation.
type GoogleDriveSuiteProvider struct {
	version string
}

// GoogleDriveSuiteClients holds the initialized API clients for a resource.
type GoogleDriveSuiteClients struct {
	SheetsService *sheets.Service
	DriveService  *drive.Service
	StorageClient *storage.Client
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
		Attributes:  map[string]schema.Attribute{},
	}
}

func (p *GoogleDriveSuiteProvider) Configure(_ context.Context, _ provider.ConfigureRequest, _ *provider.ConfigureResponse) {
	// Credentials are configured at the resource level, so the provider has no configuration.
}

func (p *GoogleDriveSuiteProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewPermissionResource,
		NewSpreadsheetResource,
		NewSheetResource,
		NewSpreadsheetBackupResource,
	}
}

func (p *GoogleDriveSuiteProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

// resolveCredentials resolves the credentials JSON string from the resource attribute
// or the GOOGLE_APPLICATION_CREDENTIALS environment variable.
func resolveCredentials(credentials types.String) (string, error) {
	var credentialsJSON string
	if credentials.IsNull() || credentials.IsUnknown() {
		credentialsJSON = os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	} else {
		credentialsJSON = credentials.ValueString()
	}

	if credentialsJSON == "" {
		return "", fmt.Errorf("missing Google service account credentials: set the 'credentials' attribute on the resource or the GOOGLE_APPLICATION_CREDENTIALS environment variable")
	}

	return credentialsJSON, nil
}

// newClients initializes Google API clients from the given credentials JSON string.
func newClients(ctx context.Context, credentialsJSON string) (*GoogleDriveSuiteClients, error) {
	credOption := option.WithAuthCredentialsJSON(option.ServiceAccount, []byte(credentialsJSON))

	sheetsService, err := sheets.NewService(ctx, credOption)
	if err != nil {
		return nil, fmt.Errorf("unable to create Google Sheets client: %w", err)
	}

	driveService, err := drive.NewService(ctx, credOption)
	if err != nil {
		return nil, fmt.Errorf("unable to create Google Drive client: %w", err)
	}

	storageClient, err := storage.NewClient(ctx, credOption)
	if err != nil {
		return nil, fmt.Errorf("unable to create Google Cloud Storage client: %w", err)
	}

	return &GoogleDriveSuiteClients{
		SheetsService: sheetsService,
		DriveService:  driveService,
		StorageClient: storageClient,
	}, nil
}

// isNotFound returns true if the error represents an HTTP 404 Not Found
// response from a Google API call.
func isNotFound(err error) bool {
	var apiErr *googleapi.Error
	if errors.As(err, &apiErr) {
		return apiErr.Code == http.StatusNotFound
	}
	return false
}
