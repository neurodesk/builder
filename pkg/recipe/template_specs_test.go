package recipe

import (
	"strings"
	"testing"

	"github.com/neurodesk/builder/pkg/common"
)

func TestTemplateSpecContextArchExposedToUrls(t *testing.T) {
	templateSpec, err := getTemplateSpec("miniconda")
	if err != nil {
		t.Fatalf("Failed to get miniconda template spec: %v", err)
	}

	result, err := templateSpec.Execute(templateContext{
		PackageManager: common.PkgManagerApt,
		Arch:           "aarch64",
	}, func(k string) (any, bool, error) {
		switch k {
		case "method":
			return "binaries", true, nil
		case "version":
			return "latest", true, nil
		default:
			return nil, false, nil
		}
	})
	if err != nil {
		t.Fatalf("Failed to execute miniconda template spec: %v", err)
	}

	if got := result.Environment["PATH"]; got != "/opt/miniconda-latest/bin:$PATH" {
		t.Fatalf("Unexpected PATH env: %q", got)
	}

	want := "https://repo.anaconda.com/miniconda/Miniconda3-latest-Linux-aarch64.sh"
	if !strings.Contains(result.Instructions, want) {
		t.Fatalf("Expected rendered instructions to contain %q, got:\n%s", want, result.Instructions)
	}
}

func TestTemplateSpecOptionalArgumentCanOverrideInstallerVersion(t *testing.T) {
	templateSpec, err := getTemplateSpec("miniconda")
	if err != nil {
		t.Fatalf("Failed to get miniconda template spec: %v", err)
	}

	result, err := templateSpec.Execute(templateContext{
		PackageManager: common.PkgManagerApt,
		Arch:           "aarch64",
	}, func(k string) (any, bool, error) {
		switch k {
		case "method":
			return "binaries", true, nil
		case "version":
			return "4.12.0", true, nil
		case "installer_version":
			return "py37_4.12.0", true, nil
		default:
			return nil, false, nil
		}
	})
	if err != nil {
		t.Fatalf("Failed to execute miniconda template spec with installer override: %v", err)
	}

	want := "https://repo.anaconda.com/miniconda/Miniconda3-py37_4.12.0-Linux-aarch64.sh"
	if !strings.Contains(result.Instructions, want) {
		t.Fatalf("Expected rendered instructions to contain %q, got:\n%s", want, result.Instructions)
	}
}
