package templates

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTemplateOverride(t *testing.T) {
	// Create a temporary directory for test templates
	tempDir, err := os.MkdirTemp("", "test-templates-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create an override template
	overrideContent := `name: test-template
url: https://example.com
binaries:
  instructions: "echo 'This is an override template'"
`
	overridePath := filepath.Join(tempDir, "test-template.yaml")
	if err := os.WriteFile(overridePath, []byte(overrideContent), 0644); err != nil {
		t.Fatalf("Failed to write override template: %v", err)
	}

	// Test without template directory (should fail)
	_, err = Get("test-template")
	if err == nil {
		t.Error("Expected error when template doesn't exist, but got none")
	}

	// Set template directory and test override
	SetTemplateDir(tempDir)
	defer SetTemplateDir("") // Reset after test

	template, err := Get("test-template")
	if err != nil {
		t.Fatalf("Failed to get override template: %v", err)
	}

	if template.Name != "test-template" {
		t.Errorf("Expected template name 'test-template', got '%s'", template.Name)
	}

	if template.URL != "https://example.com" {
		t.Errorf("Expected URL 'https://example.com', got '%s'", template.URL)
	}
}

func TestBuiltInTemplateFallback(t *testing.T) {
	// Create a temporary directory for test templates (empty)
	tempDir, err := os.MkdirTemp("", "test-templates-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Set template directory to empty directory
	SetTemplateDir(tempDir)
	defer SetTemplateDir("") // Reset after test

	// Test that built-in templates are still accessible
	template, err := Get("jq")
	if err != nil {
		t.Fatalf("Failed to get built-in template: %v", err)
	}

	if template.Name != "jq" {
		t.Errorf("Expected template name 'jq', got '%s'", template.Name)
	}
}

func TestNoTemplateDirectory(t *testing.T) {
	// Ensure no template directory is set
	SetTemplateDir("")

	// Test that built-in templates work without template directory
	template, err := Get("jq")
	if err != nil {
		t.Fatalf("Failed to get built-in template without template dir: %v", err)
	}

	if template.Name != "jq" {
		t.Errorf("Expected template name 'jq', got '%s'", template.Name)
	}
}
