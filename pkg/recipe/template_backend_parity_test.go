package recipe

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/moby/buildkit/frontend/dockerfile/parser"
	"github.com/neurodesk/builder/pkg/ir"
)

func findNeurocontainersRepo(t *testing.T) string {
	t.Helper()

	if v := os.Getenv("NEUROCONTAINERS_DIR"); v != "" {
		if st, err := os.Stat(v); err == nil && st.IsDir() {
			return v
		}
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting cwd: %v", err)
	}

	dir := wd
	for {
		cand := filepath.Join(dir, "..", "..", "neurocontainers")
		if st, err := os.Stat(cand); err == nil && st.IsDir() {
			abs, err := filepath.Abs(cand)
			if err != nil {
				t.Fatalf("resolving neurocontainers path: %v", err)
			}
			return abs
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	t.Skip("neurocontainers repo not found; set NEUROCONTAINERS_DIR to enable parity test")
	return ""
}

func dockerfileForBackend(t *testing.T, recipePath, includeDir string, backend string) string {
	t.Helper()

	if err := SetTemplateBackend(backend); err != nil {
		t.Fatalf("setting template backend %q: %v", backend, err)
	}

	build, err := LoadBuildFile(recipePath)
	if err != nil {
		t.Fatalf("loading recipe %s: %v", recipePath, err)
	}

	def, _, err := build.GenerateWithStaging([]string{includeDir})
	if err != nil {
		t.Fatalf("generating recipe %s with backend %s: %v", recipePath, backend, err)
	}

	df, err := ir.GenerateDockerfile(def)
	if err != nil {
		t.Fatalf("rendering dockerfile for %s with backend %s: %v", recipePath, backend, err)
	}
	return df
}

func assertDockerfileParses(t *testing.T, recipeName, backend, dockerfile string) {
	t.Helper()

	if dockerfile == "" {
		t.Fatalf("empty dockerfile for %s with backend %s", recipeName, backend)
	}

	res, err := parser.Parse(bytes.NewBufferString(dockerfile))
	if err != nil {
		t.Fatalf("parsing dockerfile for %s with backend %s: %v", recipeName, backend, err)
	}
	if len(res.AST.Children) == 0 {
		t.Fatalf("dockerfile for %s with backend %s has no instructions", recipeName, backend)
	}
}

func TestTemplateBackendsGenerateValidDockerfilesAgainstNeurocontainers(t *testing.T) {
	repo := findNeurocontainersRepo(t)
	recipesDir := filepath.Join(repo, "recipes")

	defer func() {
		if err := SetTemplateBackend(string(TemplateBackendLegacy)); err != nil {
			t.Fatalf("restoring legacy template backend: %v", err)
		}
	}()

	var recipePaths []string
	if err := filepath.WalkDir(recipesDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Name() == "build.yaml" {
			recipePaths = append(recipePaths, filepath.Dir(path))
		}
		return nil
	}); err != nil {
		t.Fatalf("walking recipes: %v", err)
	}

	if len(recipePaths) == 0 {
		t.Fatalf("no build.yaml files found under %s", recipesDir)
	}

	for _, recipePath := range recipePaths {
		name := filepath.Base(recipePath)
		t.Run(name, func(t *testing.T) {
			legacy := dockerfileForBackend(t, recipePath, repo, string(TemplateBackendLegacy))
			macro := dockerfileForBackend(t, recipePath, repo, string(TemplateBackendMacro))
			assertDockerfileParses(t, name, string(TemplateBackendLegacy), legacy)
			assertDockerfileParses(t, name, string(TemplateBackendMacro), macro)
		})
	}
}
