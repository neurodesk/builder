package main

import (
    "fmt"
    "io"
    "log/slog"
    "os"
    "path/filepath"
    "regexp"
    "strings"
    "os/exec"
    "net/http"

	"github.com/neurodesk/builder/pkg/ir"
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

var rootCmd = cobra.Command{
	Use:   "builder",
	Short: "A tool to build container images from recipes",
}

var generateDockerfileCmd = cobra.Command{
	Use:   "generate [recipe]",
	Short: "Generate a Dockerfile for the specified recipe",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("no recipe specified")
		}
		recipeName := args[0]

		var cfg builderConfig

		if err := cfg.loadConfig(rootBuilderConfig); err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		// Set the template directory if specified in config
		if cfg.TemplateDir != "" {
			templates.SetTemplateDir(cfg.TemplateDir)
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

func testRecipes(recipes []string) error {
	var cfg builderConfig

	if err := cfg.loadConfig(rootBuilderConfig); err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Set the template directory if specified in config
	if cfg.TemplateDir != "" {
		templates.SetTemplateDir(cfg.TemplateDir)
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

		out, err := build.Generate(cfg.IncludeDirs)
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
		var cfg builderConfig

		if err := cfg.loadConfig(rootBuilderConfig); err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		// Set the template directory if specified in config
		if cfg.TemplateDir != "" {
			templates.SetTemplateDir(cfg.TemplateDir)
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
        if len(args) == 0 {
            return fmt.Errorf("no recipe specified")
        }
        recipeName := args[0]

        var cfg builderConfig
        if err := cfg.loadConfig(rootBuilderConfig); err != nil {
            return fmt.Errorf("loading config: %w", err)
        }
        if cfg.TemplateDir != "" {
            templates.SetTemplateDir(cfg.TemplateDir)
        }

        // Resolve recipe path
        var recipePath string
        if strings.ContainsRune(recipeName, os.PathSeparator) || strings.HasPrefix(recipeName, ".") || strings.HasPrefix(recipeName, "/") {
            recipePath = recipeName
        } else {
            for _, root := range cfg.RecipeRoots {
                cand := filepath.Join(root, recipeName)
                if st, err := os.Stat(cand); err == nil && st.IsDir() {
                    recipePath = cand
                    break
                }
            }
            if recipePath == "" {
                return fmt.Errorf("recipe not found in roots: %s", recipeName)
            }
        }

        build, err := recipe.LoadBuildFile(recipePath)
        if err != nil {
            return fmt.Errorf("loading build file: %w", err)
        }

        irDef, plan, err := build.GenerateWithStaging(cfg.IncludeDirs)
        if err != nil {
            return fmt.Errorf("generating build IR: %w", err)
        }
        dockerfile, err := ir.GenerateDockerfile(irDef)
        if err != nil {
            return fmt.Errorf("generating dockerfile: %w", err)
        }

        // Write Dockerfile to a dedicated build directory
        buildDir := filepath.Join("local", "build", build.Name)
        if err := os.MkdirAll(buildDir, 0o755); err != nil {
            return fmt.Errorf("creating build directory: %w", err)
        }
        dockerfilePath := filepath.Join(buildDir, "Dockerfile")
        if err := os.WriteFile(dockerfilePath, []byte(dockerfile), 0o644); err != nil {
            return fmt.Errorf("writing Dockerfile: %w", err)
        }

        // Stage cache files for get_file() into a local build context
        cacheDir := filepath.Join(buildDir, "cache")
        if err := os.MkdirAll(cacheDir, 0o755); err != nil {
            return fmt.Errorf("creating cache dir: %w", err)
        }
        // Helper to write a file with optional exec bit
        writeFile := func(dst string, data []byte, exec bool) error {
            if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil { return err }
            mode := os.FileMode(0o644)
            if exec { mode = 0o755 }
            return os.WriteFile(dst, data, mode)
        }
        for _, f := range plan.Files {
            dst := filepath.Join(cacheDir, filepath.FromSlash(f.Name))
            switch {
            case f.HostFilename != "":
                b, err := os.ReadFile(f.HostFilename)
                if err != nil { return fmt.Errorf("staging file %q: %w", f.HostFilename, err) }
                if err := writeFile(dst, b, f.Executable); err != nil { return err }
            case f.Contents != "":
                if err := writeFile(dst, []byte(f.Contents), f.Executable); err != nil { return err }
            case f.URL != "":
                // Best-effort: try to fetch; if network blocked, instruct user.
                // Network sandbox may restrict this; we still attempt.
                resp, err := httpGet(f.URL)
                if err != nil {
                    fmt.Printf("WARN: could not download %s: %v\n", f.URL, err)
                    continue
                }
                if err := writeFile(dst, resp, f.Executable); err != nil { return err }
            }
        }

        // Materialize Copy sources that refer to staged files into buildDir
        // by copying from cache to the requested relative path if names match
        // (simple heuristic: if src equals a staged file name)
        type copyDirective struct{ Src []string; Dest string }
        var copies []copyDirective
        // Very lightweight parse of COPY lines from Dockerfile
        // Quoted paths were emitted, strip quotes.
        for _, line := range strings.Split(dockerfile, "\n") {
            line = strings.TrimSpace(line)
            if !strings.HasPrefix(line, "COPY ") { continue }
            body := strings.TrimPrefix(line, "COPY ")
            fields := splitQuoted(body)
            if len(fields) < 2 { continue }
            dest := fields[len(fields)-1]
            srcs := fields[:len(fields)-1]
            copies = append(copies, copyDirective{Src: srcs, Dest: dest})
        }
        if len(plan.Files) > 0 {
            // index staged names
            staged := map[string]struct{}{}
            for _, f := range plan.Files { staged[f.Name] = struct{}{} }
            for _, c := range copies {
                for _, s := range c.Src {
                    s = strings.Trim(s, "\"")
                    if _, ok := staged[s]; ok {
                        // copy from cache/<s> to buildDir/<s>
                        src := filepath.Join(cacheDir, filepath.FromSlash(s))
                        dst := filepath.Join(buildDir, filepath.FromSlash(s))
                        b, err := os.ReadFile(src)
                        if err == nil {
                            _ = os.MkdirAll(filepath.Dir(dst), 0o755)
                            _ = os.WriteFile(dst, b, 0o644)
                        }
                    }
                }
            }
        }

        // Parse required named local contexts from RUN --mount ... from=<key>
        // Users can provide mappings via --local KEY=DIR flags.
        var locals []string
        if lvals, _ := cmd.Flags().GetStringArray("local"); len(lvals) > 0 {
            locals = append(locals, lvals...)
        }
        // Collect unique from= keys in Dockerfile
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
        dockerArgs := []string{"build", "-t", build.Name + ":" + build.Version, "-f", dockerfilePath}
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
        for k := range want {
            fmt.Printf("WARN: missing --build-context mapping for key %q required by RUN mounts\n", k)
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

        fmt.Printf("Built image %s:%s\n", build.Name, build.Version)
        return nil
    },
}

func init() {
    rootCmd.PersistentFlags().StringVar(&rootBuilderConfig, "config", "builder.config.yaml", "Path to builder configuration file")

    rootCmd.AddCommand(&generateDockerfileCmd)

    rootCmd.AddCommand(&testAllCmd)

    // Build command flags: --local KEY=DIR can be repeated to supply named contexts
    buildCmd.Flags().StringArray("local", []string{}, "Supply a named local context as KEY=DIR for RUN --mount from=KEY")
    rootCmd.AddCommand(&buildCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

// httpGet downloads a URL and returns the body bytes.
func httpGet(url string) ([]byte, error) {
    resp, err := http.Get(url)
    if err != nil { return nil, err }
    defer resp.Body.Close()
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
    }
    return io.ReadAll(resp.Body)
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
            if cur.Len() > 0 { out = append(out, cur.String()); cur.Reset() }
        default:
            cur.WriteRune(r)
        }
    }
    if cur.Len() > 0 { out = append(out, cur.String()) }
    return out
}
