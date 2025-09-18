package main

import (
    "fmt"
    "log/slog"
    "os"
    "path/filepath"
    "regexp"
    "strings"
    "os/exec"

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

		out, err := build.Generate(cfg.IncludeDirs)
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

        irDef, err := build.Generate(cfg.IncludeDirs)
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
