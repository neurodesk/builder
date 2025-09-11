package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/neurodesk/builder/pkg/recipe"
	"go.yaml.in/yaml/v4"
)

func discoverRecipes(dir string) ([]string, error) {
	recipesDir := filepath.Join(dir, "recipes")

	var recipes []string
	ents, err := os.ReadDir(recipesDir)
	if err != nil {
		return nil, err
	}
	for _, ent := range ents {
		if !ent.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(recipesDir, ent.Name(), "build.yaml")); err == nil {
			recipes = append(recipes, filepath.Join(recipesDir, ent.Name()))
		}
	}
	return recipes, nil
}

func appMain() error {
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	neuroContainersDir := fs.String("neurocontainers-dir", "", "Path to neurocontainers directory")

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	if *neuroContainersDir == "" {
		return fmt.Errorf("neurocontainers-dir is required")
	}

	recipes, err := discoverRecipes(*neuroContainersDir)
	if err != nil {
		return err
	}

	for _, r := range recipes {
		buildYaml := filepath.Join(r, "build.yaml")

		f, err := os.Open(buildYaml)
		if err != nil {
			return err
		}

		dec := yaml.NewDecoder(f)
		dec.KnownFields(true)

		var build recipe.BuildFile
		if err := dec.Decode(&build); err != nil {
			err, ok := err.(*yaml.TypeError)
			if ok {
				fmt.Fprintf(os.Stderr, "Recipe errors: %s\n", r)
				for _, e := range err.Errors {
					fmt.Fprintf(os.Stderr, "%s\n", e)
				}
				continue
			}
			return fmt.Errorf("failed to parse %s: %w", buildYaml, err)
		}

		ctx := recipe.Context{
			Version:        build.Version,
			PackageManager: build.Build.PackageManager,
		}

		if err := build.Validate(ctx); err != nil {
			slog.Error("validation error", "file", " "+buildYaml, "error", err)
			continue
		}

		slog.Info("validated", "file", buildYaml)
	}

	return nil
}

func main() {
	if err := appMain(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}
