package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/util/progress/progressui"
	"github.com/neurodesk/builder/pkg/jinja2"
	"go.starlark.net/starlark"
	"gopkg.in/yaml.v3"
)

var TOP_LEVEL_DIRECTORIES = []string{
	"/afm01",
	"/afm02",
	"/cvmfs",
	"/90days",
	"/30days",
	"/QRISdata",
	"/RDS",
	"/data",
	"/short",
	"/proc_temp",
	"/TMPDIR",
	"/nvme",
	"/neurodesktop-storage",
	"/local",
	"/gpfs1",
	"/working",
	"/winmounts",
	"/state",
	"/tmp",
	"/autofs",
	"/cluster",
	"/local_mount",
	"/scratch",
	"/clusterdata",
	"/nvmescratch",
}

type repository struct {
	path string
}

func (r *repository) getRecipePath(name string) (string, error) {
	recipePath := filepath.Join(r.path, "recipes", name)
	if _, err := os.Stat(recipePath); os.IsNotExist(err) {
		return "", fmt.Errorf("recipe %s does not exist in repository %s", name, r.path)
	}
	return recipePath, nil
}

func (r *repository) getRecipeDescription(name string) (BuildDescription, error) {
	recipePath, err := r.getRecipePath(name)
	if err != nil {
		return BuildDescription{}, err
	}

	yamlFile := filepath.Join(recipePath, "build.yaml")
	if _, err := os.Stat(yamlFile); os.IsNotExist(err) {
		return BuildDescription{}, fmt.Errorf("build.yaml does not exist for recipe %s in repository %s", name, r.path)
	}

	fh, err := os.Open(yamlFile)
	if err != nil {
		return BuildDescription{}, fmt.Errorf("failed to open build.yaml for recipe %s: %v", name, err)
	}
	defer fh.Close()

	var desc BuildDescription
	if err := yaml.NewDecoder(fh).Decode(&desc); err != nil {
		return BuildDescription{}, fmt.Errorf("failed to decode build.yaml for recipe %s: %v", name, err)
	}

	if err := desc.Validate(); err != nil {
		return BuildDescription{}, fmt.Errorf("validation failed for recipe %s: %v", name, err)
	}

	return desc, nil
}

func newRepository(path string) (*repository, error) {
	if path == "" {
		return nil, fmt.Errorf("repository path cannot be empty")
	}
	return &repository{path: path}, nil
}

type TemplateString string

func (t *TemplateString) Validate() error {
	if t == nil || *t == "" {
		return fmt.Errorf("template string cannot be empty")
	}

	// Try emitting Starlark using Jinja2
	j := jinja2.Jinja2Evaluator{}

	if _, err := j.ToStarlark(string(*t)); err != nil {
		return fmt.Errorf("failed to convert template string to Starlark: %v", err)
	}

	return nil
}

func (t *TemplateString) Evaluate(context *BuildContext) (string, error) {
	if t == nil || *t == "" {
		return "", fmt.Errorf("template string cannot be empty")
	}

	env, err := context.Environment()
	if err != nil {
		return "", fmt.Errorf("failed to get environment: %v", err)
	}

	// Use Jinja2 to evaluate the template string
	j := jinja2.Jinja2Evaluator{}

	result, err := j.Eval(string(*t), env)
	if err != nil {
		return "", fmt.Errorf("failed to evaluate template string: %v", err)
	}

	return result, nil
}

type CopyrightInfo struct {
	License *string `yaml:"license,omitempty"`
	Url     string  `yaml:"url"`
}

type BuildArchitecture string

const (
	BuildArchitectureX86_64  BuildArchitecture = "x86_64"
	BuildArchitectureAarch64 BuildArchitecture = "aarch64"
)

type BuildPackageManager string

const (
	BuildPackageManagerApt BuildPackageManager = "apt"
	BuildPackageManagerYum BuildPackageManager = "yum"
)

type RunDirective []TemplateString

func (r *RunDirective) Validate() error {
	if r == nil || len(*r) == 0 {
		return fmt.Errorf("run directive must contain at least one command")
	}

	return nil
}

func (r *RunDirective) ToLLB(context *BuildContext, state llb.State) (llb.State, error) {
	if r == nil || len(*r) == 0 {
		return state, fmt.Errorf("run directive cannot be empty")
	}
	var fragments []string
	for _, cmd := range *r {
		cmdStr, err := cmd.Evaluate(context)
		if err != nil {
			return state, fmt.Errorf("failed to evaluate run command: %v", err)
		}
		if cmdStr == "" {
			return state, fmt.Errorf("run command cannot be empty")
		}
		fragments = append(fragments, cmdStr)
	}
	return state.Run(llb.Args([]string{
		// TODO(joshua): Allow setting a different shell if needed
		"bash", "-c", strings.Join(fragments, " && "),
	})).Root(), nil
}

type InstallDirective string

func (d *InstallDirective) Validate() error {
	if d == nil || *d == "" {
		return fmt.Errorf("install directive cannot be empty")
	}
	return nil
}

func (d *InstallDirective) ToLLB(context *BuildContext, state llb.State) (llb.State, error) {
	if d == nil || *d == "" {
		return state, fmt.Errorf("install directive cannot be empty")
	}
	if context.PackageManager() == BuildPackageManagerApt {
		run := &RunDirective{
			"apt-get update -qq",
			TemplateString("apt-get install -y -qq --no-install-recommends " + string(*d)),
			"rm -rf /var/lib/apt/lists/*",
		}
		return run.ToLLB(context, state)
	} else if context.PackageManager() == BuildPackageManagerYum {
		return state, fmt.Errorf("yum package manager is not yet supported")
	} else {
		return state, fmt.Errorf("unsupported package manager: %s", context.PackageManager())
	}
}

type GroupDirective []BuildDirective

func (g *GroupDirective) Validate() error {
	if g == nil || len(*g) == 0 {
		return fmt.Errorf("group directive must contain at least one directive")
	}
	for _, directive := range *g {
		if err := directive.Validate(); err != nil {
			return fmt.Errorf("group directive validation failed: %v", err)
		}
	}
	return nil
}

func (g *GroupDirective) ToLLB(context *BuildContext, state llb.State) (llb.State, error) {
	if g == nil || len(*g) == 0 {
		return state, fmt.Errorf("group directive cannot be empty")
	}

	for _, directive := range *g {
		var err error
		state, err = directive.ToLLB(context, state)
		if err != nil {
			return state, fmt.Errorf("failed to convert group directive to LLB: %v", err)
		}
	}

	return state, nil
}

type VariablesDirective map[string]TemplateString

func (v *VariablesDirective) Validate() error {
	if v == nil || len(*v) == 0 {
		return fmt.Errorf("variables directive must contain at least one variable")
	}
	for key, value := range *v {
		if key == "" {
			return fmt.Errorf("variable name cannot be empty")
		}
		if value == "" {
			return fmt.Errorf("variable value cannot be empty for key: %s", key)
		}
	}
	return nil
}

func (v *VariablesDirective) ToLLB(context *BuildContext, state llb.State) (llb.State, error) {
	for key, value := range *v {
		valueStr, err := value.Evaluate(context)
		if err != nil {
			return state, fmt.Errorf("failed to evaluate variable %s: %v", key, err)
		}
		if err := context.SetVariable(key, valueStr); err != nil {
			return state, fmt.Errorf("failed to set variable %s: %v", key, err)
		}
	}

	return state, nil
}

type DeployDirective struct {
	Binaries []string `yaml:"bins,omitempty"`
	Paths    []string `yaml:"path,omitempty"`
}

func (d *DeployDirective) Validate() error {
	if len(d.Binaries) == 0 && len(d.Paths) == 0 {
		return fmt.Errorf("at least one binary or path must be specified in deploy directive")
	}
	if slices.Contains(d.Binaries, "") {
		return fmt.Errorf("binary name cannot be empty in deploy directive")
	}
	if slices.Contains(d.Paths, "") {
		return fmt.Errorf("path cannot be empty in deploy directive")
	}
	return nil
}

func (d *DeployDirective) ToLLB(context *BuildContext, state llb.State) (llb.State, error) {
	context.AddDeployInfo(d)
	return state, nil
}

type BuiltinTest string

const (
	BuiltinTestDeploy BuiltinTest = "test_deploy.sh"
)

type TestDirective struct {
	Name    string       `yaml:"name"`
	Builtin *BuiltinTest `yaml:"builtin,omitempty"`
	Script  *string      `yaml:"script,omitempty"`
}

func (d *TestDirective) Validate() error {
	if d.Name == "" {
		return fmt.Errorf("test directive name is required")
	}
	if d.Builtin == nil && d.Script == nil {
		return fmt.Errorf("either builtin or script must be provided in test directive")
	}
	if d.Builtin != nil && d.Script != nil {
		return fmt.Errorf("only one of builtin or script can be provided in test directive")
	}
	return nil
}

func (d *TestDirective) ToLLB(context *BuildContext, state llb.State) (llb.State, error) {
	// Nothing to do here for now, as we don't have a test implementation yet
	return state, nil
}

type BuildDirective struct {
	Run       *RunDirective       `yaml:"run,omitempty"`
	Install   *InstallDirective   `yaml:"install,omitempty"`
	Group     *GroupDirective     `yaml:"group,omitempty"`
	Variables *VariablesDirective `yaml:"variables,omitempty"`
	Deploy    *DeployDirective    `yaml:"deploy,omitempty"`
	Test      *TestDirective      `yaml:"test,omitempty"`
}

func (d *BuildDirective) Validate() error {
	switch {
	case d.Run != nil:
		return d.Run.Validate()
	case d.Install != nil:
		return d.Install.Validate()
	case d.Group != nil:
		return d.Group.Validate()
	case d.Variables != nil:
		return d.Variables.Validate()
	case d.Deploy != nil:
		return d.Deploy.Validate()
	case d.Test != nil:
		return d.Test.Validate()
	default:
		return fmt.Errorf("no valid directive found")
	}
}

func (d *BuildDirective) ToLLB(context *BuildContext, state llb.State) (llb.State, error) {
	switch {
	case d.Run != nil:
		return d.Run.ToLLB(context, state)
	case d.Install != nil:
		return d.Install.ToLLB(context, state)
	case d.Group != nil:
		return d.Group.ToLLB(context, state)
	case d.Variables != nil:
		return d.Variables.ToLLB(context, state)
	case d.Deploy != nil:
		return d.Deploy.ToLLB(context, state)
	case d.Test != nil:
		return d.Test.ToLLB(context, state)
	default:
		return state, fmt.Errorf("no valid directive found to convert to LLB")
	}
}

type BuildInfo struct {
	Kind               string              `yaml:"kind"`
	BaseImage          string              `yaml:"base-image"`
	PackageManager     BuildPackageManager `yaml:"pkg-manager"`
	AddDefaultTemplate *bool               `yaml:"add-default-template,omitempty"`
	Directives         []BuildDirective    `yaml:"directives"`
}

func (b *BuildInfo) Validate() error {
	if b.Kind != "neurodocker" {
		return fmt.Errorf("unsupported build kind: %s", b.Kind)
	}

	if b.BaseImage == "" {
		return fmt.Errorf("base image is required")
	}
	if b.PackageManager != BuildPackageManagerApt && b.PackageManager != BuildPackageManagerYum {
		return fmt.Errorf("unsupported package manager: %s", b.PackageManager)
	}

	for _, directive := range b.Directives {
		if err := directive.Validate(); err != nil {
			return fmt.Errorf("directive validation failed: %v", err)
		}
	}

	return nil
}

func install(pkgs string) BuildDirective {
	dir := InstallDirective(pkgs)
	return BuildDirective{Install: &dir}
}

func run(cmds ...string) BuildDirective {
	dir := RunDirective{}
	for _, cmd := range cmds {
		dir = append(dir, TemplateString(cmd))
	}
	return BuildDirective{Run: &dir}
}

func runDirectives(context *BuildContext, state llb.State, directives ...BuildDirective) (llb.State, error) {
	for _, directive := range directives {
		if err := directive.Validate(); err != nil {
			return state, fmt.Errorf("failed to validate directive: %v", err)
		}
		newState, err := directive.ToLLB(context, state)
		if err != nil {
			return state, fmt.Errorf("failed to convert directive to LLB: %v", err)
		}
		state = newState
	}
	return state, nil
}

func (b *BuildInfo) ToLLB(context *BuildContext) (llb.State, error) {
	state := llb.Image(b.BaseImage)

	var defaultDirectives []BuildDirective

	if b.AddDefaultTemplate == nil || *b.AddDefaultTemplate {
		defaultDirectives = append(defaultDirectives,
			install("apt-utils bzip2 ca-certificates curl locales unzip"),
			run(
				"sed -i -e 's/# en_US.UTF-8 UTF-8/en_US.UTF-8 UTF-8/' /etc/locale.gen",
				"dpkg-reconfigure --frontend=noninteractive locales",
				`update-locale LANG="en_US.UTF-8"`,
				"chmod 777 /opt && chmod a+s /opt",
			),
		)
	}

	defaultDirectives = append(defaultDirectives, run(
		"printf '#!/bin/bash\\nls -la' > /usr/bin/ll",
		"chmod +x /usr/bin/ll",
		"mkdir -p "+strings.Join(TOP_LEVEL_DIRECTORIES, " "),
	))

	if newState, err := runDirectives(context, state, defaultDirectives...); err != nil {
		return llb.State{}, fmt.Errorf("failed to apply default directives: %v", err)
	} else {
		state = newState
	}

	for _, directive := range b.Directives {
		if newState, err := directive.ToLLB(context, state); err != nil {
			return llb.State{}, fmt.Errorf("failed to convert directive to LLB: %v", err)
		} else {
			state = newState
		}
	}

	// TODO(joshua): Add deploy directive

	// TODO(joshua): Add test scripts

	return state, nil
}

type BuildDescription struct {
	Name          string              `yaml:"name"`
	Version       string              `yaml:"version"`
	Copyright     []CopyrightInfo     `yaml:"copyright,omitempty"`
	Architectures []BuildArchitecture `yaml:"architectures"`
	Build         BuildInfo           `yaml:"build"`
	Categories    []string            `yaml:"categories,omitempty"`
	Readme        TemplateString      `yaml:"readme"`
}

func (d *BuildDescription) Validate() error {
	// Check for a name and version
	if d.Name == "" {
		return fmt.Errorf("name is required")
	}
	if d.Version == "" {
		return fmt.Errorf("version is required")
	}

	// Check for at least one architecture
	if len(d.Architectures) == 0 {
		return fmt.Errorf("at least one architecture must be specified")
	}

	// Check for at least one category
	if len(d.Categories) == 0 {
		return fmt.Errorf("at least one category must be specified")
	}

	// Check for a readme
	if d.Readme == "" {
		return fmt.Errorf("readme is required")
	}

	// Validate the build info
	if err := d.Build.Validate(); err != nil {
		return fmt.Errorf("build info validation failed: %v", err)
	}

	return nil
}

type locals map[string]starlark.Value

func (l *locals) Freeze()               {}
func (l *locals) Hash() (uint32, error) { return 0, fmt.Errorf("locals is not hashable") }
func (l *locals) Truth() starlark.Bool  { return starlark.True }
func (l *locals) Type() string          { return "locals" }

func (l locals) String() string {
	var sb strings.Builder
	sb.WriteString("locals:\n")
	for k, v := range l {
		sb.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
	}
	return sb.String()
}

func (l locals) Attr(name string) (starlark.Value, error) {
	if value, ok := l[name]; ok {
		return value, nil
	}
	return nil, fmt.Errorf("attribute %s not found in locals", name)
}

func (l locals) AttrNames() []string {
	names := make([]string, 0, len(l))
	for name := range l {
		names = append(names, name)
	}
	return names
}

var (
	_ starlark.HasAttrs = &locals{}
)

type BuildContext struct {
	desc       BuildDescription
	deployInfo DeployDirective
	locals     locals
}

func (c *BuildContext) AddDeployInfo(d *DeployDirective) {
	if d == nil {
		return
	}

	if c.deployInfo.Binaries == nil {
		c.deployInfo.Binaries = []string{}
	}
	if c.deployInfo.Paths == nil {
		c.deployInfo.Paths = []string{}
	}

	c.deployInfo.Binaries = append(c.deployInfo.Binaries, d.Binaries...)
	c.deployInfo.Paths = append(c.deployInfo.Paths, d.Paths...)
}

func (c *BuildContext) SetVariable(name, value string) error {
	c.locals[name] = starlark.String(value)
	return nil
}

func (c *BuildContext) PackageManager() BuildPackageManager {
	return c.desc.Build.PackageManager
}

func (c *BuildContext) Tag() string {
	return fmt.Sprintf("%s:%s", c.desc.Name, c.desc.Version)
}

func (c *BuildContext) Environment() ([]starlark.Tuple, error) {
	var ret []starlark.Tuple

	ret = append(ret, starlark.Tuple{starlark.String("local"), &c.locals})
	ret = append(ret, starlark.Tuple{starlark.String("parallel_jobs"), starlark.String("1")})

	return ret, nil
}

func (c *BuildContext) ToLLB() (llb.State, error) {
	buildState, err := c.desc.Build.ToLLB(c)
	if err != nil {
		return llb.State{}, fmt.Errorf("failed to convert build info to LLB state: %v", err)
	}

	// Add the README.md file
	readmeContents, err := c.desc.Readme.Evaluate(c)
	if err != nil {
		return llb.State{}, fmt.Errorf("failed to evaluate readme template: %v", err)
	}
	buildState = buildState.File(llb.Mkfile("/README.md", 0644, []byte(readmeContents)))

	// Stringify the build description and add it as /build.yaml
	descBytes, err := yaml.Marshal(c.desc)
	if err != nil {
		return llb.State{}, fmt.Errorf("failed to marshal build description: %v", err)
	}
	buildState = buildState.File(llb.Mkfile("/build.yaml", 0644, descBytes))

	return buildState, nil
}

func (c *BuildContext) Build(ctx context.Context) error {
	state, err := c.ToLLB()
	if err != nil {
		return fmt.Errorf("failed to convert build context to LLB state: %v", err)
	}
	_ = state

	bk, err := client.New(ctx, "", client.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		c1, c2 := net.Pipe()
		cmd := exec.Command("docker", "buildx", "dial-stdio")
		cmd.Stdin = c1
		cmd.Stdout = c1

		if err := cmd.Start(); err != nil {
			c1.Close()
			c2.Close()
			return nil, err
		}

		go func() {
			cmd.Wait()
			c2.Close()
		}()

		return c2, nil
	}))
	if err != nil {
		return fmt.Errorf("failed to create buildkit client: %v", err)
	}
	defer bk.Close()

	tag := c.Tag()

	def, err := state.Marshal(ctx, llb.LinuxAmd64)
	if err != nil {
		return fmt.Errorf("failed to marshal LLB state: %v", err)
	}

	solveOpt := client.SolveOpt{
		Exports: []client.ExportEntry{
			{
				// when your using docker this should be used instead of client.ExporterImage
				// https://github.com/moby/buildkit/issues/5637
				Type:  "moby",
				Attrs: map[string]string{"name": tag},
			},
		},
	}

	d, err := progressui.NewDisplay(os.Stdout, progressui.DefaultMode)
	if err != nil {
		return fmt.Errorf("failed to create progress display: %v", err)
	}

	statusChan := make(chan *client.SolveStatus)

	go func() {
		warnings, err := d.UpdateFrom(context.Background(), statusChan)
		if err != nil {
			slog.Error("failed to update progress display", "error", err)
			return
		}

		for _, warning := range warnings {
			slog.Warn("build warning", "warning", warning)
		}
	}()

	// Create a new build request
	resp, err := bk.Solve(ctx, def, solveOpt, statusChan)
	if err != nil {
		return fmt.Errorf("failed to solve build: %v", err)
	}

	slog.Info("build completed", "resp", resp)

	return nil
}

func NewBuildContext(repo *repository, recipe string) (*BuildContext, error) {
	desc, err := repo.getRecipeDescription(recipe)
	if err != nil {
		return nil, fmt.Errorf("failed to get recipe description: %v", err)
	}

	if err := desc.Validate(); err != nil {
		return nil, fmt.Errorf("recipe description validation failed: %v", err)
	}

	return &BuildContext{
		desc:   desc,
		locals: make(locals),
	}, nil
}

var (
	repo   = flag.String("repo", "", "Path to the repository")
	recipe = flag.String("recipe", "", "Recipe name to build")
)

func appMain() error {
	flag.Parse()

	if *repo == "" {
		return fmt.Errorf("repository path is required")
	}
	if *recipe == "" {
		return fmt.Errorf("recipe name is required")
	}

	repo, err := newRepository(*repo)
	if err != nil {
		return fmt.Errorf("failed to create repository: %v", err)
	}

	ctx, err := NewBuildContext(repo, *recipe)
	if err != nil {
		return fmt.Errorf("failed to create build context: %v", err)
	}

	if err := ctx.Build(context.Background()); err != nil {
		return fmt.Errorf("failed to build recipe %s: %v", *recipe, err)
	}

	return nil
}

func main() {
	if err := appMain(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}
