package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/neurodesk/builder/pkg/common"
	"github.com/neurodesk/builder/pkg/ir"
	"github.com/neurodesk/builder/pkg/recipe"
	"github.com/neurodesk/builder/pkg/templates"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v4"
)

var templateTestsCmd = cobra.Command{
	Use:   "template-tests [selector ...]",
	Short: "Generate builder recipes from template test definitions",
	RunE: func(cmd *cobra.Command, args []string) error {
		if verbose {
			os.Setenv("BUILDER_VERBOSE", "1")
		}

		cfg, err := loadBuilderConfig()
		if err != nil {
			return err
		}

		specs, err := loadTemplateTestSpecs(cfg.TemplateDir)
		if err != nil {
			return err
		}
		if len(specs) == 0 {
			return fmt.Errorf("no template tests defined in test_all.yaml")
		}

		selected, err := filterTemplateSpecs(specs, args)
		if err != nil {
			return err
		}
		if len(selected) == 0 {
			return fmt.Errorf("no template tests matched the provided selectors")
		}

		printDockerfile, _ := cmd.Flags().GetBool("print-dockerfile")
		doBuild, _ := cmd.Flags().GetBool("build")
		testNames, _ := cmd.Flags().GetStringSlice("tests")
		runAllTests, _ := cmd.Flags().GetBool("run-tests")

		shouldRunTests := runAllTests || len(testNames) > 0
		if !printDockerfile && !doBuild && !shouldRunTests {
			return fmt.Errorf("no action requested: specify at least one of --print-dockerfile, --build, or --run-tests/--tests")
		}

		if doBuild || shouldRunTests {
			if _, err := exec.LookPath("docker"); err != nil {
				return fmt.Errorf("docker CLI not found in PATH; please install Docker and rerun")
			}
		}

		// Normalise requested test names once for lookup
		requestedTests := normaliseTestFilters(testNames)
		var missingFilters []string

		for _, spec := range selected {
			fmt.Printf("Processing template test %s\n", spec.Identifier())

			buildFile, err := spec.ToBuildFile()
			if err != nil {
				return fmt.Errorf("%s: %w", spec.Identifier(), err)
			}

			stage, err := stageBuildFileForTemplate(cfg, buildFile)
			if err != nil {
				return fmt.Errorf("%s: %w", spec.Identifier(), err)
			}

			if printDockerfile {
				fmt.Printf("Dockerfile for %s written to %s\n", spec.Identifier(), stage.DockerfilePath)
				fmt.Println(stage.Dockerfile)
			}

			if doBuild {
				if err := runDockerBuild(stage); err != nil {
					return fmt.Errorf("%s: %w", spec.Identifier(), err)
				}
			}

			if shouldRunTests {
				tests, missing := spec.SelectTests(requestedTests, runAllTests)
				missingFilters = append(missingFilters, missing...)
				if len(tests) == 0 {
					fmt.Printf("No tests selected for %s\n", spec.Identifier())
					continue
				}

				if !doBuild {
					exist, err := imageExists(stage.Tag)
					if err != nil {
						return fmt.Errorf("%s: %w", spec.Identifier(), err)
					}
					if !exist {
						return fmt.Errorf("image %s not found; run with --build first", stage.Tag)
					}
				}

				if err := runTemplateTests(stage, tests); err != nil {
					return fmt.Errorf("%s: %w", spec.Identifier(), err)
				}
			}
		}

		if len(missingFilters) > 0 {
			return fmt.Errorf("unknown tests requested: %s", strings.Join(uniqueStrings(missingFilters), ", "))
		}

		return nil
	},
}

type templateTestSpec struct {
	Name           string             `yaml:"name"`
	Template       string             `yaml:"template"`
	Arguments      map[string]any     `yaml:"arguments"`
	BaseImage      string             `yaml:"base_image"`
	PackageManager string             `yaml:"package_manager"`
	ImageVersion   string             `yaml:"image_version"`
	Tests          []templateTestCase `yaml:"tests"`

	resolvedName string `yaml:"-"`
}

type templateTestCase struct {
	Name    string `yaml:"name"`
	Command string `yaml:"command"`
}

var invalidNameChars = regexp.MustCompile(`[^a-z0-9-]+`)

func (t *templateTestCase) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		cmd := strings.TrimSpace(value.Value)
		if cmd == "" {
			return fmt.Errorf("test command must not be empty")
		}
		t.Command = cmd
		t.Name = deriveTestName(cmd)
		return nil
	case yaml.MappingNode:
		type alias templateTestCase
		var tmp alias
		if err := value.Decode(&tmp); err != nil {
			return err
		}
		if strings.TrimSpace(tmp.Command) == "" {
			return fmt.Errorf("test command must not be empty")
		}
		tmp.Command = strings.TrimSpace(tmp.Command)
		tmp.Name = strings.TrimSpace(tmp.Name)
		if tmp.Name == "" {
			tmp.Name = deriveTestName(tmp.Command)
		}
		*t = templateTestCase(tmp)
		return nil
	default:
		return fmt.Errorf("unsupported test entry type: %v", value.Kind)
	}
}

func deriveTestName(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return "test"
	}
	if idx := strings.IndexAny(cmd, " \t"); idx != -1 {
		cmd = cmd[:idx]
	}
	name := strings.ToLower(cmd)
	name = invalidNameChars.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if name == "" {
		name = "test"
	}
	return name
}

func (s templateTestSpec) Identifier() string {
	if s.Name != "" {
		return s.Name
	}
	return s.Template
}

func (s *templateTestSpec) ensureResolvedName(counter map[string]int) {
	if s.resolvedName != "" {
		return
	}
	base := s.Name
	if base == "" {
		base = fmt.Sprintf("template-%s", s.Template)
	}
	base = strings.ToLower(base)
	base = invalidNameChars.ReplaceAllString(base, "-")
	base = strings.Trim(base, "-")
	if base == "" {
		base = "template"
	}
	count := counter[base]
	if count > 0 {
		s.resolvedName = fmt.Sprintf("%s-%d", base, count+1)
	} else {
		s.resolvedName = base
	}
	counter[base] = count + 1
}

func (s templateTestSpec) ToBuildFile() (*recipe.BuildFile, error) {
	if s.Template == "" {
		return nil, fmt.Errorf("template name is required")
	}

	mgr, err := parsePackageManager(s.PackageManager)
	if err != nil {
		return nil, err
	}

	baseImage := s.BaseImage
	if baseImage == "" {
		switch mgr {
		case common.PkgManagerApt:
			baseImage = "ubuntu:22.04"
		case common.PkgManagerYum:
			baseImage = "rockylinux:9"
		default:
			baseImage = "ubuntu:22.04"
		}
	}

	version := strings.TrimSpace(s.ImageVersion)
	if version == "" {
		version = "0.0.0"
	}

	params := map[string]any{}
	for k, v := range s.Arguments {
		params[k] = v
	}

	build := &recipe.BuildFile{
		Name:          s.resolvedName,
		Version:       version,
		Architectures: []recipe.CPUArchitecture{recipe.CPUArchAMD64},
		Build: recipe.BuildRecipe{
			Kind:           recipe.BuildKindNeuroDocker,
			BaseImage:      baseImage,
			PackageManager: mgr,
			Directives: []recipe.Directive{
				{Template: &recipe.TemplateDirective{ //nolint:exhaustruct
					Name:   s.Template,
					Params: params,
				}},
			},
		},
	}

	if err := build.Validate(recipe.Context{PackageManager: mgr}); err != nil {
		return nil, fmt.Errorf("generated build file invalid: %w", err)
	}

	return build, nil
}

func parsePackageManager(input string) (common.PackageManager, error) {
	v := strings.ToLower(strings.TrimSpace(input))
	switch v {
	case "", "apt":
		return common.PkgManagerApt, nil
	case "yum":
		return common.PkgManagerYum, nil
	default:
		var zero common.PackageManager
		return zero, fmt.Errorf("unsupported package manager %q", input)
	}
}

func loadTemplateTestSpecs(templateDir string) ([]templateTestSpec, error) {
	var data []byte
	if templateDir != "" {
		path := filepath.Join(templateDir, "test_all.yaml")
		b, err := os.ReadFile(path)
		if err == nil {
			data = b
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
	}

	if len(data) == 0 {
		embedded, err := templates.Files.ReadFile("test_all.yaml")
		if err != nil {
			return nil, fmt.Errorf("reading embedded test definitions: %w", err)
		}
		data = embedded
	}

	var specs []templateTestSpec
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&specs); err != nil {
		return nil, fmt.Errorf("decoding test definitions: %w", err)
	}

	counter := map[string]int{}
	for i := range specs {
		if specs[i].Arguments == nil {
			specs[i].Arguments = map[string]any{}
		}
		specs[i].ensureResolvedName(counter)
	}

	return specs, nil
}

func filterTemplateSpecs(specs []templateTestSpec, selectors []string) ([]templateTestSpec, error) {
	if len(selectors) == 0 {
		return specs, nil
	}
	sel := make([]string, 0, len(selectors))
	for _, s := range selectors {
		s = strings.TrimSpace(s)
		if s != "" {
			sel = append(sel, s)
		}
	}
	if len(sel) == 0 {
		return specs, nil
	}

	set := map[string]struct{}{}
	for _, s := range sel {
		set[strings.ToLower(s)] = struct{}{}
	}

	var filtered []templateTestSpec
	for _, spec := range specs {
		id := strings.ToLower(spec.Identifier())
		templateName := strings.ToLower(spec.Template)
		resolved := strings.ToLower(spec.resolvedName)
		if _, ok := set[id]; ok {
			filtered = append(filtered, spec)
			continue
		}
		if _, ok := set[templateName]; ok {
			filtered = append(filtered, spec)
			continue
		}
		if _, ok := set[resolved]; ok {
			filtered = append(filtered, spec)
		}
	}

	return filtered, nil
}

func normaliseTestFilters(filters []string) map[string]struct{} {
	if len(filters) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(filters))
	for _, f := range filters {
		for _, part := range strings.Split(f, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			out[strings.ToLower(part)] = struct{}{}
		}
	}
	return out
}

func (s templateTestSpec) SelectTests(filters map[string]struct{}, runAll bool) ([]templateTestCase, []string) {
	if !runAll && len(filters) == 0 {
		return nil, nil
	}

	if runAll && len(filters) == 0 {
		return append([]templateTestCase(nil), s.Tests...), nil
	}

	if len(s.Tests) == 0 {
		return nil, nil
	}

	selected := []templateTestCase{}
	remaining := map[string]struct{}{}
	for k := range filters {
		remaining[k] = struct{}{}
	}

	for _, test := range s.Tests {
		key := strings.ToLower(test.Name)
		full := strings.ToLower(fmt.Sprintf("%s/%s", s.Identifier(), test.Name))
		if _, ok := filters[key]; ok {
			selected = append(selected, test)
			delete(remaining, key)
			delete(remaining, full)
			continue
		}
		if _, ok := filters[full]; ok {
			selected = append(selected, test)
			delete(remaining, key)
			delete(remaining, full)
		}
	}

	missing := make([]string, 0, len(remaining))
	for k := range remaining {
		missing = append(missing, k)
	}

	return selected, missing
}

func stageBuildFileForTemplate(cfg builderConfig, build *recipe.BuildFile) (*dockerStageResult, error) {
	irDef, plan, err := build.GenerateWithStaging(cfg.IncludeDirs)
	if err != nil {
		return nil, fmt.Errorf("generating build IR: %w", err)
	}

	dockerfile, err := ir.GenerateDockerfile(irDef)
	if err != nil {
		return nil, fmt.Errorf("generating dockerfile: %w", err)
	}

	if strings.Contains(dockerfile, "\" + ") {
		return nil, fmt.Errorf("detected unrendered string concatenation in generated Dockerfile; fix template %q", build.Name)
	}

	buildDir := filepath.Join("local", "template-tests", build.Name)
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating build directory: %w", err)
	}

	dockerfilePath := filepath.Join(buildDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(dockerfile), 0o644); err != nil {
		return nil, fmt.Errorf("writing Dockerfile: %w", err)
	}

	if plan == nil {
		plan = &recipe.StagingPlan{}
	}

	if err := stageIntoBuildContext(cfg, "", dockerfile, buildDir, plan); err != nil {
		return nil, err
	}

	res := &dockerStageResult{ //nolint:exhaustruct
		Name:           build.Name,
		Version:        build.Version,
		Tag:            build.Name + ":" + build.Version,
		Arch:           string(build.Architectures[0]),
		BuildDir:       buildDir,
		DockerfilePath: dockerfilePath,
		CacheDir:       filepath.Join(buildDir, "cache"),
		Dockerfile:     dockerfile,
	}

	return res, nil
}

func ensureTemplateLogDir(stage *dockerStageResult) (string, error) {
	logDir := filepath.Join("local", "template_tests", stage.Name)
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return "", fmt.Errorf("creating log directory: %w", err)
	}
	return logDir, nil
}

func runDockerBuild(stage *dockerStageResult) error {
	logDir, err := ensureTemplateLogDir(stage)
	if err != nil {
		return err
	}
	dockerArgs := []string{
		"build",
		"-t", stage.Tag,
		"-f", stage.DockerfilePath,
		"--build-context", "cache=" + stage.CacheDir,
		stage.BuildDir,
	}

	fmt.Printf("Running: DOCKER_BUILDKIT=1 docker %s\n", strings.Join(dockerArgs, " "))

	cmd := exec.Command("docker", dockerArgs...)
	cmd.Env = append(os.Environ(), "DOCKER_BUILDKIT=1")
	var buf bytes.Buffer
	multi := io.MultiWriter(os.Stdout, &buf)
	cmd.Stdout = multi
	cmd.Stderr = multi

	runErr := cmd.Run()
	exitCode := 0
	if exitErr, ok := runErr.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	} else if runErr != nil {
		exitCode = 1
	}

	logPath := filepath.Join(logDir, "build.log")
	logHeader := fmt.Sprintf("Command: DOCKER_BUILDKIT=1 docker %s\nExitCode: %d\n\n", strings.Join(dockerArgs, " "), exitCode)
	if writeErr := os.WriteFile(logPath, append([]byte(logHeader), buf.Bytes()...), 0o644); writeErr != nil {
		return fmt.Errorf("writing build log: %w", writeErr)
	}
	fmt.Printf("Build log written to %s\n", logPath)

	if runErr != nil {
		return fmt.Errorf("docker build failed: %w", runErr)
	}

	fmt.Printf("Built image %s\n", stage.Tag)
	return nil
}

func runTemplateTests(stage *dockerStageResult, tests []templateTestCase) error {
	logDir, err := ensureTemplateLogDir(stage)
	if err != nil {
		return err
	}

	tag := stage.Tag
	for _, t := range tests {
		fmt.Printf("Running test %s for %s\n", t.Name, tag)
		args := []string{"run", "--rm", tag, "bash", "-lc", t.Command}
		cmd := exec.Command("docker", args...)
		var buf bytes.Buffer
		multi := io.MultiWriter(os.Stdout, &buf)
		cmd.Stdout = multi
		cmd.Stderr = multi
		err := cmd.Run()
		exitCode := 0
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if err != nil {
			exitCode = 1
		}

		logPath := filepath.Join(logDir, fmt.Sprintf("%s.log", t.Name))
		logHeader := fmt.Sprintf("Command: docker %s\nExitCode: %d\n\n", strings.Join(args, " "), exitCode)
		if writeErr := os.WriteFile(logPath, append([]byte(logHeader), buf.Bytes()...), 0o644); writeErr != nil {
			return fmt.Errorf("writing log %s: %w", logPath, writeErr)
		}
		fmt.Printf("Log written to %s\n", logPath)
		if err != nil {
			return fmt.Errorf("test %s failed: %w", t.Name, err)
		}
	}
	return nil
}

func imageExists(tag string) (bool, error) {
	cmd := exec.Command("docker", "image", "inspect", tag)
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, fmt.Errorf("checking image %s: %w", tag, err)
	}
	return true, nil
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return values
	}
	sort.Strings(values)
	out := values[:0]
	var last string
	for _, v := range values {
		if v == last {
			continue
		}
		out = append(out, v)
		last = v
	}
	return out
}

func init() {
	templateTestsCmd.Flags().Bool("print-dockerfile", false, "Print generated Dockerfiles to stdout")
	templateTestsCmd.Flags().Bool("build", false, "Build images for the selected templates")
	templateTestsCmd.Flags().Bool("run-tests", false, "Run all tests for the selected templates")
	templateTestsCmd.Flags().StringSlice("tests", nil, "Run only the specified tests (by name or template/test)")
	rootCmd.AddCommand(&templateTestsCmd)
}
