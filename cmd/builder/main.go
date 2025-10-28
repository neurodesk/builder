package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/moby/buildkit/frontend/dockerfile/parser"
	"github.com/neurodesk/builder/pkg/ir"
	"github.com/neurodesk/builder/pkg/netcache"
	"github.com/neurodesk/builder/pkg/recipe"
	"github.com/neurodesk/builder/pkg/templates"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v4"
)

type builderConfig struct {
	RecipeRoots []string `yaml:"recipe_roots"`
	IncludeDirs []string `yaml:"include_dirs"`
	TemplateDir string   `yaml:"template_dir,omitempty"`
}

func (b *builderConfig) getRecipeByName(name string) (*recipe.BuildFile, error) {
	for _, root := range b.RecipeRoots {
		// look for a directory with the name of the recipe
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			return recipe.LoadBuildFile(filepath.Join(root, name))
		}
	}
	return nil, fmt.Errorf("recipe not found: %s", name)
}

func (b *builderConfig) loadConfig(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := yaml.NewDecoder(f).Decode(b); err != nil {
		return fmt.Errorf("decoding config file: %w", err)
	}
	return nil
}

var rootBuilderConfig string
var testCaptureOutput bool
var verbose bool
var graphOutputPath string

var rootCmd = cobra.Command{
	Use:   "builder",
	Short: "A tool to build container images from recipes",
}

var generateDockerfileCmd = cobra.Command{
	Use:   "generate [recipe]",
	Short: "Generate a Dockerfile for the specified recipe",
	RunE: func(cmd *cobra.Command, args []string) error {
		if verbose {
			os.Setenv("BUILDER_VERBOSE", "1")
		}
		if len(args) == 0 {
			return fmt.Errorf("no recipe specified")
		}
		recipeName := args[0]

		cfg, err := loadBuilderConfig()
		if err != nil {
			return err
		}

		build, err := cfg.getRecipeByName(recipeName)
		if err != nil {
			return err
		}

		out, _, err := build.GenerateWithStaging(cfg.IncludeDirs)
		if err != nil {
			return fmt.Errorf("generating build IR: %w", err)
		}

		dockerfile, err := ir.GenerateDockerfile(out)
		if err != nil {
			return fmt.Errorf("generating dockerfile: %w", err)
		}

		fmt.Println(dockerfile)
		return nil
	},
}

// helper: load config and apply template dir
func loadBuilderConfig() (builderConfig, error) {
	var cfg builderConfig
	if err := cfg.loadConfig(rootBuilderConfig); err != nil {
		return cfg, fmt.Errorf("loading config: %w", err)
	}
	if cfg.TemplateDir != "" {
		templates.SetTemplateDir(cfg.TemplateDir)
	}
	return cfg, nil
}

// helper: resolve a recipe spec to a directory using configured roots
func resolveRecipePath(cfg builderConfig, spec string) (string, error) {
	if strings.ContainsRune(spec, os.PathSeparator) || strings.HasPrefix(spec, ".") || strings.HasPrefix(spec, "/") {
		return spec, nil
	}
	for _, root := range cfg.RecipeRoots {
		cand := filepath.Join(root, spec)
		if st, err := os.Stat(cand); err == nil && st.IsDir() {
			return cand, nil
		}
	}
	return "", fmt.Errorf("recipe not found: %s", spec)
}

// helper: copy a whole directory tree
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target, false)
	})
}

// helper: write a reader to a file path with optional exec bit
func writeFromReader(dst string, r io.Reader, exec bool) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if exec {
		mode = 0o755
	}
	tmp := dst + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, r); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

// helper: copy a single file path
func copyFile(src, dst string, exec bool) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	return writeFromReader(dst, in, exec)
}

// helper: try to hard link cache entries into the build context to avoid copying where possible
func linkOrCopyCacheFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.Remove(dst); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.Link(src, dst); err == nil {
		return nil
	}
	return copyFile(src, dst, false)
}

// helper: parse local flags into keys and kv pairs
func parseLocalFlags(lvals []string) (keys []string, kvs []string) {
	if len(lvals) == 0 {
		return nil, nil
	}
	kvs = append(kvs, lvals...)
	for _, kv := range lvals {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 && parts[0] != "" {
			keys = append(keys, parts[0])
		}
	}
	return keys, kvs
}

// helper: parse COPY directives into srcs/dest (best-effort; handles flags and JSON form)
type copySpec struct {
	Src  []string
	Dest string
}

func parseCopySpecs(dockerfile string) []copySpec {
	var specs []copySpec
	lines := strings.Split(dockerfile, "\n")
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		upper := strings.ToUpper(line)
		if !strings.HasPrefix(upper, "COPY ") {
			continue
		}
		// JSON form
		rest := strings.TrimSpace(line[len("COPY "):])
		if strings.HasPrefix(rest, "[") {
			// crude JSON array split
			end := strings.Index(rest, "]")
			if end <= 0 {
				continue
			}
			inner := rest[1:end]
			var parts []string
			for _, p := range strings.Split(inner, ",") {
				p = strings.TrimSpace(strings.Trim(p, "\""))
				if p != "" {
					parts = append(parts, p)
				}
			}
			if len(parts) >= 2 {
				specs = append(specs, copySpec{Src: parts[:len(parts)-1], Dest: parts[len(parts)-1]})
			}
			continue
		}
		// Shell form: split quoted and drop flags
		toks := splitQuoted(rest)
		// Drop leading --flag tokens
		i := 0
		for i < len(toks) && strings.HasPrefix(toks[i], "--") {
			i++
		}
		parts := toks[i:]
		if len(parts) >= 2 {
			specs = append(specs, copySpec{Src: parts[:len(parts)-1], Dest: parts[len(parts)-1]})
		}
	}
	return specs
}

// helper: stage cache/top-level files and COPY sources into the build context
func stageIntoBuildContext(cfg builderConfig, recipePath, dockerfile, buildDir string, plan *recipe.StagingPlan) error {
	// 1) stage plan files into cache/
	cacheDir := filepath.Join(buildDir, "cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return fmt.Errorf("creating cache dir: %w", err)
	}

	httpCacheDir := os.Getenv("BUILDER_HTTP_CACHE_DIR")
	if httpCacheDir == "" {
		httpCacheDir = filepath.Join("local", "httpcache")
	}

	if err := os.MkdirAll(httpCacheDir, 0o755); err != nil {
		return fmt.Errorf("creating http cache dir: %w", err)
	}

	hc := netcache.New(httpCacheDir)
	for _, f := range plan.Files {
		dst := filepath.Join(cacheDir, filepath.FromSlash(f.Name))
		switch {
		case f.HostFilename != "":
			src := f.HostFilename
			if !filepath.IsAbs(src) {
				cand := filepath.Join(recipePath, src)
				if _, err := os.Stat(cand); err == nil {
					src = cand
				} else {
					for _, d := range cfg.IncludeDirs {
						alt := filepath.Join(d, src)
						if _, err := os.Stat(alt); err == nil {
							src = alt
							break
						}
					}
				}
			}
			if verbose {
				fmt.Printf("[verbose] Staging local file %s -> %s\n", src, dst)
			}
			if err := copyFile(src, dst, f.Executable); err != nil {
				return fmt.Errorf("staging local file %q: %w", f.Name, err)
			}
		case f.URL != "":
			if verbose {
				fmt.Printf("[verbose] Downloading %s -> %s\n", f.URL, dst)
			}
			ctx := context.Background()
			localPath, fromCache, err := hc.Get(ctx, f.URL)
			if err != nil {
				return fmt.Errorf("fetching %q: %w", f.URL, err)
			}
			if verbose {
				if fromCache {
					fmt.Printf("[verbose] Using cached %s\n", localPath)
				} else {
					fmt.Printf("[verbose] Downloaded to cache %s\n", localPath)
				}
			}
			if err := copyFile(localPath, dst, f.Executable); err != nil {
				return fmt.Errorf("staging downloaded file %q: %w", f.URL, err)
			}
		default:
			if verbose {
				fmt.Printf("[verbose] Staging literal file %s (%d bytes) -> %s\n", f.Name, len(f.Contents), dst)
			}
			if err := writeFromReader(dst, strings.NewReader(f.Contents), f.Executable); err != nil {
				return fmt.Errorf("staging literal file %q: %w", f.Name, err)
			}
		}
	}

	// Build a set of virtual file names declared via files{} to support COPY of virtual files
	vset := map[string]struct{}{}
	for _, f := range plan.Files {
		if f.Name != "" {
			vset[f.Name] = struct{}{}
		}
	}

	// 2) stage COPY sources into build context (relative to recipe dir)
	baseDirAbs, _ := filepath.Abs(recipePath)
	buildDirAbs, _ := filepath.Abs(buildDir)
	for _, spec := range parseCopySpecs(dockerfile) {
		for _, srcRel := range spec.Src {
			// Normalize to forward slashes for checks
			srcNorm := strings.TrimPrefix(strings.ReplaceAll(srcRel, "\\", "/"), "./")

			// Support COPY from virtual cache using either absolute get_file path, or cache/<name>
			if filepath.IsAbs(srcRel) {
				if strings.HasPrefix(srcNorm, "/.neurocontainer-cache/") {
					name := strings.TrimPrefix(srcNorm, "/.neurocontainer-cache/")
					srcRel = "cache/" + name
					srcNorm = srcRel
				} else {
					return fmt.Errorf("absolute COPY sources are not allowed: %q", srcRel)
				}
			}

			// Destination in build context is buildDir/<srcRel>
			bcPath := filepath.Join(buildDir, filepath.FromSlash(srcRel))
			// keep inside buildDir
			bcAbs := bcPath
			if abs, err := filepath.Abs(bcPath); err == nil {
				bcAbs = abs
			}
			if rel, err := filepath.Rel(buildDirAbs, bcAbs); err != nil || strings.HasPrefix(rel, "..") {
				return fmt.Errorf("COPY destination path escapes build context: %q", srcRel)
			}

			// Handle virtual cache/<name> paths declared via files{}; otherwise fall through to real file handling
			if strings.HasPrefix(srcNorm, "cache/") {
				name := strings.TrimPrefix(srcNorm, "cache/")
				if _, ok := vset[name]; ok {
					cacheSrc := filepath.Join(buildDir, "cache", filepath.FromSlash(name))
					if _, err := os.Stat(cacheSrc); err != nil {
						return fmt.Errorf("COPY source %q refers to missing staged cache file %q", srcRel, cacheSrc)
					}
					// No additional copy needed; Docker build will read from buildDir/cache/...
					continue
				}
			}

			// Handle bare virtual names (no slash) declared via files{}: materialize into build context
			if !strings.Contains(srcNorm, "/") {
				if _, ok := vset[srcNorm]; ok {
					cacheSrc := filepath.Join(buildDir, "cache", filepath.FromSlash(srcNorm))
					if st, err := os.Stat(cacheSrc); err != nil || st.IsDir() {
						return fmt.Errorf("virtual COPY source %q not found in staged cache at %q", srcRel, cacheSrc)
					}
					if verbose {
						fmt.Printf("[verbose] Materializing virtual file %s -> %s\n", cacheSrc, bcPath)
					}
					if err := linkOrCopyCacheFile(cacheSrc, bcPath); err != nil {
						return fmt.Errorf("copying virtual file %q into build context: %w", srcRel, err)
					}
					continue
				}
			}

			// Fallback: treat as real file from the recipe directory
			src := filepath.Join(baseDirAbs, filepath.FromSlash(srcRel))
			srcEval, err := filepath.EvalSymlinks(src)
			if err != nil {
				return fmt.Errorf("COPY source %q not found in recipe directory", srcRel)
			}
			if rel, err := filepath.Rel(baseDirAbs, srcEval); err != nil || strings.HasPrefix(rel, "..") {
				return fmt.Errorf("COPY source %q is outside the recipe directory", srcRel)
			}
			st, err := os.Stat(srcEval)
			if err != nil {
				return fmt.Errorf("COPY source %q not found in recipe directory", srcRel)
			}
			if st.IsDir() {
				if verbose {
					fmt.Printf("[verbose] Copying directory %s -> %s\n", srcEval, bcPath)
				}
				if err := copyDir(srcEval, bcPath); err != nil {
					return fmt.Errorf("copying directory %q into build context: %w", srcRel, err)
				}
			} else {
				if verbose {
					fmt.Printf("[verbose] Copying file %s -> %s\n", srcEval, bcPath)
				}
				if err := copyFile(srcEval, bcPath, false); err != nil {
					return fmt.Errorf("copying file %q into build context: %w", srcRel, err)
				}
			}
		}
	}

	return nil
}

type dockerStageResult struct {
	Name           string   `json:"name"`
	Version        string   `json:"version"`
	Tag            string   `json:"tag"`
	Arch           string   `json:"arch"`
	BuildDir       string   `json:"build_dir"`
	DockerfilePath string   `json:"dockerfile"`
	CacheDir       string   `json:"cache_dir"`
	LocalContext   []string `json:"local_context,omitempty"`
	Dockerfile     string   `json:"-"`
}

type compiledRecipe struct {
	Path       string
	Build      *recipe.BuildFile
	Definition *ir.Definition
	Plan       *recipe.StagingPlan
	Dockerfile string
}

type recipeGenerationResult struct {
	Compiled   *compiledRecipe
	OutputPath string
	Errors     []string
}

type genericStageResult struct {
	cfg        builderConfig
	recipePath string
	irDef      *ir.Definition
	build      *recipe.BuildFile
	plan       *recipe.StagingPlan
	locals     []string
}

// helper: generate, render, write dockerfile, and stage files/COPYs
func prepareStage(cfg builderConfig, recipeSpec string, locals []string) (*genericStageResult, error) {
	recipePath, err := resolveRecipePath(cfg, recipeSpec)
	if err != nil {
		return nil, err
	}

	build, err := recipe.LoadBuildFile(recipePath)
	if err != nil {
		return nil, fmt.Errorf("loading build file: %w", err)
	}

	// local keys for named contexts
	keys, _ := parseLocalFlags(locals)

	irDef, plan, err := build.GenerateWithStagingAndLocals(cfg.IncludeDirs, keys)
	if err != nil {
		return nil, fmt.Errorf("generating build IR: %w", err)
	}

	return &genericStageResult{
		cfg:        cfg,
		recipePath: recipePath,
		irDef:      irDef,
		build:      build,
		plan:       plan,
		locals:     keys,
	}, nil
}

func prepareDockerStage(stage *genericStageResult) (*dockerStageResult, error) {
	build := stage.build

	dockerfile, err := ir.GenerateDockerfile(stage.irDef)
	if err != nil {
		return nil, fmt.Errorf("generating dockerfile: %w", err)
	}

	if strings.Contains(dockerfile, "\" + ") {
		return nil, fmt.Errorf("detected unrendered string concatenation in generated Dockerfile; fix recipe/templates")
	}

	// Write Dockerfile
	buildDir := filepath.Join("local", "build", build.Name)
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating build directory: %w", err)
	}

	dockerfilePath := filepath.Join(buildDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(dockerfile), 0o644); err != nil {
		return nil, fmt.Errorf("writing Dockerfile: %w", err)
	}

	// Stage files
	if err := stageIntoBuildContext(stage.cfg, stage.recipePath, dockerfile, buildDir, stage.plan); err != nil {
		return nil, err
	}

	return &dockerStageResult{
		Name:           build.Name,
		Version:        build.Version,
		Tag:            build.Name + ":" + build.Version,
		Arch:           string(build.Architectures[0]),
		BuildDir:       buildDir,
		DockerfilePath: dockerfilePath,
		CacheDir:       filepath.Join(buildDir, "cache"),
		LocalContext:   stage.locals,
		Dockerfile:     dockerfile,
	}, nil
}

func compileRecipe(cfg builderConfig, recipeDir string) (*compiledRecipe, error) {
	build, err := recipe.LoadBuildFile(recipeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load build file: %w", err)
	}
	def, plan, err := build.GenerateWithStaging(cfg.IncludeDirs)
	if err != nil {
		return nil, fmt.Errorf("failed to generate IR: %w", err)
	}
	dockerfile, err := ir.GenerateDockerfile(def)
	if err != nil {
		return nil, fmt.Errorf("failed to generate dockerfile: %w", err)
	}
	return &compiledRecipe{
		Path:       recipeDir,
		Build:      build,
		Definition: def,
		Plan:       plan,
		Dockerfile: dockerfile,
	}, nil
}

func generateDockerfileForRecipe(cfg builderConfig, recipeDir, outputDir string) (*recipeGenerationResult, error) {
	compiled, err := compileRecipe(cfg, recipeDir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating output directory: %w", err)
	}
	outputPath := filepath.Join(outputDir, fmt.Sprintf("%s_%s.Dockerfile", compiled.Build.Name, compiled.Build.Version))
	if err := os.WriteFile(outputPath, []byte(compiled.Dockerfile), 0o644); err != nil {
		return nil, fmt.Errorf("writing dockerfile: %w", err)
	}
	issues := validateCompiledRecipe(cfg, compiled)
	return &recipeGenerationResult{
		Compiled:   compiled,
		OutputPath: outputPath,
		Errors:     issues,
	}, nil
}

func validateCompiledRecipe(cfg builderConfig, compiled *compiledRecipe) []string {
	if compiled == nil {
		return []string{"internal error: nil compiled recipe"}
	}
	if _, err := parser.Parse(strings.NewReader(compiled.Dockerfile)); err != nil {
		return []string{fmt.Sprintf("BuildKit parser validation failed: %v", err)}
	}
	var issues []string
	missing := false
	for _, f := range compiled.Plan.Files {
		if f.HostFilename == "" {
			continue
		}
		src := f.HostFilename
		var candidates []string
		if filepath.IsAbs(src) {
			candidates = []string{src}
		} else {
			candidates = append(candidates, filepath.Join(compiled.Path, src))
			for _, d := range cfg.IncludeDirs {
				candidates = append(candidates, filepath.Join(d, src))
			}
		}
		found := false
		for _, cand := range candidates {
			if st, err := os.Stat(cand); err == nil && !st.IsDir() {
				found = true
				break
			}
		}
		if !found {
			missing = true
			issues = append(issues, fmt.Sprintf("Missing file referenced by files: %s (searched: %s)", src, strings.Join(candidates, ", ")))
		}
	}

	vset := map[string]struct{}{}
	for _, f := range compiled.Plan.Files {
		if f.Name != "" {
			vset[f.Name] = struct{}{}
		}
	}

	for _, spec := range parseCopySpecs(compiled.Dockerfile) {
		for _, srcRel := range spec.Src {
			srcNorm := strings.TrimPrefix(strings.ReplaceAll(srcRel, "\\", "/"), "./")
			if filepath.IsAbs(srcRel) {
				if after, ok := strings.CutPrefix(srcNorm, "/.neurocontainer-cache/"); ok {
					if _, ok := vset[after]; !ok {
						missing = true
						issues = append(issues, fmt.Sprintf("COPY references virtual file not declared: %s (name %s)", srcRel, after))
					}
					continue
				}
				missing = true
				issues = append(issues, fmt.Sprintf("Invalid absolute COPY source: %s", srcRel))
				continue
			}

			if after, ok := strings.CutPrefix(srcNorm, "cache/"); ok {
				if _, ok := vset[after]; !ok {
					missing = true
					issues = append(issues, fmt.Sprintf("COPY references unknown cache file: %s (name %s)", srcRel, after))
				}
				continue
			}

			if !strings.Contains(srcNorm, "/") {
				if _, ok := vset[srcNorm]; ok {
					continue
				}
			}

			srcPath := filepath.Join(compiled.Path, filepath.FromSlash(srcRel))
			eval, err := filepath.EvalSymlinks(srcPath)
			if err != nil {
				missing = true
				issues = append(issues, fmt.Sprintf("COPY source not found: %s (from %s)", srcRel, srcPath))
				continue
			}
			baseAbs, _ := filepath.Abs(compiled.Path)
			evalAbs, _ := filepath.Abs(eval)
			if rel, err := filepath.Rel(baseAbs, evalAbs); err != nil || strings.HasPrefix(rel, "..") {
				missing = true
				issues = append(issues, fmt.Sprintf("COPY source escapes recipe directory: %s -> %s", srcRel, eval))
				continue
			}
			if _, err := os.Stat(eval); err != nil {
				missing = true
				issues = append(issues, fmt.Sprintf("COPY source missing: %s (resolved %s)", srcRel, eval))
			}
		}
	}

	if missing {
		issues = append(issues, "One or more required files are missing")
	}
	return issues
}

func buildTesterBinary(goarch string) (string, func(), error) {
	tmpDir, err := os.MkdirTemp("", "builder-tester-")
	if err != nil {
		return "", nil, fmt.Errorf("creating temp dir for tester: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tmpDir) }
	outputPath := filepath.Join(tmpDir, "tester")
	args := []string{"build", "-o", outputPath, "./cmd/tester"}
	cmd := exec.Command("go", args...)
	cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+goarch)
	if verbose {
		fmt.Printf("Building tester binary (GOARCH=%s)\n", goarch)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("building tester binary: %w\n%s", err, string(out))
	}
	abs, err := filepath.Abs(outputPath)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("resolving tester path: %w", err)
	}
	return abs, cleanup, nil
}

func goArchFromRecipe(b *recipe.BuildFile) (string, error) {
	arch := recipe.CPUArchAMD64
	if len(b.Architectures) > 0 {
		arch = b.Architectures[0]
	}
	switch arch {
	case recipe.CPUArchAMD64:
		return "amd64", nil
	case recipe.CPUArchARM64:
		return "arm64", nil
	default:
		return "", fmt.Errorf("unsupported architecture %q", arch)
	}
}

func runTesterInContainer(tag, testerPath, platform string, captureOutput bool) ([]byte, error) {
	mount := fmt.Sprintf("%s:/tester/tester:ro", testerPath)
	args := []string{"run", "--rm"}
	if platform != "" {
		args = append(args, "--platform", platform)
	}
	args = append(args, "-v", mount, "--entrypoint", "/tester/tester", tag)
	if captureOutput {
		args = append(args, "--capture-output")
	}
	cmd := exec.Command("docker", args...)
	return cmd.CombinedOutput()
}

var testCmd = cobra.Command{
	Use:   "test [recipe]",
	Short: "Run the deployment tester inside the built container",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if verbose {
			os.Setenv("BUILDER_VERBOSE", "1")
		}
		if _, err := exec.LookPath("docker"); err != nil {
			return fmt.Errorf("docker CLI not found in PATH; please install Docker and rerun")
		}

		recipeSpec := args[0]
		cfg, err := loadBuilderConfig()
		if err != nil {
			return err
		}
		recipePath, err := resolveRecipePath(cfg, recipeSpec)
		if err != nil {
			return err
		}
		build, err := recipe.LoadBuildFile(recipePath)
		if err != nil {
			return fmt.Errorf("loading build file: %w", err)
		}
		goarch, err := goArchFromRecipe(build)
		if err != nil {
			return err
		}
		testerPath, cleanup, err := buildTesterBinary(goarch)
		if err != nil {
			return err
		}
		defer cleanup()

		tag := build.Name + ":" + build.Version
		inspect := exec.Command("docker", "image", "inspect", tag)
		if out, err := inspect.CombinedOutput(); err != nil {
			return fmt.Errorf("docker image %s not found: %w\n%s", tag, err, string(out))
		}

		platform := "linux/" + goarch
		output, err := runTesterInContainer(tag, testerPath, platform, testCaptureOutput)
		fmt.Print(string(output))
		if err != nil {
			return fmt.Errorf("tester reported failure: %w", err)
		}
		return nil
	},
}

// stageCmd prepares the build context (Dockerfile + staged files) but does not build.
// It emits a small JSON blob with details so wrapper scripts can invoke BuildKit.
var stageCmd = cobra.Command{
	Use:   "stage [recipe]",
	Short: "Generate Dockerfile and stage build context (no build)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if verbose {
			os.Setenv("BUILDER_VERBOSE", "1")
		}
		if len(args) == 0 {
			return fmt.Errorf("no recipe specified")
		}
		recipeName := args[0]

		// Parse optional local contexts supplied as --local KEY=DIR and pass keys to generator
		var locals []string
		if lvals, _ := cmd.Flags().GetStringArray("local"); len(lvals) > 0 {
			locals = append(locals, lvals...)
		}
		cfg, err := loadBuilderConfig()
		if err != nil {
			return err
		}
		res, err := prepareStage(cfg, recipeName, locals)
		if err != nil {
			return err
		}
		b, err := json.Marshal(res)
		if err != nil {
			return err
		}
		os.Stdout.Write(b)
		os.Stdout.Write([]byte("\n"))
		return nil
	},
}

func testRecipes(recipes []string) error {
	cfg, err := loadBuilderConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	outputDir := filepath.Join("local", "docker")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	var (
		success int
		failed  int
	)

	for _, r := range recipes {
		fmt.Printf("Testing recipe: %s\n", r)
		res, err := generateDockerfileForRecipe(cfg, r, outputDir)
		if err != nil {
			failed++
			fmt.Printf("\033[31m  %v\033[0m\n", err)
			continue
		}
		if len(res.Errors) > 0 {
			failed++
			for _, msg := range res.Errors {
				fmt.Printf("\033[31m  %s\033[0m\n", msg)
			}
			continue
		}
		fmt.Printf("\033[32m  Successfully generated Dockerfile: %s\033[0m\n", res.OutputPath)
		success++
	}

	fmt.Printf("Tested %d recipes: %d succeeded, %d failed\n", len(recipes), success, failed)
	if failed > 0 {
		return fmt.Errorf("%d recipes failed", failed)
	}
	return nil
}

func listRecipes(cfg builderConfig) ([]string, error) {
	var recipes []string
	for _, root := range cfg.RecipeRoots {
		entries, err := os.ReadDir(root)
		if err != nil {
			return nil, fmt.Errorf("reading recipe root %s: %w", root, err)
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			if _, err := os.Stat(filepath.Join(root, entry.Name(), "build.yaml")); err != nil {
				continue
			}
			recipes = append(recipes, filepath.Join(root, entry.Name()))
		}
	}
	sort.Strings(recipes)
	return recipes, nil
}

var testAllCmd = cobra.Command{
	Use:   "test-all",
	Short: "Test all recipes in the configured recipe roots",
	RunE: func(cmd *cobra.Command, args []string) error {
		if verbose {
			os.Setenv("BUILDER_VERBOSE", "1")
		}
		cfg, err := loadBuilderConfig()
		if err != nil {
			return err
		}
		recipes, err := listRecipes(cfg)
		if err != nil {
			return err
		}
		return testRecipes(recipes)
	},
}

var graphCmd = cobra.Command{
	Use:   "graph [recipe...]",
	Short: "Generate all Dockerfiles and emit a hashed layer Graphviz graph",
	RunE: func(cmd *cobra.Command, args []string) error {
		if verbose {
			os.Setenv("BUILDER_VERBOSE", "1")
		}
		cfg, err := loadBuilderConfig()
		if err != nil {
			return err
		}

		var recipeDirs []string
		if len(args) > 0 {
			for _, spec := range args {
				path, err := resolveRecipePath(cfg, spec)
				if err != nil {
					return err
				}
				recipeDirs = append(recipeDirs, path)
			}
		} else {
			recipeDirs, err = listRecipes(cfg)
			if err != nil {
				return err
			}
		}

		if len(recipeDirs) == 0 {
			return fmt.Errorf("no recipes to process")
		}

		outputDir := filepath.Join("local", "docker")
		results := make([]*recipeGenerationResult, 0, len(recipeDirs))
		var failures []string
		for _, r := range recipeDirs {
			fmt.Printf("Processing recipe: %s\n", r)
			res, err := generateDockerfileForRecipe(cfg, r, outputDir)
			if err != nil {
				failures = append(failures, fmt.Sprintf("%s: %v", r, err))
				fmt.Printf("\033[31m  %v\033[0m\n", err)
				continue
			}
			if len(res.Errors) > 0 {
				failures = append(failures, fmt.Sprintf("%s: %s", r, strings.Join(res.Errors, "; ")))
				for _, msg := range res.Errors {
					fmt.Printf("\033[31m  %s\033[0m\n", msg)
				}
				continue
			}
			results = append(results, res)
			fmt.Printf("\033[32m  Dockerfile ready: %s\033[0m\n", res.OutputPath)
		}

		if len(results) == 0 {
			if len(failures) > 0 {
				return fmt.Errorf("all recipes failed: %s", strings.Join(failures, "; "))
			}
			return fmt.Errorf("no recipes processed")
		}
		if len(failures) > 0 {
			return fmt.Errorf("%d recipe(s) failed; aborting graph generation", len(failures))
		}

		outPath := graphOutputPath
		if outPath == "" {
			outPath = filepath.Join("local", "graphs", "layers.dot")
		}
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return fmt.Errorf("creating graph output directory: %w", err)
		}

		dot := buildGraphviz(results)
		if err := os.WriteFile(outPath, []byte(dot), 0o644); err != nil {
			return fmt.Errorf("writing Graphviz output: %w", err)
		}
		fmt.Printf("Graphviz graph written to %s\n", outPath)
		return nil
	},
}

func buildGraphviz(results []*recipeGenerationResult) string {
	var b strings.Builder
	b.WriteString("digraph BuilderLayers {\n")
	b.WriteString("  rankdir=LR;\n")
	b.WriteString("  graph [fontname=\"Helvetica\"];\n")
	b.WriteString("  node [shape=box, style=filled, fillcolor=\"#e2e8f0\", fontname=\"Helvetica\"];\n")
	b.WriteString("  edge [color=\"#6b7280\", fontname=\"Helvetica\"];\n\n")

	nodes := make(map[string]string)
	nodeAttrs := make(map[string][]string)
	edges := make(map[string]map[string]struct{})

	addEdge := func(from, to string) {
		if from == "" || to == "" {
			return
		}
		if edges[from] == nil {
			edges[from] = make(map[string]struct{})
		}
		edges[from][to] = struct{}{}
	}

	for idx, res := range results {
		startID := fmt.Sprintf("start_%d", idx)
		startLabel := fmt.Sprintf("%s:%s", res.Compiled.Build.Name, res.Compiled.Build.Version)
		nodes[startID] = startLabel
		nodeAttrs[startID] = []string{"shape=oval", "fillcolor=\"#fde68a\""}

		prev := startID
		directives := res.Compiled.Definition.Directives
		if len(directives) == 0 {
			emptyID := fmt.Sprintf("empty_%d", idx)
			nodes[emptyID] = "EMPTY"
			nodeAttrs[emptyID] = []string{"style=dashed", "fillcolor=\"#ffffff\""}
			addEdge(prev, emptyID)
			continue
		}

		for _, directive := range directives {
			hash, summary := directiveHashAndSummary(directive.Directive)
			nodeID := "layer_" + strings.ToLower(hash)
			if _, ok := nodes[nodeID]; !ok {
				label := shortenLabel(summary, 96)
				nodes[nodeID] = label
				tooltip := fmt.Sprintf("%s\\n%s", hash, summary)
				nodeAttrs[nodeID] = []string{
					"shape=box",
					"fillcolor=\"#cbd5f5\"",
					fmt.Sprintf("tooltip=%s", quoteGraphviz(tooltip)),
				}
			}
			addEdge(prev, nodeID)
			prev = nodeID
		}
	}

	var nodeIDs []string
	for id := range nodes {
		nodeIDs = append(nodeIDs, id)
	}
	sort.Strings(nodeIDs)
	for _, id := range nodeIDs {
		attrs := append([]string{fmt.Sprintf("label=%s", quoteGraphviz(nodes[id]))}, nodeAttrs[id]...)
		b.WriteString(fmt.Sprintf("  %s [%s];\n", id, strings.Join(attrs, ", ")))
	}

	var fromIDs []string
	for from := range edges {
		fromIDs = append(fromIDs, from)
	}
	sort.Strings(fromIDs)
	for _, from := range fromIDs {
		var toIDs []string
		for to := range edges[from] {
			toIDs = append(toIDs, to)
		}
		sort.Strings(toIDs)
		for _, to := range toIDs {
			b.WriteString(fmt.Sprintf("  %s -> %s;\n", from, to))
		}
	}

	b.WriteString("}\n")
	return b.String()
}

func directiveHashAndSummary(d ir.Directive) (string, string) {
	summary := formatDirectiveLabel(d)
	sum := sha256.Sum256([]byte(summary))
	hash := strings.ToUpper(hex.EncodeToString(sum[:])[:12])
	return hash, summary
}

func formatDirectiveLabel(d ir.Directive) string {
	switch v := d.(type) {
	case ir.FromImageDirective:
		return "FROM " + string(v)
	case ir.EnvironmentDirective:
		if len(v) == 0 {
			return "ENV"
		}
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s=%q", k, v[k]))
		}
		return "ENV " + strings.Join(parts, " ")
	case ir.RunDirective:
		return "RUN " + string(v)
	case ir.RunWithMountsDirective:
		parts := make([]string, 0, len(v.Mounts))
		for _, m := range v.Mounts {
			parts = append(parts, "--mount="+m)
		}
		if len(parts) > 0 {
			return "RUN " + strings.Join(parts, " ") + " " + v.Command
		}
		return "RUN " + v.Command
	case ir.CopyDirective:
		return "COPY " + strings.Join(v.Parts, " ")
	case ir.WorkDirDirective:
		return "WORKDIR " + string(v)
	case ir.UserDirective:
		return "USER " + string(v)
	case ir.EntryPointDirective:
		return "ENTRYPOINT " + string(v)
	case ir.ExecEntryPointDirective:
		if len(v) == 0 {
			return "ENTRYPOINT []"
		}
		quoted := make([]string, len(v))
		for i, arg := range v {
			quoted[i] = fmt.Sprintf("%q", arg)
		}
		return "ENTRYPOINT [" + strings.Join(quoted, ", ") + "]"
	case ir.LiteralFileDirective:
		if v.Name != "" {
			return fmt.Sprintf("RUN (literal file %s)", v.Name)
		}
		return "RUN (literal file)"
	default:
		return fmt.Sprintf("%T", d)
	}
}

func quoteGraphviz(s string) string {
	replaced := strings.ReplaceAll(s, "\\", "\\\\")
	replaced = strings.ReplaceAll(replaced, "\"", "\\\"")
	replaced = strings.ReplaceAll(replaced, "\n", "\\n")
	return "\"" + replaced + "\""
}

func shortenLabel(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}

var (
	buildMethod string
)

var buildCmd = cobra.Command{
	Use:   "build [recipe]",
	Short: "Generate Dockerfile and print buildctl command for the recipe",
	RunE: func(cmd *cobra.Command, args []string) error {
		if verbose {
			os.Setenv("BUILDER_VERBOSE", "1")
		}
		if len(args) == 0 {
			return fmt.Errorf("no recipe specified")
		}
		recipeName := args[0]

		cfg, err := loadBuilderConfig()
		if err != nil {
			return err
		}
		// Parse optional local contexts supplied as --local KEY=DIR
		var locals []string
		if lvals, _ := cmd.Flags().GetStringArray("local"); len(lvals) > 0 {
			locals = append(locals, lvals...)
		}

		switch buildMethod {
		case "docker":
			stage, err := prepareStage(cfg, recipeName, locals)
			if err != nil {
				return err
			}

			res, err := prepareDockerStage(stage)
			if err != nil {
				return err
			}

			buildDir := res.BuildDir
			dockerfile := res.Dockerfile
			dockerfilePath := res.DockerfilePath
			cacheDir := res.CacheDir

			// Parse named local contexts from RUN --mount ... from=<key>
			// Users can provide optional mappings via --local KEY=DIR flags.
			// Collect unique from= keys in Dockerfile (best-effort, informational)
			re := regexp.MustCompile(`from=([^,\s]+)`)
			want := map[string]struct{}{}
			for _, m := range re.FindAllStringSubmatch(dockerfile, -1) {
				if len(m) >= 2 {
					want[m[1]] = struct{}{}
				}
			}

			// Build with Docker BuildKit
			if _, err := exec.LookPath("docker"); err != nil {
				fmt.Printf("Dockerfile written to %s\n", dockerfilePath)
				return fmt.Errorf("docker CLI not found in PATH; please install Docker and rerun")
			}

			// Assemble docker build command
			// docker build -t name:version -f Dockerfile [--build-context key=dir ...] buildDir
			dockerArgs := []string{"build", "-t", res.Name + ":" + res.Version, "-f", dockerfilePath}
			// Provide cache= build context automatically
			dockerArgs = append(dockerArgs, "--build-context", "cache="+cacheDir)
			// Append user-provided build contexts for named mounts
			for _, kv := range locals {
				parts := strings.SplitN(kv, "=", 2)
				if len(parts) != 2 {
					fmt.Printf("WARN: ignoring invalid --local %q (want KEY=DIR)\n", kv)
					continue
				}
				dockerArgs = append(dockerArgs, "--build-context", kv)
				delete(want, parts[0])
			}
			// Any remaining keys in 'want' are optional locals; recipes typically guard with has_local.
			// We only emit an informational message to aid debugging.
			if len(want) > 0 {
				var keys []string
				for k := range want {
					keys = append(keys, k)
				}
				fmt.Printf("Info: optional locals not supplied: %s (guard with has_local)\n", strings.Join(keys, ", "))
			}
			dockerArgs = append(dockerArgs, buildDir)

			// Ensure DOCKER_BUILDKIT is enabled
			cmdRun := exec.Command("docker", dockerArgs...)
			cmdRun.Env = append(os.Environ(), "DOCKER_BUILDKIT=1")
			cmdRun.Stdout = os.Stdout
			cmdRun.Stderr = os.Stderr

			fmt.Printf("Running: DOCKER_BUILDKIT=1 docker %s\n", strings.Join(dockerArgs, " "))
			if err := cmdRun.Run(); err != nil {
				return fmt.Errorf("docker build failed: %w", err)
			}

			fmt.Printf("Built image %s:%s\n", res.Name, res.Version)
			return nil
		case "llb":
			// Build with buildctl and LLB
			if _, err := exec.LookPath("buildctl"); err != nil {
				return fmt.Errorf("buildctl not found in PATH; please install BuildKit and rerun")
			}

			stage, err := prepareStage(cfg, recipeName, locals)
			if err != nil {
				return err
			}

			llbGen, err := ir.GenerateLLBDefinition(stage.irDef)
			if err != nil {
				return fmt.Errorf("generating LLB definition: %w", err)
			}

			slog.Info("submitting build to Docker via Buildx")

			events := make(chan ir.Event)

			// Pretty console streaming of BuildKit events.
			// - Prints step start/done/cached/error using vertex names (your original names).
			// - Streams stdout/stderr from each step with a clear prefix.
			// - Avoids duplicate messages when BuildKit resends updates.
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()

				started := map[string]bool{}
				done := map[string]bool{}
				vertexNames := map[string]string{} // digest -> name
				buildStart := time.Now()
				var hadError bool

				// helper to resolve a friendly name for a vertex digest
				nameOf := func(dgst string) string {
					if n := vertexNames[dgst]; n != "" {
						return n
					}
					// Short fallback if we have no name yet
					if len(dgst) > 19 { // "sha256:" + 12 chars
						return dgst[:19]
					}
					return dgst
				}

				for ev := range events {
					switch ev.Type {
					case ir.EventTypeStatus:
						s := ev.Status
						if s == nil {
							continue
						}

						// Merge provided vertex names into our local map.
						for id, n := range ev.VertexNames {
							if n != "" {
								vertexNames[id] = n
							}
						}

						// Vertex lifecycle updates (start/done/cached/error).
						for _, v := range s.Vertexes {
							id := v.Digest.String()
							if v.Name != "" {
								vertexNames[id] = v.Name
							}
							n := nameOf(id)

							// Start (only once)
							if !started[id] && v.Started != nil && !v.Started.IsZero() {
								started[id] = true
								slog.Info("step started", "name", n)
							}

							// Error
							if v.Error != "" && !done[id] {
								hadError = true
								done[id] = true
								var dur time.Duration
								if !v.Started.IsZero() && v.Started != nil && !v.Completed.IsZero() {
									dur = v.Completed.Sub(*v.Started)
								}
								slog.Error("step failed", "name", n, "duration", dur, "error", v.Error)
								continue
							}

							// Cached
							if v.Cached && !done[id] {
								done[id] = true
								slog.Info("step cached", "name", n)
								continue
							}

							// Completed
							if v.Completed != nil && !v.Completed.IsZero() && !done[id] {
								done[id] = true
								dur := v.Completed.Sub(*v.Started)
								slog.Info("step completed", "name", n, "duration", dur)
							}
						}

						// Stream logs with step-aware prefixes.
						for _, l := range s.Logs {
							id := l.Vertex.String()
							n := nameOf(id)
							stream := "stdout"
							if l.Stream == 2 {
								stream = "stderr"
							}
							// Print line by line to keep output tidy.
							b := l.Data
							for len(b) > 0 {
								i := bytes.IndexByte(b, '\n')
								if i < 0 {
									i = len(b)
								}
								line := bytes.TrimRight(b[:i], "\r")
								if len(line) > 0 {
									fmt.Printf("[%s] %s: %s\n", n, stream, string(line))
								}
								if i == len(b) {
									break
								}
								b = b[i+1:]
							}
						}

					case ir.EventTypeError:
						hadError = true
						if ev.Error != "" {
							slog.Error("build failed", "error", ev.Error)
						} else {
							slog.Error("build failed")
						}

					case ir.EventTypeResult:
						total := time.Since(buildStart)
						if hadError {
							slog.Error("build finished with errors", "duration", total)
						} else {
							slog.Info("build finished successfully", "duration", total)
						}
					}
				}
			}()

			if err := ir.SubmitToDockerViaBuildx(context.Background(), llbGen, "", "", events); err != nil {
				wg.Wait()
				return fmt.Errorf("submitting to Docker via Buildx: %w", err)
			}
			wg.Wait()

			return nil
		default:
			return fmt.Errorf("unsupported build method %q", buildMethod)
		}
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&rootBuilderConfig, "config", "builder.config.yaml", "Path to builder configuration file")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")

	rootCmd.AddCommand(&generateDockerfileCmd)

	// test-all flags
	rootCmd.AddCommand(&testAllCmd)

	graphCmd.Flags().StringVar(&graphOutputPath, "output", filepath.Join("local", "graphs", "layers.dot"), "Path to Graphviz DOT output")
	rootCmd.AddCommand(&graphCmd)

	// test command
	testCmd.Flags().BoolVar(&testCaptureOutput, "capture-output", false, "Capture output from commands")
	rootCmd.AddCommand(&testCmd)

	// Build command flags: --local KEY=DIR can be repeated to supply named contexts
	buildCmd.Flags().StringArray("local", []string{}, "Supply a named local context as KEY=DIR for RUN --mount from=KEY")
	buildCmd.Flags().StringVar(&buildMethod, "method", "docker", "Build method to use (docker,llb)")
	rootCmd.AddCommand(&buildCmd)

	// Stage command (no build), supports --local as well
	stageCmd.Flags().StringArray("local", []string{}, "Supply a named local context as KEY=DIR for RUN --mount from=KEY")
	rootCmd.AddCommand(&stageCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

// splitQuoted splits a string of quoted args into fields (simple parser for our COPY lines).
func splitQuoted(s string) []string {
	var out []string
	var cur strings.Builder
	inQ := false
	esc := false
	for _, r := range s {
		switch {
		case esc:
			cur.WriteRune(r)
			esc = false
		case r == '\\' && inQ:
			esc = true
		case r == '"':
			inQ = !inQ
		case r == ' ' && !inQ:
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}
