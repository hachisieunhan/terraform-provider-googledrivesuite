package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/storage"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
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
	opts := []option.ClientOption{
		option.WithAuthCredentialsJSON(option.ServiceAccount, []byte(credentialsJSON)),
		option.WithScopes(
			sheets.SpreadsheetsScope,
			drive.DriveScope,
			drive.DriveFileScope,
			storage.ScopeFullControl,
		),
	}

	// Extract the project_id from the service account JSON and set it as the
	// quota project. This ensures API calls are correctly attributed even when
	// the service account belongs to a different project than the one where
	// the APIs are enabled.
	var sa struct {
		ProjectID string `json:"project_id"`
	}
	if err := json.Unmarshal([]byte(credentialsJSON), &sa); err == nil && sa.ProjectID != "" {
		opts = append(opts, option.WithQuotaProject(sa.ProjectID))
	}

	sheetsService, err := sheets.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("unable to create Google Sheets client: %w", err)
	}

	driveService, err := drive.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("unable to create Google Drive client: %w", err)
	}

	storageClient, err := storage.NewClient(ctx, opts...)
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

// isForbidden returns true if the error represents an HTTP 403 Forbidden
// response from a Google API call.
func isForbidden(err error) bool {
	var apiErr *googleapi.Error
	if errors.As(err, &apiErr) {
		return apiErr.Code == http.StatusForbidden
	}
	return false
}

// retryOn403 retries the given function with exponential backoff when a 403
// Forbidden error is returned. This handles the common race condition where
// a Google Cloud API (e.g. Sheets, Drive) has just been enabled via
// google_project_service but the permission has not yet propagated.
func retryOn403(ctx context.Context, timeout time.Duration, fn func() error) error {
	const (
		initialBackoff = 5 * time.Second
		maxBackoff     = 30 * time.Second
		multiplier     = 2.0
	)

	deadline := time.Now().Add(timeout)
	backoff := initialBackoff

	for {
		err := fn()
		if err == nil {
			return nil
		}

		if !isForbidden(err) {
			return err
		}

		if time.Now().Add(backoff).After(deadline) {
			return fmt.Errorf("%w (retried until timeout %s)", err, timeout)
		}

		tflog.Warn(ctx, "received 403 Forbidden, retrying after backoff",
			map[string]interface{}{
				"backoff": backoff.String(),
				"error":   err.Error(),
			},
		)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}

		backoff = time.Duration(float64(backoff) * multiplier)
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}
