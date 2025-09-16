package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/neurodesk/builder/pkg/ir"
	"github.com/neurodesk/builder/pkg/recipe"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v4"
)

type builderConfig struct {
	RecipeRoots []string `yaml:"recipe_roots"`
	IncludeDirs []string `yaml:"include_dirs"`
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

func init() {
	rootCmd.PersistentFlags().StringVar(&rootBuilderConfig, "config", "builder.config.yaml", "Path to builder configuration file")

	rootCmd.AddCommand(&generateDockerfileCmd)

	rootCmd.AddCommand(&testAllCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}
