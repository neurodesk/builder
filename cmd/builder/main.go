package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

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
var verbose bool

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

            // Handle virtual cache/<name> paths: ensure staged cache file exists; no copy needed
            if strings.HasPrefix(srcNorm, "cache/") {
                name := strings.TrimPrefix(srcNorm, "cache/")
                cacheSrc := filepath.Join(buildDir, "cache", filepath.FromSlash(name))
                if _, err := os.Stat(cacheSrc); err != nil {
                    return fmt.Errorf("COPY source %q refers to missing staged cache file %q", srcRel, cacheSrc)
                }
                // No additional copy needed; Docker build will read from buildDir/cache/...
                continue
            }

            // Handle bare virtual names (no slash) that refer to declared files: copy from cache into build context
            if !strings.Contains(srcNorm, "/") {
                if _, ok := vset[srcNorm]; ok {
                    cacheSrc := filepath.Join(buildDir, "cache", filepath.FromSlash(srcNorm))
                    if st, err := os.Stat(cacheSrc); err != nil || st.IsDir() {
                        return fmt.Errorf("virtual COPY source %q not found in staged cache at %q", srcRel, cacheSrc)
                    }
                    if verbose {
                        fmt.Printf("[verbose] Materializing virtual file %s -> %s\n", cacheSrc, bcPath)
                    }
                    if err := copyFile(cacheSrc, bcPath, false); err != nil {
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

type stageResult struct {
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

// helper: generate, render, write dockerfile, and stage files/COPYs
func prepareStage(cfg builderConfig, recipeSpec string, locals []string) (*stageResult, error) {
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
	dockerfile, err := ir.GenerateDockerfile(irDef)
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
	if err := stageIntoBuildContext(cfg, recipePath, dockerfile, buildDir, plan); err != nil {
		return nil, err
	}
	res := &stageResult{
		Name:           build.Name,
		Version:        build.Version,
		Tag:            build.Name + ":" + build.Version,
		Arch:           string(build.Architectures[0]),
		BuildDir:       buildDir,
		DockerfilePath: dockerfilePath,
		CacheDir:       filepath.Join(buildDir, "cache"),
		LocalContext:   locals,
		Dockerfile:     dockerfile,
	}
	return res, nil
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
		build, err := recipe.LoadBuildFile(r)
		if err != nil {
			failed++
			fmt.Printf("\033[31m  Failed to load build file: %v\033[0m\n", err)
			continue
		}

		// Generate IR and staging plan so we can validate file presence
		out, plan, err := build.GenerateWithStaging(cfg.IncludeDirs)
		if err != nil {
			failed++
			fmt.Printf("\033[31m  Failed to generate IR: %v\033[0m\n", err)
			continue
		}

		dockerfile, err := ir.GenerateDockerfile(out)
		if err != nil {
			failed++
			fmt.Printf("\033[31m  Failed to generate dockerfile: %v\033[0m\n", err)
			continue
		}

		// write it to local/docker/<name>_<version>.Dockerfile
		outputPath := filepath.Join(outputDir, fmt.Sprintf("%s_%s.Dockerfile", build.Name, build.Version))
		if err := os.WriteFile(outputPath, []byte(dockerfile), 0o644); err != nil {
			return fmt.Errorf("writing dockerfile: %w", err)
		}

		// Optionally validate the Dockerfile using the official BuildKit parser
		if _, err := parser.Parse(strings.NewReader(dockerfile)); err != nil {
			failed++
			fmt.Printf("\033[31m  BuildKit parser validation failed: %v\033[0m\n", err)
			continue
		}

		// Validate staged file sources exist in expected locations
		missing := false
		for _, f := range plan.Files {
			if f.HostFilename == "" { // URLs and literals are not checked here
				continue
			}
			src := f.HostFilename
			// Resolve relative to recipe dir first, then include dirs
			var candidates []string
			if filepath.IsAbs(src) {
				candidates = []string{src}
			} else {
				candidates = append(candidates, filepath.Join(r, src))
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
				fmt.Printf("\033[31m  Missing file referenced by files: %s (searched: %s)\033[0m\n", src, strings.Join(candidates, ", "))
			}
		}

		// Validate COPY sources are present, supporting "virtual" files declared via files{}.
		// Virtual files are those declared in the recipe's files list; they are staged
		// into the special cache context and may be referenced by name, by cache/<name>,
		// or via the get_file() path "/.neurocontainer-cache/<name>".
		// We do NOT attempt to download HTTP files here; we only validate names.
		// Build a quick lookup of declared virtual file names.
		vset := map[string]struct{}{}
		for _, f := range plan.Files {
			if f.Name != "" {
				vset[f.Name] = struct{}{}
			}
		}

		for _, spec := range parseCopySpecs(dockerfile) {
			for _, srcRel := range spec.Src {
				// Normalize to forward slashes for checks
				srcNorm := strings.TrimPrefix(strings.ReplaceAll(srcRel, "\\", "/"), "./")

				// Handle absolute path that matches get_file() convention
				if filepath.IsAbs(srcRel) {
					if after, ok := strings.CutPrefix(srcNorm, "/.neurocontainer-cache/"); ok {
						if _, ok := vset[after]; !ok {
							missing = true
							fmt.Printf("\033[31m  COPY references virtual file not declared: %s (name %s)\033[0m\n", srcRel, after)
						}
						continue
					}
					missing = true
					fmt.Printf("\033[31m  Invalid absolute COPY source: %s\033[0m\n", srcRel)
					continue
				}

				// cache/<name> maps to a declared virtual file
				if after, ok := strings.CutPrefix(srcNorm, "cache/"); ok {
					if _, ok := vset[after]; !ok {
						missing = true
						fmt.Printf("\033[31m  COPY references unknown cache file: %s (name %s)\033[0m\n", srcRel, after)
					}
					continue
				}

				// Bare name may refer to a declared virtual file
				if !strings.Contains(srcNorm, "/") {
					if _, ok := vset[srcNorm]; ok {
						// It's a declared virtual; accept without filesystem check
						continue
					}
				}

				// Otherwise, expect a real file in the recipe directory
				srcPath := filepath.Join(r, filepath.FromSlash(srcRel))
				// Resolve symlinks and ensure it remains within recipe dir
				eval, err := filepath.EvalSymlinks(srcPath)
				if err != nil {
					missing = true
					fmt.Printf("\033[31m  COPY source not found: %s (from %s)\033[0m\n", srcRel, srcPath)
					continue
				}
				// Ensure path does not escape the recipe directory
				baseAbs, _ := filepath.Abs(r)
				evalAbs, _ := filepath.Abs(eval)
				if rel, err := filepath.Rel(baseAbs, evalAbs); err != nil || strings.HasPrefix(rel, "..") {
					missing = true
					fmt.Printf("\033[31m  COPY source escapes recipe directory: %s -> %s\033[0m\n", srcRel, eval)
					continue
				}
				if _, err := os.Stat(eval); err != nil {
					missing = true
					fmt.Printf("\033[31m  COPY source missing: %s (resolved %s)\033[0m\n", srcRel, eval)
					continue
				}
			}
		}

		if missing {
			failed++
			fmt.Printf("\033[31m  One or more required files are missing\033[0m\n")
			continue
		}

		fmt.Printf("\033[32m  Successfully generated Dockerfile: %s\033[0m\n", outputPath)
		success++
	}

	fmt.Printf("Tested %d recipes: %d succeeded, %d failed\n", len(recipes), success, failed)
	if failed > 0 {
		return fmt.Errorf("%d recipes failed", failed)
	}
	return nil
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

		var recipes []string
		for _, root := range cfg.RecipeRoots {
			entries, err := os.ReadDir(root)
			if err != nil {
				return fmt.Errorf("reading recipe root %s: %w", root, err)
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

		return testRecipes(recipes)
	},
}

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
		res, err := prepareStage(cfg, recipeName, locals)
		if err != nil {
			return err
		}
		buildDir := res.BuildDir
		dockerfile := res.Dockerfile
		dockerfilePath := res.DockerfilePath
		cacheDir := res.CacheDir
		// staging already done in prepareStage
		// Staging is already done by prepareStage()

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
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&rootBuilderConfig, "config", "builder.config.yaml", "Path to builder configuration file")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")

	rootCmd.AddCommand(&generateDockerfileCmd)

	// test-all flags
	rootCmd.AddCommand(&testAllCmd)

	// Build command flags: --local KEY=DIR can be repeated to supply named contexts
	buildCmd.Flags().StringArray("local", []string{}, "Supply a named local context as KEY=DIR for RUN --mount from=KEY")
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
