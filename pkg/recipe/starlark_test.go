package recipe

import (
	"testing"

	"github.com/neurodesk/builder/pkg/common"
	"github.com/neurodesk/builder/pkg/ir"
	"github.com/neurodesk/builder/pkg/jinja2"
)

func TestStarlarkDirectiveValidation(t *testing.T) {
	ctx := Context{}

	tests := []struct {
		name      string
		directive StarlarkDirective
		wantErr   bool
	}{
		{
			name:      "empty directive",
			directive: StarlarkDirective{},
			wantErr:   true, // Must have either script or file
		},
		{
			name: "valid script",
			directive: StarlarkDirective{
				Script: jinja2.TemplateString("x = 1 + 1"),
			},
			wantErr: false,
		},
		{
			name: "valid file",
			directive: StarlarkDirective{
				File: "script.star",
			},
			wantErr: false,
		},
		{
			name: "both script and file",
			directive: StarlarkDirective{
				Script: jinja2.TemplateString("x = 1"),
				File:   "script.star",
			},
			wantErr: true, // Can't have both
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.directive.Validate(ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("StarlarkDirective.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestStarlarkDirectiveApply(t *testing.T) {
	// Create a test context
	ctx := newContext(
		common.PkgManagerApt,
		"1.0.0",
		[]string{}, // no include directories for this test
		ir.New(),
		nil,
	)

	// Test simple script execution
	directive := StarlarkDirective{
		Script: jinja2.TemplateString(`
# Set some variables
package_name = "test-package"
version_info = "1.0.0"

# Call built-in functions
install_packages("curl", "wget")
set_variable("computed_name", package_name + "-" + version_info)
`),
	}

	err := directive.Apply(ctx)
	if err != nil {
		t.Fatalf("StarlarkDirective.Apply() error = %v", err)
	}

	// Check that variables were set
	if val, ok := ctx.variables["computed_name"]; ok {
		if str, ok := val.(jinja2.StringValue); ok {
			if string(str) != "test-package-1.0.0" {
				t.Errorf("Expected computed_name='test-package-1.0.0', got %v", string(str))
			}
		} else {
			t.Errorf("Expected computed_name to be StringValue, got %T", val)
		}
	} else {
		t.Error("Expected computed_name variable to be set")
	}
}

func TestStarlarkDirectiveWithTemplating(t *testing.T) {
	// Create a test context with some variables
	ctx := newContext(
		common.PkgManagerApt,
		"2.0.0",
		[]string{},
		ir.New(),
		nil,
	)
	ctx.SetVariable("base_name", "myapp")

	// Test script with context object access
	directive := StarlarkDirective{
		Script: jinja2.TemplateString(`
# Use context variables via context object
app_name = context.base_name
version = context.version

# Create computed values
full_name = app_name + "-v" + version
set_variable("full_app_name", full_name)

# Install packages based on app name
install_packages(app_name + "-dev", app_name + "-tools")
`),
	}

	err := directive.Apply(ctx)
	if err != nil {
		t.Fatalf("StarlarkDirective.Apply() error = %v", err)
	}

	// Check that the computed variable was set correctly
	if val, ok := ctx.variables["full_app_name"]; ok {
		if str, ok := val.(jinja2.StringValue); ok {
			expected := "myapp-v2.0.0"
			if string(str) != expected {
				t.Errorf("Expected full_app_name=%q, got %q", expected, string(str))
			}
		} else {
			t.Errorf("Expected full_app_name to be StringValue, got %T", val)
		}
	} else {
		t.Error("Expected full_app_name variable to be set")
	}
}

func TestStarlarkDirectiveComplexScript(t *testing.T) {
	ctx := newContext(
		common.PkgManagerApt,
		"1.5.0",
		[]string{},
		ir.New(),
		nil,
	)
	ctx.SetVariable("enable_dev_tools", true)
	ctx.SetVariable("target_arch", "amd64")

	directive := StarlarkDirective{
		Script: jinja2.TemplateString(`
def install_all_packages():
    if context.enable_dev_tools:
        install_packages("build-essential", "cmake")
    
    if context.target_arch == "amd64":
        install_packages("libc6-dev")
    
    # Set computed configuration
    config_name = "build-config-" + context.target_arch
    if context.enable_dev_tools:
        config_name = config_name + "-dev"
    
    set_variable("build_config", config_name)

# Call the function
install_all_packages()
`),
	}

	err := directive.Apply(ctx)
	if err != nil {
		t.Fatalf("StarlarkDirective.Apply() error = %v", err)
	}

	// Check the computed build config
	if val, ok := ctx.variables["build_config"]; ok {
		if str, ok := val.(jinja2.StringValue); ok {
			expected := "build-config-amd64-dev"
			if string(str) != expected {
				t.Errorf("Expected build_config=%q, got %q", expected, string(str))
			}
		} else {
			t.Errorf("Expected build_config to be StringValue, got %T", val)
		}
	} else {
		t.Error("Expected build_config variable to be set")
	}
}

func TestStarlarkContextObjectAccess(t *testing.T) {
	ctx := newContext(
		common.PkgManagerApt,
		"1.2.3",
		[]string{},
		ir.New(),
		nil,
	)
	ctx.SetVariable("test_var", "test_value")
	ctx.SetVariable("boolean_var", true)

	directive := StarlarkDirective{
		Script: jinja2.TemplateString(`
# Test accessing variables via context object
version_from_context = context.version
test_from_context = context.test_var
bool_from_context = context.boolean_var

# Test accessing the same via local object
version_from_local = local.version
test_from_local = local.test_var

# Set variables based on context access
set_variable("accessed_version", version_from_context)
set_variable("accessed_test", test_from_context)
set_variable("accessed_bool", bool_from_context)

# Verify local and context are the same
def check_objects_match():
    if version_from_context == version_from_local and test_from_context == test_from_local:
        set_variable("objects_match", True)
    else:
        set_variable("objects_match", False)

check_objects_match()
`),
	}

	err := directive.Apply(ctx)
	if err != nil {
		t.Fatalf("StarlarkDirective.Apply() error = %v", err)
	}

	// Check that variables were accessed correctly
	if val, ok := ctx.variables["accessed_version"]; ok {
		if str, ok := val.(jinja2.StringValue); ok {
			if string(str) != "1.2.3" {
				t.Errorf("Expected accessed_version='1.2.3', got %q", string(str))
			}
		} else {
			t.Errorf("Expected accessed_version to be StringValue, got %T", val)
		}
	} else {
		t.Error("Expected accessed_version variable to be set")
	}

	if val, ok := ctx.variables["accessed_test"]; ok {
		if str, ok := val.(jinja2.StringValue); ok {
			if string(str) != "test_value" {
				t.Errorf("Expected accessed_test='test_value', got %q", string(str))
			}
		} else {
			t.Errorf("Expected accessed_test to be StringValue, got %T", val)
		}
	} else {
		t.Error("Expected accessed_test variable to be set")
	}

	if val, ok := ctx.variables["accessed_bool"]; ok {
		if boolVal, ok := val.(jinja2.BoolValue); ok {
			if !bool(boolVal) {
				t.Errorf("Expected accessed_bool=true, got %v", bool(boolVal))
			}
		} else {
			t.Errorf("Expected accessed_bool to be BoolValue, got %T", val)
		}
	} else {
		t.Error("Expected accessed_bool variable to be set")
	}

	if val, ok := ctx.variables["objects_match"]; ok {
		if boolVal, ok := val.(jinja2.BoolValue); ok {
			if !bool(boolVal) {
				t.Error("Expected context and local objects to provide the same values")
			}
		} else {
			t.Errorf("Expected objects_match to be BoolValue, got %T", val)
		}
	} else {
		t.Error("Expected objects_match variable to be set")
	}
}

func TestStarlarkDirectiveFullFunctionality(t *testing.T) {
	ctx := newContext(
		common.PkgManagerApt,
		"1.0.0",
		[]string{},
		ir.New(),
		nil,
	)
	ctx.SetVariable("app_name", "myapp")
	ctx.SetVariable("enable_ssl", true)

	directive := StarlarkDirective{
		Script: jinja2.TemplateString(`
# Test comprehensive Starlark functionality
app = context.app_name
version = context.version

# Install base packages
install_packages("curl", "wget")

# Install app-specific packages  
install_packages(app + "-dev", app + "-client")

# Set environment variables
set_environment("APP_NAME", app)
set_environment("APP_VERSION", version)

def setup_ssl():
    if context.enable_ssl:
        install_packages("openssl", "ca-certificates")
        set_environment("ENABLE_SSL", "true")
        run_command("openssl version")

setup_ssl()

# Set computed variables
full_app_name = app + "-v" + version
set_variable("full_app_name", full_app_name)

# Run final setup
run_command("echo 'Setup complete for " + full_app_name + "'")
`),
	}

	err := directive.Apply(ctx)
	if err != nil {
		t.Fatalf("StarlarkDirective.Apply() error = %v", err)
	}

	// Check computed variable
	if val, ok := ctx.variables["full_app_name"]; ok {
		if str, ok := val.(jinja2.StringValue); ok {
			expected := "myapp-v1.0.0"
			if string(str) != expected {
				t.Errorf("Expected full_app_name=%q, got %q", expected, string(str))
			}
		} else {
			t.Errorf("Expected full_app_name to be StringValue, got %T", val)
		}
	} else {
		t.Error("Expected full_app_name variable to be set")
	}

	// Verify the builder was modified by checking the compiled definition
	def, err := ctx.Compile()
	if err != nil {
		t.Fatalf("Failed to compile context: %v", err)
	}

	// Check that commands and environment variables were added
	// Note: This is a basic check - in a real test you'd verify the specific commands/env vars
	if len(def.Directives) == 0 {
		t.Error("Expected some build directives to be generated")
	}
}

func TestStarlarkRunCommandAccumulation(t *testing.T) {
	ctx := newContext(
		common.PkgManagerApt,
		"1.0.0",
		[]string{},
		ir.New(),
		nil,
	)

	directive := StarlarkDirective{
		Script: jinja2.TemplateString(`
run_command("echo one")
run_command("echo two")
`),
	}

	if err := directive.Apply(ctx); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	def, err := ctx.Compile()
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	var runCount int
	for _, d := range def.Directives {
		switch d.(type) {
		case ir.RunDirective, ir.RunWithMountsDirective:
			runCount++
		}
	}
	if runCount < 2 {
		t.Fatalf("expected at least 2 RUN directives, got %d", runCount)
	}
}
