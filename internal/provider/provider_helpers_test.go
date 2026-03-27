package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestResolveCredentials_ExplicitValue(t *testing.T) {
	t.Parallel()

	creds := types.StringValue(`{"type":"service_account","project_id":"test"}`)
	result, err := resolveCredentials(creds)

	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	expected := `{"type":"service_account","project_id":"test"}`
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestResolveCredentials_NullFallsBackToEnvVar(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", `{"type":"service_account","from":"env"}`)

	result, err := resolveCredentials(types.StringNull())
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	expected := `{"type":"service_account","from":"env"}`
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestResolveCredentials_UnknownFallsBackToEnvVar(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", `{"type":"service_account","from":"env"}`)

	result, err := resolveCredentials(types.StringUnknown())
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	expected := `{"type":"service_account","from":"env"}`
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestResolveCredentials_ExplicitValueTakesPrecedenceOverEnv(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", `{"type":"service_account","from":"env"}`)

	creds := types.StringValue(`{"type":"service_account","from":"explicit"}`)
	result, err := resolveCredentials(creds)

	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	expected := `{"type":"service_account","from":"explicit"}`
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestResolveCredentials_NullAndNoEnvReturnsError(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")

	_, err := resolveCredentials(types.StringNull())
	if err == nil {
		t.Fatal("expected error when credentials are null and env is empty, got nil")
	}
}

func TestResolveCredentials_EmptyStringReturnsError(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")

	_, err := resolveCredentials(types.StringValue(""))
	if err == nil {
		t.Fatal("expected error when credentials are empty string, got nil")
	}
}

func TestResolveCredentials_ErrorMessageContainsGuidance(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")

	_, err := resolveCredentials(types.StringNull())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	errMsg := err.Error()
	if !contains(errMsg, "credentials") {
		t.Errorf("expected error message to mention 'credentials', got: %s", errMsg)
	}
	if !contains(errMsg, "GOOGLE_APPLICATION_CREDENTIALS") {
		t.Errorf("expected error message to mention 'GOOGLE_APPLICATION_CREDENTIALS', got: %s", errMsg)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
