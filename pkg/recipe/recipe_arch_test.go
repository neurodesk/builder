package recipe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neurodesk/builder/pkg/ir"
)

func TestGeneratePrefersHostArchitectureWhenRecipeSupportsIt(t *testing.T) {
	hostArch, ok := currentHostArchitecture()
	if !ok {
		t.Skip("host architecture is not mapped by the recipe package")
	}

	otherArch := CPUArchAMD64
	if hostArch == CPUArchAMD64 {
		otherArch = CPUArchARM64
	}

	dir := t.TempDir()
	buildYAML := `name: arch-pref
version: latest

architectures:
  - ` + string(otherArch) + `
  - ` + string(hostArch) + `

build:
  kind: neurodocker
  base-image: ubuntu:24.04
  pkg-manager: apt
  directives:
    - template:
        name: miniconda
        version: latest
`

	if err := os.WriteFile(filepath.Join(dir, "build.yaml"), []byte(buildYAML), 0o644); err != nil {
		t.Fatalf("writing build.yaml: %v", err)
	}

	build, err := LoadBuildFile(dir)
	if err != nil {
		t.Fatalf("loading build file: %v", err)
	}

	def, _, err := build.GenerateWithStaging(nil)
	if err != nil {
		t.Fatalf("generating build: %v", err)
	}

	dockerfile, err := ir.GenerateDockerfile(def)
	if err != nil {
		t.Fatalf("rendering dockerfile: %v", err)
	}

	want := "Miniconda3-latest-Linux-" + string(hostArch) + ".sh"
	if !strings.Contains(dockerfile, want) {
		t.Fatalf("expected dockerfile to contain %q, got:\n%s", want, dockerfile)
	}
}
