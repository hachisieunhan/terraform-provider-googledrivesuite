package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// getResourceSchema is a test helper that retrieves the schema from a resource.
func getResourceSchema(t *testing.T, r resource.Resource) schema.Schema {
	t.Helper()
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics getting schema: %s", resp.Diagnostics)
	}
	return resp.Schema
}

// assertHasAttribute verifies a schema contains the named attribute.
func assertHasAttribute(t *testing.T, s schema.Schema, name string) {
	t.Helper()
	if _, ok := s.Attributes[name]; !ok {
		t.Errorf("expected schema to have attribute %q", name)
	}
}

// assertAttributeIsRequired verifies a schema attribute is required.
func assertAttributeIsRequired(t *testing.T, s schema.Schema, name string) {
	t.Helper()
	attr, ok := s.Attributes[name]
	if !ok {
		t.Fatalf("attribute %q not found in schema", name)
	}
	if !attr.IsRequired() {
		t.Errorf("expected attribute %q to be required", name)
	}
}

// assertAttributeIsOptional verifies a schema attribute is optional.
func assertAttributeIsOptional(t *testing.T, s schema.Schema, name string) {
	t.Helper()
	attr, ok := s.Attributes[name]
	if !ok {
		t.Fatalf("attribute %q not found in schema", name)
	}
	if !attr.IsOptional() {
		t.Errorf("expected attribute %q to be optional", name)
	}
}

// assertAttributeIsComputed verifies a schema attribute is computed.
func assertAttributeIsComputed(t *testing.T, s schema.Schema, name string) {
	t.Helper()
	attr, ok := s.Attributes[name]
	if !ok {
		t.Fatalf("attribute %q not found in schema", name)
	}
	if !attr.IsComputed() {
		t.Errorf("expected attribute %q to be computed", name)
	}
}

// assertAttributeIsSensitive verifies a schema attribute is sensitive.
func assertAttributeIsSensitive(t *testing.T, s schema.Schema, name string) {
	t.Helper()
	attr, ok := s.Attributes[name]
	if !ok {
		t.Fatalf("attribute %q not found in schema", name)
	}
	if !attr.IsSensitive() {
		t.Errorf("expected attribute %q to be sensitive", name)
	}
}

// --- Spreadsheet Resource ---

func TestSpreadsheetResource_Metadata(t *testing.T) {
	t.Parallel()
	r := NewSpreadsheetResource()
	resp := &resource.MetadataResponse{}
	r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "googledrivesuite"}, resp)

	if resp.TypeName != "googledrivesuite_spreadsheet" {
		t.Errorf("expected type name 'googledrivesuite_spreadsheet', got %q", resp.TypeName)
	}
}

func TestSpreadsheetResource_Schema_HasExpectedAttributes(t *testing.T) {
	t.Parallel()
	s := getResourceSchema(t, NewSpreadsheetResource())

	expectedAttrs := []string{"id", "credentials", "title", "locale", "time_zone", "spreadsheet_url"}
	for _, attr := range expectedAttrs {
		assertHasAttribute(t, s, attr)
	}

	if len(s.Attributes) != len(expectedAttrs) {
		t.Errorf("expected %d attributes, got %d", len(expectedAttrs), len(s.Attributes))
	}
}

func TestSpreadsheetResource_Schema_AttributeProperties(t *testing.T) {
	t.Parallel()
	s := getResourceSchema(t, NewSpreadsheetResource())

	assertAttributeIsComputed(t, s, "id")
	assertAttributeIsOptional(t, s, "credentials")
	assertAttributeIsSensitive(t, s, "credentials")
	assertAttributeIsRequired(t, s, "title")
	assertAttributeIsOptional(t, s, "locale")
	assertAttributeIsComputed(t, s, "locale")
	assertAttributeIsOptional(t, s, "time_zone")
	assertAttributeIsComputed(t, s, "time_zone")
	assertAttributeIsComputed(t, s, "spreadsheet_url")
}

// --- Sheet Resource ---

func TestSheetResource_Metadata(t *testing.T) {
	t.Parallel()
	r := NewSheetResource()
	resp := &resource.MetadataResponse{}
	r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "googledrivesuite"}, resp)

	if resp.TypeName != "googledrivesuite_sheet" {
		t.Errorf("expected type name 'googledrivesuite_sheet', got %q", resp.TypeName)
	}
}

func TestSheetResource_Schema_HasExpectedAttributes(t *testing.T) {
	t.Parallel()
	s := getResourceSchema(t, NewSheetResource())

	expectedAttrs := []string{"id", "credentials", "spreadsheet_id", "sheet_id", "title", "index", "row_count", "column_count"}
	for _, attr := range expectedAttrs {
		assertHasAttribute(t, s, attr)
	}

	if len(s.Attributes) != len(expectedAttrs) {
		t.Errorf("expected %d attributes, got %d", len(expectedAttrs), len(s.Attributes))
	}
}

func TestSheetResource_Schema_AttributeProperties(t *testing.T) {
	t.Parallel()
	s := getResourceSchema(t, NewSheetResource())

	assertAttributeIsComputed(t, s, "id")
	assertAttributeIsOptional(t, s, "credentials")
	assertAttributeIsSensitive(t, s, "credentials")
	assertAttributeIsRequired(t, s, "spreadsheet_id")
	assertAttributeIsComputed(t, s, "sheet_id")
	assertAttributeIsRequired(t, s, "title")
	assertAttributeIsOptional(t, s, "index")
	assertAttributeIsComputed(t, s, "index")
	assertAttributeIsOptional(t, s, "row_count")
	assertAttributeIsComputed(t, s, "row_count")
	assertAttributeIsOptional(t, s, "column_count")
	assertAttributeIsComputed(t, s, "column_count")
}

// --- Permission Resource ---

func TestPermissionResource_Metadata(t *testing.T) {
	t.Parallel()
	r := NewPermissionResource()
	resp := &resource.MetadataResponse{}
	r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "googledrivesuite"}, resp)

	if resp.TypeName != "googledrivesuite_permission" {
		t.Errorf("expected type name 'googledrivesuite_permission', got %q", resp.TypeName)
	}
}

func TestPermissionResource_Schema_HasExpectedAttributes(t *testing.T) {
	t.Parallel()
	s := getResourceSchema(t, NewPermissionResource())

	expectedAttrs := []string{"id", "credentials", "file_id", "role", "type", "email_address", "domain", "send_notification"}
	for _, attr := range expectedAttrs {
		assertHasAttribute(t, s, attr)
	}

	if len(s.Attributes) != len(expectedAttrs) {
		t.Errorf("expected %d attributes, got %d", len(expectedAttrs), len(s.Attributes))
	}
}

func TestPermissionResource_Schema_AttributeProperties(t *testing.T) {
	t.Parallel()
	s := getResourceSchema(t, NewPermissionResource())

	assertAttributeIsComputed(t, s, "id")
	assertAttributeIsOptional(t, s, "credentials")
	assertAttributeIsSensitive(t, s, "credentials")
	assertAttributeIsRequired(t, s, "file_id")
	assertAttributeIsRequired(t, s, "role")
	assertAttributeIsRequired(t, s, "type")
	assertAttributeIsOptional(t, s, "email_address")
	assertAttributeIsOptional(t, s, "domain")
	assertAttributeIsOptional(t, s, "send_notification")
}

// --- Spreadsheet Backup Resource ---

func TestSpreadsheetBackupResource_Metadata(t *testing.T) {
	t.Parallel()
	r := NewSpreadsheetBackupResource()
	resp := &resource.MetadataResponse{}
	r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "googledrivesuite"}, resp)

	if resp.TypeName != "googledrivesuite_spreadsheet_backup" {
		t.Errorf("expected type name 'googledrivesuite_spreadsheet_backup', got %q", resp.TypeName)
	}
}

func TestSpreadsheetBackupResource_Schema_HasExpectedAttributes(t *testing.T) {
	t.Parallel()
	s := getResourceSchema(t, NewSpreadsheetBackupResource())

	expectedAttrs := []string{"id", "credentials", "spreadsheet_id", "bucket", "object_path", "export_format", "gcs_object_url", "last_backup"}
	for _, attr := range expectedAttrs {
		assertHasAttribute(t, s, attr)
	}

	if len(s.Attributes) != len(expectedAttrs) {
		t.Errorf("expected %d attributes, got %d", len(expectedAttrs), len(s.Attributes))
	}
}

func TestSpreadsheetBackupResource_Schema_AttributeProperties(t *testing.T) {
	t.Parallel()
	s := getResourceSchema(t, NewSpreadsheetBackupResource())

	assertAttributeIsComputed(t, s, "id")
	assertAttributeIsOptional(t, s, "credentials")
	assertAttributeIsSensitive(t, s, "credentials")
	assertAttributeIsRequired(t, s, "spreadsheet_id")
	assertAttributeIsRequired(t, s, "bucket")
	assertAttributeIsRequired(t, s, "object_path")
	assertAttributeIsOptional(t, s, "export_format")
	assertAttributeIsComputed(t, s, "export_format")
	assertAttributeIsComputed(t, s, "gcs_object_url")
	assertAttributeIsComputed(t, s, "last_backup")
}

// --- All resources have credentials ---

func TestAllResources_HaveCredentialsAttribute(t *testing.T) {
	t.Parallel()

	resources := []struct {
		name    string
		factory func() resource.Resource
	}{
		{"spreadsheet", NewSpreadsheetResource},
		{"sheet", NewSheetResource},
		{"permission", NewPermissionResource},
		{"spreadsheet_backup", NewSpreadsheetBackupResource},
	}

	for _, tc := range resources {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := getResourceSchema(t, tc.factory())
			assertHasAttribute(t, s, "credentials")
			assertAttributeIsOptional(t, s, "credentials")
			assertAttributeIsSensitive(t, s, "credentials")
		})
	}
}

// --- All resources have a description ---

func TestAllResources_HaveDescription(t *testing.T) {
	t.Parallel()

	resources := []struct {
		name    string
		factory func() resource.Resource
	}{
		{"spreadsheet", NewSpreadsheetResource},
		{"sheet", NewSheetResource},
		{"permission", NewPermissionResource},
		{"spreadsheet_backup", NewSpreadsheetBackupResource},
	}

	for _, tc := range resources {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := getResourceSchema(t, tc.factory())
			if s.Description == "" {
				t.Errorf("expected non-empty description for resource %q", tc.name)
			}
		})
	}
}
