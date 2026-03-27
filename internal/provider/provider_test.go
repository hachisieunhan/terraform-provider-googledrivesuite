package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// testProviderFactories returns provider factories for acceptance-style tests.
func testProviderFactories() map[string]func() (provider.Provider, error) {
	return map[string]func() (provider.Provider, error){
		"googledrivesuite": func() (provider.Provider, error) {
			return New("test")(), nil
		},
	}
}

func TestProviderMetadata(t *testing.T) {
	t.Parallel()

	p := New("1.2.3")()
	resp := &provider.MetadataResponse{}
	p.Metadata(context.Background(), provider.MetadataRequest{}, resp)

	if resp.TypeName != "googledrivesuite" {
		t.Errorf("expected TypeName 'googledrivesuite', got %q", resp.TypeName)
	}
	if resp.Version != "1.2.3" {
		t.Errorf("expected Version '1.2.3', got %q", resp.Version)
	}
}

func TestProviderSchema_HasNoAttributes(t *testing.T) {
	t.Parallel()

	p := New("test")()
	resp := &provider.SchemaResponse{}
	p.Schema(context.Background(), provider.SchemaRequest{}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %s", resp.Diagnostics)
	}

	if len(resp.Schema.Attributes) != 0 {
		t.Errorf("expected empty provider schema attributes, got %d attributes", len(resp.Schema.Attributes))
	}
}

func TestProviderSchema_Description(t *testing.T) {
	t.Parallel()

	p := New("test")()
	resp := &provider.SchemaResponse{}
	p.Schema(context.Background(), provider.SchemaRequest{}, resp)

	if resp.Schema.Description == "" {
		t.Error("expected non-empty provider schema description")
	}
}

func TestProviderResources_RegistersFourResources(t *testing.T) {
	t.Parallel()

	p := New("test")()
	resources := p.Resources(context.Background())

	if len(resources) != 4 {
		t.Fatalf("expected 4 resources, got %d", len(resources))
	}

	// Verify each factory returns a non-nil resource.
	for i, factory := range resources {
		r := factory()
		if r == nil {
			t.Errorf("resource factory at index %d returned nil", i)
		}
	}
}

func TestProviderResources_TypeNames(t *testing.T) {
	t.Parallel()

	p := New("test")()
	resources := p.Resources(context.Background())

	expectedTypeNames := map[string]bool{
		"googledrivesuite_permission":         false,
		"googledrivesuite_spreadsheet":        false,
		"googledrivesuite_sheet":              false,
		"googledrivesuite_spreadsheet_backup": false,
	}

	for _, factory := range resources {
		r := factory()
		metaResp := &resource.MetadataResponse{}
		r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "googledrivesuite"}, metaResp)
		if _, ok := expectedTypeNames[metaResp.TypeName]; !ok {
			t.Errorf("unexpected resource type name: %q", metaResp.TypeName)
		}
		expectedTypeNames[metaResp.TypeName] = true
	}

	for name, found := range expectedTypeNames {
		if !found {
			t.Errorf("expected resource type %q was not registered", name)
		}
	}
}

func TestProviderDataSources_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	p := New("test")()
	dataSources := p.DataSources(context.Background())

	if len(dataSources) != 0 {
		t.Errorf("expected 0 data sources, got %d", len(dataSources))
	}
}

func TestProviderConfigure_NoErrors(t *testing.T) {
	t.Parallel()

	p := New("test")()
	resp := &provider.ConfigureResponse{}
	p.Configure(context.Background(), provider.ConfigureRequest{}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics during Configure: %s", resp.Diagnostics)
	}
}
