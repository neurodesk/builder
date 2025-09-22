package recipe

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/neurodesk/builder/pkg/common"
	"github.com/neurodesk/builder/pkg/ir"
	"github.com/neurodesk/builder/pkg/jinja2"
	starlarkpkg "github.com/neurodesk/builder/pkg/starlark"
	"github.com/neurodesk/builder/pkg/templates"
	v "github.com/neurodesk/builder/pkg/validator"
	"go.yaml.in/yaml/v4"
)

var GLOBAL_MOUNT_POINT_LIST = []string{
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

type file interface {
	isFile()

	GetName() string
}

type contextFile struct {
	Name         string
	HostFilename string
	Executable   bool
}

func (c contextFile) isFile() {}

func (c contextFile) GetName() string { return c.Name }

type httpFile struct {
	Name       string
	URL        string
	Executable bool
	Retry      *int
	Insecure   *bool
}

func (h httpFile) isFile() {}

func (h httpFile) GetName() string { return h.Name }

type literalFile struct {
	Name       string
	Contents   string
	Executable bool
}

func (l literalFile) isFile() {}

func (l literalFile) GetName() string { return l.Name }

var (
	_ file = contextFile{}
	_ file = httpFile{}
	_ file = literalFile{}
)

type Context struct {
	PackageManager     common.PackageManager
	Version            string
	OriginalVersion    string
	IncludeDirectories []string
	Arch               CPUArchitecture

	builder   ir.Builder
	parent    *Context
	variables map[string]jinja2.Value
	files     map[string]file

	deployBins []string
	deployPath []string

	// Accumulated commands from Starlark run_command builtins
	runCommands []string
}

// OnLookup implements jinja2.LookupHook.
func (c Context) OnLookup(key string) (jinja2.Value, bool) {
	switch key {
	case "version":
		return jinja2.FromGo(c.Version), true
	case "original_version":
		return jinja2.FromGo(c.OriginalVersion), true
	case "has_local":
		return jinja2.CallableValue{Fn: func(args []jinja2.Value) (jinja2.Value, error) {
			// Local contexts are not yet wired; default to false.
			return jinja2.BoolValue(false), nil
		}}, true
	case "get_local":
		return jinja2.CallableValue{Fn: func(args []jinja2.Value) (jinja2.Value, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("get_local expects 1 argument")
			}
			key := args[0].String()
			return jinja2.StringValue("/.neurocontainer-local/" + key), nil
		}}, true
	case "get_file":
		return jinja2.CallableValue{Fn: func(args []jinja2.Value) (jinja2.Value, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("get_file expects 1 argument")
			}
			name := args[0].String()
			// If a file with this name exists in the recipe context, return a stable path
			// inside the image where it would be placed. For now, do not materialize; this
			// is for template rendering only.
			if _, ok := c.files[name]; ok {
				return jinja2.StringValue("/.neurocontainer-cache/" + name), nil
			}
			// Return a best-effort path regardless, to allow rendering to proceed.
			return jinja2.StringValue("/.neurocontainer-cache/" + name), nil
		}}, true
	default:
		if val, ok := c.variables[key]; ok {
			return val, true
		}

		if c.parent != nil {
			return c.parent.OnLookup(key)
		}

		// not found
		return nil, false
	}
}

// String implements jinja2.Value.
func (c Context) String() string {
	return "<context>"
}

// Truth implements jinja2.Value.
func (c Context) Truth() bool {
	return true
}

func (c *Context) SetVariable(key string, value any) {
	c.variables[key] = jinja2.FromGo(value)
}

// AddRunCommand implements starlark.RecipeContext hook to accumulate commands.
func (c *Context) AddRunCommand(cmd string) { c.runCommands = append(c.runCommands, cmd) }

// EvaluateValue is a public wrapper for evaluateValue to satisfy the RecipeContext interface
func (c *Context) EvaluateValue(value any) (any, error) {
	return c.evaluateValue(value)
}

// InstallPackages is a public wrapper for installPackages to satisfy the RecipeContext interface
func (c *Context) InstallPackages(pkgs ...string) error {
	return c.installPackages(pkgs...)
}

func (c *Context) Compile() (*ir.Definition, error) {
	return c.builder.Compile()
}

func (c *Context) childContext() *Context {
	return newContext(
		c.PackageManager,
		c.Version,
		c.IncludeDirectories,
		c.builder,
		c,
	)
}

func (c *Context) parallelJobs() int {
	return 1
}

func (c *Context) evaluateValue(value any) (any, error) {
	switch val := value.(type) {
	case jinja2.TemplateString:
		ctx := jinja2.Context{
			"context":       c,
			"local":         c,
			"parallel_jobs": jinja2.IntValue(c.parallelJobs()),
			"arch":          jinja2.StringValue(string(c.Arch)),
		}
		// Top-level helpers to match Python builder methods
		ctx["has_local"] = jinja2.CallableValue{Fn: func(args []jinja2.Value) (jinja2.Value, error) {
			return jinja2.BoolValue(false), nil
		}}
		ctx["get_local"] = jinja2.CallableValue{Fn: func(args []jinja2.Value) (jinja2.Value, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("get_local expects 1 argument")
			}
			return jinja2.StringValue("/.neurocontainer-local/" + args[0].String()), nil
		}}
		ctx["get_file"] = jinja2.CallableValue{Fn: func(args []jinja2.Value) (jinja2.Value, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("get_file expects 1 argument")
			}
			name := args[0].String()
			if _, ok := c.files[name]; ok {
				return jinja2.StringValue("/.neurocontainer-cache/" + name), nil
			}
			return jinja2.StringValue("/.neurocontainer-cache/" + name), nil
		}}

		ret, err := val.Render(ctx)
		if err != nil {
			return nil, fmt.Errorf("rendering template: %w", err)
		}

		return ret, nil
	case string:
		tpl := jinja2.TemplateString(val)
		ctx := jinja2.Context{
			"local":         c,
			"context":       c,
			"parallel_jobs": jinja2.IntValue(c.parallelJobs()),
			"arch":          jinja2.StringValue(string(c.Arch)),
		}
		ctx["has_local"] = jinja2.CallableValue{Fn: func(args []jinja2.Value) (jinja2.Value, error) {
			return jinja2.BoolValue(false), nil
		}}
		ctx["get_local"] = jinja2.CallableValue{Fn: func(args []jinja2.Value) (jinja2.Value, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("get_local expects 1 argument")
			}
			return jinja2.StringValue("/.neurocontainer-local/" + args[0].String()), nil
		}}
		ctx["get_file"] = jinja2.CallableValue{Fn: func(args []jinja2.Value) (jinja2.Value, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("get_file expects 1 argument")
			}
			name := args[0].String()
			if _, ok := c.files[name]; ok {
				return jinja2.StringValue("/.neurocontainer-cache/" + name), nil
			}
			return jinja2.StringValue("/.neurocontainer-cache/" + name), nil
		}}

		ret, err := tpl.Render(ctx)
		if err != nil {
			return nil, fmt.Errorf("rendering template: %w", err)
		}

		return ret, nil
	case []any:
		// Evaluate each element recursively
		out := make([]any, 0, len(val))
		for i := range val {
			v, err := c.evaluateValue(val[i])
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	case VariablesDirective:
		// Treat like a generic map for evaluation
		mv := map[string]any(val)
		return c.evaluateValue(mv)
	case map[string]any:
		// Support special "try" structure like the Python builder
		if tv, ok := val["try"]; ok {
			lst, ok := tv.([]any)
			if !ok {
				return nil, fmt.Errorf("'try' must be a list, got %T", tv)
			}
			// Prepare Jinja context for condition evaluation
			condCtx := jinja2.Context{
				"context":       c,
				"local":         c,
				"parallel_jobs": jinja2.IntValue(c.parallelJobs()),
				"arch":          jinja2.StringValue(string(c.Arch)),
			}
			// Also expose helpers at top-level for conditions if needed
			condCtx["has_local"] = jinja2.CallableValue{Fn: func(args []jinja2.Value) (jinja2.Value, error) {
				return jinja2.BoolValue(false), nil
			}}
			condCtx["get_local"] = jinja2.CallableValue{Fn: func(args []jinja2.Value) (jinja2.Value, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("get_local expects 1 argument")
				}
				return jinja2.StringValue("/.neurocontainer-local/" + args[0].String()), nil
			}}
			condCtx["get_file"] = jinja2.CallableValue{Fn: func(args []jinja2.Value) (jinja2.Value, error) {
				if len(args) != 1 {
					return nil, fmt.Errorf("get_file expects 1 argument")
				}
				name := args[0].String()
				if _, ok := c.files[name]; ok {
					return jinja2.StringValue("/.neurocontainer-cache/" + name), nil
				}
				return jinja2.StringValue("/.neurocontainer-cache/" + name), nil
			}}

			ev := jinja2.NewEvaluator()
			for _, it := range lst {
				// Accept either a plain map[string]any or a VariablesDirective (alias of map[string]any)
				var m map[string]any
				switch t := it.(type) {
				case map[string]any:
					m = t
				case VariablesDirective:
					m = map[string]any(t)
				default:
					return nil, fmt.Errorf("'try' items must be maps, got %T", it)
				}
				cond, _ := m["condition"].(string)
				valAny, hasVal := m["value"]
				if cond == "" || !hasVal {
					continue
				}
				okTruth, err := ev.Truthy(cond, condCtx)
				if err != nil {
					return nil, fmt.Errorf("evaluating condition %q: %w", cond, err)
				}
				if okTruth {
					return c.evaluateValue(valAny)
				}
			}
			return nil, fmt.Errorf("no 'try' conditions matched")
		}
		// Default: evaluate each entry
		out := map[string]any{}
		for k, v := range val {
			rv, err := c.evaluateValue(v)
			if err != nil {
				return nil, err
			}
			out[k] = rv
		}
		return out, nil
	case bool:
		return val, nil
	default:
		return nil, fmt.Errorf("unsupported value type: %T", val)
	}
}

func (c *Context) installPackages(pkgs ...string) error {
	switch c.PackageManager {
	case common.PkgManagerApt:
		cmd := "apt-get update && apt-get install -y " + strings.Join(pkgs, " ")
		c.builder = c.builder.AddRunCommand(cmd)
		return nil
	case common.PkgManagerYum:
		cmd := "yum install -y " + strings.Join(pkgs, " ")
		c.builder = c.builder.AddRunCommand(cmd)
	default:
		return fmt.Errorf("unsupported package manager: %s", c.PackageManager)
	}
	return nil
}

func (c *Context) addFile(f file) error {
	name := f.GetName()
	// check if a file with the same name already exists
	if _, exists := c.files[name]; exists {
		return fmt.Errorf("file with name %q already exists", name)
	}
	c.files[name] = f
	return nil
}

func (c *Context) addBuiltinTest(name string, manual bool, builtin string) {
	// TODO(joshua): Handle tests
}

func (c *Context) addScriptTest(name string, manual bool, executable string, script string) {
	// TODO(joshua): Handle tests
}

var (
	_ jinja2.Value      = Context{}
	_ jinja2.LookupHook = Context{}
)

func newContext(
	packageManager common.PackageManager,
	version string,
	includeDirs []string,
	builder ir.Builder,
	parent *Context,
) *Context {
	return &Context{
		PackageManager:     packageManager,
		Version:            version,
		OriginalVersion:    version,
		IncludeDirectories: includeDirs,
		Arch:               CPUArchAMD64,

		builder:   builder,
		parent:    parent,
		variables: map[string]jinja2.Value{},
		files:     map[string]file{},
	}
}

// shellWords parses a shell-like word string into tokens, supporting simple quotes and escapes.
func shellWords(s string) ([]string, error) {
	var out []string
	var cur strings.Builder
	inS, inD := false, false
	esc := false
	for _, r := range s {
		switch {
		case esc:
			cur.WriteRune(r)
			esc = false
		case r == '\\' && (inS || inD):
			esc = true
		case r == '\'' && !inD:
			inS = !inS
		case r == '"' && !inS:
			inD = !inD
		case r == ' ' && !inS && !inD:
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if inS || inD {
		return nil, fmt.Errorf("unclosed quote in %q", s)
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out, nil
}

type CPUArchitecture string

const (
	CPUArchAMD64 CPUArchitecture = "x86_64"
	CPUArchARM64 CPUArchitecture = "aarch64"
)

type StructuredReadme struct {
	Description   string `yaml:"description,omitempty"`
	Documentation string `yaml:"documentation,omitempty"`
	Example       string `yaml:"example,omitempty"`
	Citation      string `yaml:"citation,omitempty"`
}

type Copyright struct {
	// Custom name for license.
	Name string `yaml:"name,omitempty"`
	// SPDX License Identifier, e.g. "MIT", "GPL-3.0-or-later", "Apache-2.0"
	License string `yaml:"license,omitempty"`
	// URL to license text.
	URL string `yaml:"url,omitempty"`
}

type Category string

type DeployInfo struct {
	Bins []jinja2.TemplateString `yaml:"bins,omitempty"`
	Path []jinja2.TemplateString `yaml:"path,omitempty"`
}

type FileInfo struct {
	Name       string `yaml:"name"`
	Executable bool   `yaml:"executable,omitempty"`
	Retry      *int   `yaml:"retry,omitempty"`
	Insecure   *bool  `yaml:"insecure,omitempty"`

	// Only one of the following should be set.
	Filename string `yaml:"filename,omitempty"` // Path to a file to include.
	Url      string `yaml:"url,omitempty"`      // URL to download file from.
	Contents string `yaml:"contents,omitempty"` // Literal contents of the file.
}

type GuiApp struct {
	Name string `yaml:"name"`
	Exec string `yaml:"exec"`
}

type OptionInfo struct {
	Description   string `yaml:"description,omitempty"`
	Default       any    `yaml:"default,omitempty"`
	VersionSuffix string `yaml:"version_suffix,omitempty"`
}

type TestBuiltin string

type TestInfo struct {
	Name   string `yaml:"name"`
	Manual bool   `yaml:"manual,omitempty"`

	Executable jinja2.TemplateString `yaml:"executable,omitempty"`
	Script     jinja2.TemplateString `yaml:"script,omitempty"`
	Builtin    TestBuiltin           `yaml:"builtin,omitempty"`
}

type BuildKind string

const (
	BuildKindNeuroDocker BuildKind = "neurodocker"
)

type GroupDirective []Directive

func (g GroupDirective) Validate(ctx Context) error {
	return v.Map(g, func(directive Directive, description string) error {
		return directive.Validate(ctx)
	}, "group")
}

func (g GroupDirective) Apply(ctx *Context, with map[string]any) error {
	child := ctx.childContext()

	for k, v := range with {
		result, err := ctx.evaluateValue(v)
		if err != nil {
			return fmt.Errorf("evaluating 'with' variable %q: %w", k, err)
		}
		child.SetVariable(k, result)
	}

	for _, directive := range g {
		if err := directive.Apply(child); err != nil {
			return fmt.Errorf("applying group directive: %w", err)
		}
	}

	// Propagate builder changes back to the parent.
	ctx.builder = child.builder
	// Optionally propagate variables and files to parent to make groups transparent.
	// Prefer parent values on conflict.
	for k, v := range child.variables {
		if _, exists := ctx.variables[k]; !exists {
			ctx.variables[k] = v
		}
	}
	for name, f := range child.files {
		if _, exists := ctx.files[name]; !exists {
			ctx.files[name] = f
		}
	}
	// Accumulate any run commands produced in child
	if len(child.runCommands) > 0 {
		ctx.runCommands = append(ctx.runCommands, child.runCommands...)
	}
	return nil
}

type RunDirective []jinja2.TemplateString

func (r RunDirective) Validate() error {
	return v.Map(r, func(cmd jinja2.TemplateString, description string) error {
		return cmd.Validate()
	}, "run")
}

func (r RunDirective) Apply(ctx *Context) error {
	// Use a stable, named local context for cache files.
	// The CLI will provide --build-context cache=<dir>.
	targetBase := "/.neurocontainer-cache"
	cacheMount := "--mount=type=bind,from=cache,source=/,target=/.neurocontainer-cache,readonly"

	// Track mounts (dedup)
	seenMount := map[string]struct{}{}
	mounts := []string{}
	addMount := func(m string) {
		if _, ok := seenMount[m]; ok {
			return
		}
		seenMount[m] = struct{}{}
		mounts = append(mounts, m)
	}

	// Expose helpers that register mounts while rendering
	makeCtx := func() jinja2.Context {
		jctx := jinja2.Context{
			"local":         ctx,
			"context":       ctx,
			"parallel_jobs": jinja2.IntValue(ctx.parallelJobs()),
			"arch":          jinja2.StringValue(string(ctx.Arch)),
		}
		jctx["has_local"] = jinja2.CallableValue{Fn: func(args []jinja2.Value) (jinja2.Value, error) {
			return jinja2.BoolValue(false), nil
		}}
		jctx["get_local"] = jinja2.CallableValue{Fn: func(args []jinja2.Value) (jinja2.Value, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("get_local expects 1 argument")
			}
			key := args[0].String()
			m := fmt.Sprintf("--mount=type=bind,from=%s,source=/,target=/.neurocontainer-local/%s,readonly", key, key)
			addMount(m)
			return jinja2.StringValue("/.neurocontainer-local/" + key), nil
		}}
		jctx["get_file"] = jinja2.CallableValue{Fn: func(args []jinja2.Value) (jinja2.Value, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("get_file expects 1 argument")
			}
			name := args[0].String()
			// Register the cache bind mount when first needed
			addMount(cacheMount)
			return jinja2.StringValue(targetBase + "/" + name), nil
		}}
		return jctx
	}

	var commands []string
	for _, cmd := range r {
		// Render with mount-collecting helpers
		rendered, err := cmd.Render(makeCtx())
		if err != nil {
			return fmt.Errorf("evaluating run command: %w", err)
		}
		commands = append(commands, rendered)
	}

	joined := strings.Join(commands, " &&\n ")
	if len(mounts) > 0 {
		ctx.builder = ctx.builder.AddRunWithMounts(mounts, joined)
	} else {
		ctx.builder = ctx.builder.AddRunCommand(joined)
	}
	return nil
}

type FileDirective FileInfo

func (f FileDirective) Validate() error {
	return v.All(
		v.NotEmpty(f.Name, "file.name"),
		func() error {
			count := 0
			if f.Filename != "" {
				count++
			}
			if f.Url != "" {
				count++
			}
			if f.Contents != "" {
				count++
			}
			if count == 0 {
				return fmt.Errorf("file must have one of filename, url, or contents")
			}
			if count > 1 {
				return fmt.Errorf("file must have only one of filename, url, or contents")
			}
			return nil
		}(),
	)
}

func (f FileDirective) Apply(ctx *Context) error {
	if f.Filename != "" {
		return ctx.addFile(contextFile{
			Name:         f.Name,
			HostFilename: f.Filename,
			Executable:   f.Executable,
		})
	} else if f.Url != "" {
		return ctx.addFile(httpFile{
			Name:       f.Name,
			URL:        f.Url,
			Executable: f.Executable,
			Retry:      f.Retry,
			Insecure:   f.Insecure,
		})
	} else if f.Contents != "" {
		return ctx.addFile(literalFile{
			Name:       f.Name,
			Contents:   f.Contents,
			Executable: f.Executable,
		})
	} else {
		return fmt.Errorf("file directive not implemented")
	}
}

type InstallDirective any // string or []string

type UserDirective jinja2.TemplateString

func (u UserDirective) Validate() error {
	return jinja2.TemplateString(u).Validate()
}

func (u UserDirective) Apply(ctx *Context) error {
	result, err := ctx.evaluateValue(jinja2.TemplateString(u))
	if err != nil {
		return fmt.Errorf("evaluating user: %w", err)
	}
	s, ok := result.(string)
	if !ok {
		return fmt.Errorf("user must be a string, got %T", result)
	}
	ctx.builder = ctx.builder.SetCurrentUser(s)
	return nil
}

type WorkDirDirective jinja2.TemplateString

func (w WorkDirDirective) Validate() error {
	return jinja2.TemplateString(w).Validate()
}

func (w WorkDirDirective) Apply(ctx *Context) error {
	val, err := ctx.evaluateValue(jinja2.TemplateString(w))
	if err != nil {
		return fmt.Errorf("evaluating workdir: %w", err)
	}
	s, ok := val.(string)
	if !ok {
		return fmt.Errorf("workdir must be a string, got %T", val)
	}
	ctx.builder = ctx.builder.SetWorkingDirectory(s)
	return nil
}

type EntryPointDirective jinja2.TemplateString

func (e EntryPointDirective) Validate() error {
	return jinja2.TemplateString(e).Validate()
}

func (e EntryPointDirective) Apply(ctx *Context) error {
	val, err := ctx.evaluateValue(jinja2.TemplateString(e))
	if err != nil {
		return fmt.Errorf("evaluating entrypoint: %w", err)
	}

	s, ok := val.(string)
	if !ok {
		return fmt.Errorf("entrypoint must be a string, got %T", val)
	}

	ctx.builder = ctx.builder.SetEntryPoint(s)
	return nil
}

type DeployDirective DeployInfo

func (d DeployDirective) Validate() error {
	return v.All(
		v.Map(d.Bins, func(cmd jinja2.TemplateString, description string) error {
			return cmd.Validate()
		}, "deploy.bins"),
		v.Map(d.Path, func(cmd jinja2.TemplateString, description string) error {
			return cmd.Validate()
		}, "deploy.path"),
	)
}

func (d DeployDirective) Apply(ctx *Context) error {
	if len(d.Bins) > 0 {
		var bins []string
		for _, cmd := range d.Bins {
			result, err := ctx.evaluateValue(cmd)
			if err != nil {
				return fmt.Errorf("evaluating deploy.bin command: %w", err)
			}
			s, ok := result.(string)
			if !ok {
				return fmt.Errorf("deploy.bin command must be a string, got %T", result)
			}
			bins = append(bins, s)
		}
		ctx.deployBins = append(ctx.deployBins, bins...)
	}

	if len(d.Path) > 0 {
		var path []string
		for _, cmd := range d.Path {
			result, err := ctx.evaluateValue(cmd)
			if err != nil {
				return fmt.Errorf("evaluating deploy.path command: %w", err)
			}
			s, ok := result.(string)
			if !ok {
				return fmt.Errorf("deploy.path command must be a string, got %T", result)
			}
			path = append(path, s)
		}
		ctx.deployPath = append(ctx.deployPath, path...)
	}

	return nil
}

type EnvironmentDirective map[string]jinja2.TemplateString

func (e EnvironmentDirective) Validate() error {
	for k, val := range e {
		if err := v.HasNoJinja(k, "environment key"); err != nil {
			return err
		}
		if err := val.Validate(); err != nil {
			return fmt.Errorf("environment[%q]: %w", k, err)
		}
	}
	return nil
}

func (e EnvironmentDirective) Apply(ctx *Context) error {
	env := map[string]string{}
	for key, val := range e {
		result, err := ctx.evaluateValue(val)
		if err != nil {
			return fmt.Errorf("evaluating environment[%q]: %w", key, err)
		}
		s, ok := result.(string)
		if !ok {
			return fmt.Errorf("environment[%q] must be a string, got %T", key, result)
		}
		env[key] = s
	}
	ctx.builder = ctx.builder.AddEnvironment(env)
	return nil
}

type TestDirective TestInfo

func (t TestDirective) Validate() error {
	return v.All(
		v.NotEmpty(t.Name, "test.name"),
		func() error {
			count := 0
			if t.Script != "" {
				count++
			}
			if t.Builtin != "" {
				count++
			}
			if count == 0 {
				return fmt.Errorf("test must have one of script, or builtin")
			}
			if count > 1 {
				return fmt.Errorf("test must have only one of script, or builtin")
			}
			return nil
		}(),
		t.Executable.Validate(),
		t.Script.Validate(),
	)
}

func (t TestDirective) Apply(ctx *Context) error {
	if t.Builtin != "" {
		ctx.addBuiltinTest(
			t.Name,
			t.Manual,
			string(t.Builtin),
		)
		return nil
	} else if t.Script != "" {
		result, err := ctx.evaluateValue(t.Script)
		if err != nil {
			return fmt.Errorf("evaluating test script: %w", err)
		}
		script, ok := result.(string)
		if !ok {
			return fmt.Errorf("test script must be a string, got %T", result)
		}

		execResult, err := ctx.evaluateValue(t.Executable)
		if err != nil {
			return fmt.Errorf("evaluating test executable: %w", err)
		}
		executable, ok := execResult.(string)
		if !ok {
			return fmt.Errorf("test executable must be a string, got %T", execResult)
		}

		ctx.addScriptTest(
			t.Name,
			t.Manual,
			executable,
			script,
		)
		return nil
	} else {
		return fmt.Errorf("test directive not implemented")
	}
}

type TemplateDirective struct {
	Name   string         `yaml:"name"`
	Params map[string]any `yaml:",inline,omitempty"`
}

func (t TemplateDirective) Validate(ctx Context) error {
	if err := v.NotEmpty(t.Name, "template.name"); err != nil {
		return err
	}

	tpl, err := templates.Get(t.Name)
	if err != nil {
		return fmt.Errorf("template %q not found", t.Name)
	}

	_ = tpl

	return nil
}

func (t TemplateDirective) Apply(ctx *Context) error {
	tpl, err := templates.Get(t.Name)
	if err != nil {
		return fmt.Errorf("template %q not found", t.Name)
	}

	result, err := tpl.Execute(templates.Context{
		PackageManager: ctx.PackageManager,
	}, func(k string) (any, bool, error) {
		if val, ok := t.Params[k]; ok {
			rss, err := ctx.evaluateValue(val)
			if err != nil {
				return nil, false, fmt.Errorf("evaluating template param %q: %w", k, err)
			}

			return rss, true, nil
		}
		return nil, false, nil
	})
	if err != nil {
		return fmt.Errorf("executing template %q: %w", t.Name, err)
	}

	if len(result.Environment) > 0 {
		env := map[string]jinja2.TemplateString{}
		for k, v := range result.Environment {
			env[k] = jinja2.TemplateString(v)
		}

		if err := EnvironmentDirective(env).Apply(ctx); err != nil {
			return fmt.Errorf("applying template %q environment: %w", t.Name, err)
		}
	}

	if err := RunDirective([]jinja2.TemplateString{
		jinja2.TemplateString(result.Instructions),
	}).Apply(ctx); err != nil {
		return fmt.Errorf("applying template %q run: %w", t.Name, err)
	}

	return nil
}

type IncludeDirective string

func (i IncludeDirective) Validate() error {
	return v.HasNoJinja(string(i), "include")
}

func (i IncludeDirective) Apply(ctx *Context) error {
	path := string(i)

	var fullPath string

	for _, dir := range ctx.IncludeDirectories {
		fullPath = filepath.Join(dir, path)
		if _, err := os.Stat(fullPath); err != nil {
			if os.IsNotExist(err) {
				fullPath = ""
				continue
			}
			return fmt.Errorf("stating include file %q: %w", fullPath, err)
		}
	}

	if fullPath == "" {
		return fmt.Errorf("include file %q not found in include directories", path)
	}

	f, err := os.Open(fullPath)
	if err != nil {
		return err
	}

	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)

	var build IncludeFile
	if err := dec.Decode(&build); err != nil {
		return err
	}

	var group GroupDirective
	for _, directive := range build.Directives {
		group = append(group, directive)
	}
	return group.Apply(ctx, map[string]any{})
}

type CopyDirective any // string or []string

type VariablesDirective map[string]any

func (v VariablesDirective) Validate() error {
	return nil
}

func (v VariablesDirective) Apply(ctx *Context) error {
	// Evaluate variables with dependency resolution across passes.
	// Some entries may reference others (e.g., context.foo used by bar).
	// Iterate until all are set or no progress.
	// Use stable key order for determinism.
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	set := map[string]bool{}
	remaining := len(keys)
	var lastErr error
	for pass := 0; pass < len(keys)*2 && remaining > 0; pass++ {
		progressed := false
		for _, k := range keys {
			if set[k] {
				continue
			}
			val := v[k]
			result, err := ctx.evaluateValue(val)
			if err != nil {
				// Keep last error to report if we cannot resolve.
				lastErr = fmt.Errorf("evaluating variable %q: %w", k, err)
				continue
			}
			ctx.SetVariable(k, result)
			set[k] = true
			remaining--
			progressed = true
		}
		if !progressed {
			break
		}
	}
	if remaining > 0 && lastErr != nil {
		return lastErr
	}
	return nil
}

type BoutiqueInput struct {
	Id              string   `yaml:"id"`
	Name            string   `yaml:"name"`
	Description     string   `yaml:"description,omitempty"`
	ValueKey        string   `yaml:"value-key"`
	Type            string   `yaml:"type"`
	Optional        bool     `yaml:"optional,omitempty"`
	CommandLineFlag string   `yaml:"command-line-flag,omitempty"`
	ValueChoices    []string `yaml:"value-choices,omitempty"`
	List            bool     `yaml:"list,omitempty"`
}

type BoutiqueDirective struct {
	SchemaVersion string `yaml:"schema-version,omitempty"`

	Name               string            `yaml:"name"`
	Description        string            `yaml:"description,omitempty"`
	Author             string            `yaml:"author,omitempty"`
	Tags               map[string]string `yaml:"tags,omitempty"`
	URL                string            `yaml:"url,omitempty"`
	ToolVersion        string            `yaml:"tool-version,omitempty"`
	CommandLine        string            `yaml:"command-line,omitempty"`
	SuggestedResources map[string]string `yaml:"suggested-resources,omitempty"`

	Inputs []BoutiqueInput `yaml:"inputs,omitempty"`
}

func (b BoutiqueDirective) Validate() error {
	return v.All(
		v.NotEmpty(b.Name, "boutique.name"),
		v.NotEmpty(b.CommandLine, "boutique.command-line"),
		v.Map(b.Inputs, func(input BoutiqueInput, description string) error {
			return v.All(
				v.NotEmpty(input.Id, description+".id"),
				v.NotEmpty(input.Name, description+".name"),
				v.NotEmpty(input.ValueKey, description+".value-key"),
				v.NotEmpty(input.Type, description+".type"),
			)
		}, "boutique.inputs"),
	)
}

func (b BoutiqueDirective) Apply(ctx *Context) error {
	// serialize boutique directive to JSON
	data, err := json.Marshal(b)
	if err != nil {
		return fmt.Errorf("serializing boutique directive: %w", err)
	}

	// add boutique.json file to image
	// TODO(joshua): this is probably incorrect compared to the Python version
	ctx.builder = ctx.builder.AddLiteralFile("/boutique.json", string(data), false)

	return nil
}

type StarlarkDirective struct {
	Script jinja2.TemplateString `yaml:"script,omitempty"`
	File   string                `yaml:"file,omitempty"`
}

func (s StarlarkDirective) Validate(ctx Context) error {
	// Exactly one of script or file must be specified
	count := 0
	if s.Script != "" {
		count++
	}
	if s.File != "" {
		count++
	}
	if count == 0 {
		return fmt.Errorf("starlark directive must have either 'script' or 'file'")
	}
	if count > 1 {
		return fmt.Errorf("starlark directive must have only one of 'script' or 'file'")
	}

	if s.Script != "" {
		return s.Script.Validate()
	}

	if s.File != "" {
		return v.HasNoJinja(s.File, "starlark.file")
	}

	return nil
}

func (s StarlarkDirective) Apply(ctx *Context) error {
	// Create Starlark evaluator with enhanced context
	eval := starlarkpkg.NewEvaluatorWithStarlarkContext(ctx)

	// Prepare context variables for Starlark
	jinjaCtx := jinja2.Context{
		"version":        jinja2.StringValue(ctx.Version),
		"parallel_jobs":  jinja2.IntValue(ctx.parallelJobs()),
		"PackageManager": jinja2.StringValue(string(ctx.PackageManager)),
		"arch":           jinja2.StringValue(string(ctx.Arch)),
	}

	// Add all context variables
	for key, value := range ctx.variables {
		jinjaCtx[key] = value
	}

	// Create context objects for Starlark
	contextObj := starlarkpkg.NewContextObject(jinjaCtx)
	localObj := starlarkpkg.NewContextObject(jinjaCtx) // local is the same as context for now

	// Set the context and local objects in Starlark
	eval.SetGlobalStarlark("context", contextObj)
	eval.SetGlobalStarlark("local", localObj)

	var script string

	if s.Script != "" {
		// Use the script directly without Jinja2 template rendering
		script = string(s.Script)
	} else if s.File != "" {
		// Find and read the file
		var fullPath string
		for _, dir := range ctx.IncludeDirectories {
			fullPath = filepath.Join(dir, s.File)
			if _, err := os.Stat(fullPath); err == nil {
				break
			}
			fullPath = ""
		}

		if fullPath == "" {
			return fmt.Errorf("starlark file %q not found in include directories", s.File)
		}

		scriptBytes, readErr := os.ReadFile(fullPath)
		if readErr != nil {
			return fmt.Errorf("reading starlark file %q: %w", fullPath, readErr)
		}
		script = string(scriptBytes)
	}

	// Execute the Starlark script
	_, execErr := eval.ExecString(script)
	if execErr != nil {
		return fmt.Errorf("executing starlark script: %w", execErr)
	}

	// Process any run commands that were set
	var runCommands []string
	var envVars map[string]string

	for key, value := range ctx.variables {
		if strings.HasPrefix(key, "_starlark_env_") {
			envKey := strings.TrimPrefix(key, "_starlark_env_")
			if envVars == nil {
				envVars = make(map[string]string)
			}
			if envVal, ok := value.(jinja2.StringValue); ok {
				envVars[envKey] = string(envVal)
			}
		}
	}

	// Also include any commands accumulated via ctx.AddRunCommand
	if len(ctx.runCommands) > 0 {
		runCommands = append(runCommands, ctx.runCommands...)
	}

	// Apply run commands
	if len(runCommands) > 0 {
		for _, cmd := range runCommands {
			ctx.builder = ctx.builder.AddRunCommand(cmd)
		}
	}

	// Apply environment variables
	if len(envVars) > 0 {
		ctx.builder = ctx.builder.AddEnvironment(envVars)
	}

	// Clean up temporary variables
	for key := range ctx.variables {
		if strings.HasPrefix(key, "_starlark_env_") {
			delete(ctx.variables, key)
		}
	}
	// Clear consumed run commands
	ctx.runCommands = nil

	return nil
}

type Directive struct {
	Group       *GroupDirective       `yaml:"group,omitempty"`
	Run         *RunDirective         `yaml:"run,omitempty"`
	File        *FileDirective        `yaml:"file,omitempty"`
	Install     *InstallDirective     `yaml:"install,omitempty"`
	Environment *EnvironmentDirective `yaml:"environment,omitempty"`
	User        *UserDirective        `yaml:"user,omitempty"`
	WorkDir     *WorkDirDirective     `yaml:"workdir,omitempty"`
	Deploy      *DeployDirective      `yaml:"deploy,omitempty"`
	EntryPoint  *EntryPointDirective  `yaml:"entrypoint,omitempty"`
	Test        *TestDirective        `yaml:"test,omitempty"`
	Template    *TemplateDirective    `yaml:"template,omitempty"`
	Include     *IncludeDirective     `yaml:"include,omitempty"`
	Copy        *CopyDirective        `yaml:"copy,omitempty"`
	Variables   *VariablesDirective   `yaml:"variables,omitempty"`
	Boutique    *BoutiqueDirective    `yaml:"boutique,omitempty"`
	Starlark    *StarlarkDirective    `yaml:"starlark,omitempty"`

	// Optional condition for this directive to be applied.
	Condition string `yaml:"condition,omitempty"`

	// Variables for the group.
	With map[string]any `yaml:"with,omitempty"`

	Custom       string         `yaml:"custom,omitempty"`
	CustomParams map[string]any `yaml:"customParams,omitempty"`
}

func (d Directive) Validate(ctx Context) error {
	if d.Group != nil {
		return d.Group.Validate(ctx)
	} else if d.Run != nil {
		return d.Run.Validate()
	} else if d.File != nil {
		return d.File.Validate()
	} else if d.Install != nil {
		val := any(*d.Install)
		switch val := val.(type) {
		case string:
			return jinja2.TemplateString(val).Validate()
		case []any:
			return v.Map(val, func(item any, description string) error {
				s, ok := item.(string)
				if !ok {
					return fmt.Errorf("%s must be a string, got %T", description, item)
				}
				return jinja2.TemplateString(s).Validate()
			}, "install")
		default:
			return fmt.Errorf("install must be a string or list of strings, got %T", val)
		}
	} else if d.Environment != nil {
		return d.Environment.Validate()
	} else if d.User != nil {
		return d.User.Validate()
	} else if d.WorkDir != nil {
		return d.WorkDir.Validate()
	} else if d.Deploy != nil {
		return d.Deploy.Validate()
	} else if d.EntryPoint != nil {
		return d.EntryPoint.Validate()
	} else if d.Test != nil {
		return d.Test.Validate()
	} else if d.Template != nil {
		return d.Template.Validate(ctx)
	} else if d.Include != nil {
		return d.Include.Validate()
	} else if d.Copy != nil {
		val := any(*d.Copy)
		switch val := val.(type) {
		case string:
			return jinja2.TemplateString(val).Validate()
		case []any:
			return v.Map(val, func(item any, description string) error {
				s, ok := item.(string)
				if !ok {
					return fmt.Errorf("%s must be a string, got %T", description, item)
				}
				return jinja2.TemplateString(s).Validate()
			}, "copy")
		default:
			return fmt.Errorf("copy must be a string or list of strings")
		}
	} else if d.Variables != nil {
		return d.Variables.Validate()
	} else if d.Boutique != nil {
		return d.Boutique.Validate()
	} else if d.Starlark != nil {
		return d.Starlark.Validate(ctx)
	}
	return fmt.Errorf("directive must have exactly one action")
}

func (d Directive) Apply(ctx *Context) error {
	if d.Group != nil {
		return d.Group.Apply(ctx, d.With)
	} else if d.Run != nil {
		return d.Run.Apply(ctx)
	} else if d.File != nil {
		return d.File.Apply(ctx)
	} else if d.Install != nil {
		install := any(*d.Install)

		evaluateAndSplit := func(s string) ([]string, error) {
			tpl := jinja2.TemplateString(s)

			result, err := ctx.evaluateValue(tpl)
			if err != nil {
				return nil, fmt.Errorf("evaluating install command: %w", err)
			}

			s, ok := result.(string)
			if !ok {
				return nil, fmt.Errorf("install command must be a string, got %T", result)
			}

			return shellWords(s)
		}

		switch install := install.(type) {
		case string:
			pkgs, err := evaluateAndSplit(install)
			if err != nil {
				return fmt.Errorf("installing packages: %w", err)
			}

			return ctx.installPackages(pkgs...)
		case []any:
			var pkgs []string
			for i, item := range install {
				s, ok := item.(string)
				if !ok {
					return fmt.Errorf("install[%d] must be a string, got %T", i, item)
				}
				sp, err := evaluateAndSplit(s)
				if err != nil {
					return fmt.Errorf("installing packages: %w", err)
				}
				pkgs = append(pkgs, sp...)
			}
			return ctx.installPackages(pkgs...)
		default:
			return fmt.Errorf("install must be a string or list of strings, got %T", install)
		}
	} else if d.Environment != nil {
		return d.Environment.Apply(ctx)
	} else if d.User != nil {
		return d.User.Apply(ctx)
	} else if d.WorkDir != nil {
		return d.WorkDir.Apply(ctx)
	} else if d.Deploy != nil {
		return d.Deploy.Apply(ctx)
	} else if d.EntryPoint != nil {
		return d.EntryPoint.Apply(ctx)
	} else if d.Test != nil {
		return d.Test.Apply(ctx)
	} else if d.Template != nil {
		return d.Template.Apply(ctx)
	} else if d.Include != nil {
		return d.Include.Apply(ctx)
	} else if d.Copy != nil {
		// string or list (accept []string or []any)
		copy := any(*d.Copy)
		switch copy := copy.(type) {
		case string:
			// render the template string then split it into an array of strings separated by spaces
			tpl := jinja2.TemplateString(copy)
			result, err := ctx.evaluateValue(tpl)
			if err != nil {
				return fmt.Errorf("evaluating copy command: %w", err)
			}
			s, ok := result.(string)
			if !ok {
				return fmt.Errorf("copy command must be a string, got %T", result)
			}
			parts, err := shellWords(s)
			if err != nil {
				return fmt.Errorf("parsing copy: %w", err)
			}
			if len(parts) != 2 {
				return fmt.Errorf("copy command must have exactly two parts: source and destination")
			}
			ctx.builder = ctx.builder.AddCopy(parts...)
			return nil
		case []string:
			var parts []string
			for i, item := range copy {
				tpl := jinja2.TemplateString(item)
				result, err := ctx.evaluateValue(tpl)
				if err != nil {
					return fmt.Errorf("evaluating copy[%d] command: %w", i, err)
				}
				s, ok := result.(string)
				if !ok {
					return fmt.Errorf("copy[%d] command must be a string, got %T", i, result)
				}
				parts = append(parts, s)
			}
			ctx.builder = ctx.builder.AddCopy(parts...)
			return nil
		case []any:
			var parts []string
			for i, item := range copy {
				s, ok := item.(string)
				if !ok {
					return fmt.Errorf("copy[%d] command must be a string, got %T", i, item)
				}
				tpl := jinja2.TemplateString(s)
				result, err := ctx.evaluateValue(tpl)
				if err != nil {
					return fmt.Errorf("evaluating copy[%d] command: %w", i, err)
				}
				str, ok := result.(string)
				if !ok {
					return fmt.Errorf("copy[%d] command must be a string, got %T", i, result)
				}
				parts = append(parts, str)
			}
			ctx.builder = ctx.builder.AddCopy(parts...)
			return nil
		default:
			return fmt.Errorf("copy command must be a string or list of strings, got %T", copy)
		}
	} else if d.Variables != nil {
		return d.Variables.Apply(ctx)
	} else if d.Boutique != nil {
		return d.Boutique.Apply(ctx)
	} else if d.Starlark != nil {
		return d.Starlark.Apply(ctx)
	} else {
		return fmt.Errorf("directive not implemented")
	}
}

type BuildRecipe struct {
	Kind BuildKind `yaml:"kind"`

	BaseImage      string                `yaml:"base-image"`
	PackageManager common.PackageManager `yaml:"pkg-manager,omitempty"`

	Directives []Directive `yaml:"directives,omitempty"`

	AddDefaultTemplate *bool `yaml:"add-default-template,omitempty"`
	AddTzdata          *bool `yaml:"add-tzdata,omitempty"`
	FixLocaleDef       *bool `yaml:"fix-locale-def,omitempty"`
}

func (b BuildRecipe) Validate(ctx Context) error {
	return v.All(
		v.MatchesAllowed(b.Kind, []BuildKind{BuildKindNeuroDocker}, "build.kind"),
		v.NotEmpty(b.BaseImage, "build.base-image"),
		v.MatchesAllowed(b.PackageManager, []common.PackageManager{
			common.PkgManagerApt,
			common.PkgManagerYum,
		}, "build.pkg-manager"),
		v.Map(b.Directives, func(directive Directive, description string) error {
			return directive.Validate(ctx)
		}, "build.directives"),
	)
}

func (b *BuildRecipe) Generate(ctx *Context) error {
	if b.Kind != BuildKindNeuroDocker {
		return fmt.Errorf("unsupported build kind: %s", b.Kind)
	}

	baseImg, err := ctx.evaluateValue(b.BaseImage)
	if err != nil {
		return fmt.Errorf("evaluating base image: %w", err)
	}
	s, ok := baseImg.(string)
	if !ok {
		return fmt.Errorf("base image must be a string, got %T", baseImg)
	}

	ctx.builder = ctx.builder.AddFromImage(s)

	// Always set the user to root initially to ensure we can install packages
	ctx.builder = ctx.builder.SetCurrentUser("root")

	if b.AddDefaultTemplate == nil || *b.AddDefaultTemplate {
		tpl, err := templates.Get("_header")
		if err != nil {
			return fmt.Errorf("loading default header template: %w", err)
		}

		result, err := tpl.Execute(templates.Context{
			PackageManager: ctx.PackageManager,
		}, func(k string) (any, bool, error) {
			if k == "method" {
				return "source", true, nil
			}
			return nil, false, nil
		})
		if err != nil {
			return fmt.Errorf("executing default header template: %w", err)
		}

		if len(result.Environment) > 0 {
			ctx.builder = ctx.builder.AddEnvironment(result.Environment)
		}

		ctx.builder = ctx.builder.AddRunCommand(result.Instructions)
	}

	if err := (GroupDirective{
		Directive{Run: &RunDirective{
			"printf '#!/bin/bash\\nls -la' > /usr/bin/ll",
			"chmod +x /usr/bin/ll",
			jinja2.TemplateString(fmt.Sprintf("mkdir -p %s", strings.Join(GLOBAL_MOUNT_POINT_LIST, " "))),
		}},
	}).Apply(ctx, nil); err != nil {
		return fmt.Errorf("adding default environment variables: %w", err)
	}

	if (b.AddTzdata == nil || *b.AddTzdata) && ctx.PackageManager == common.PkgManagerApt {
		install := InstallDirective("tzdata")
		if err := (GroupDirective{
			Directive{Environment: &EnvironmentDirective{
				"DEBIAN_FRONTEND": "noninteractive",
				"TZ":              "UTC",
			}},
			Directive{Install: &install},
			Directive{Run: &RunDirective{"ln -snf /usr/share/zoneinfo/UTC /etc/localtime && echo UTC > /etc/timezone"}},
		}).Apply(ctx, nil); err != nil {
			return fmt.Errorf("adding tzdata: %w", err)
		}
	}

	for _, directive := range b.Directives {
		if err := directive.Apply(ctx); err != nil {
			return fmt.Errorf("applying directive: %w", err)
		}
	}

	if len(ctx.deployBins) > 0 {
		path := strings.Join(ctx.deployBins, ":")
		ctx.builder = ctx.builder.AddEnvironment(map[string]string{
			"DEPLOY_BINS": path,
		})
	}

	if len(ctx.deployPath) > 0 {
		path := strings.Join(ctx.deployPath, ":")
		ctx.builder = ctx.builder.AddEnvironment(map[string]string{
			"DEPLOY_PATH": path,
		})
	}

	// TODO(joshua): handle README.md file.

    if b.FixLocaleDef != nil && *b.FixLocaleDef {
        // No-op for now: older recipes may set this flag. Left intentionally
        // blank to avoid failing generation.
    }

	return nil
}

type IncludeFile struct {
	Builder    BuildKind   `yaml:"builder"`
	Directives []Directive `yaml:"directives,omitempty"`
}

type BuildFile struct {
	Name          string                `yaml:"name"`
	Version       string                `yaml:"version"`
	Epoch         int                   `yaml:"epoch,omitempty"`
	Architectures []CPUArchitecture     `yaml:"architectures"`
	Options       map[string]OptionInfo `yaml:"options,omitempty"`

	Build BuildRecipe `yaml:"build"`

	Copyright        []Copyright           `yaml:"copyright,omitempty"`
	StructuredReadme StructuredReadme      `yaml:"structured_readme,omitempty"`
	Readme           jinja2.TemplateString `yaml:"readme,omitempty"`
	ReadmeUrl        string                `yaml:"readme_url,omitempty"`
	// List of categories.
	Categories []Category `yaml:"categories,omitempty"`
	// Application Icon in base64-encoded PNG format.
	Icon    string   `yaml:"icon,omitempty"`
	GuiApps []GuiApp `yaml:"gui_apps,omitempty"`

	// Deprecated (still supported for backward compatibility)
	Draft     bool           `yaml:"draft,omitempty"`
	Variables map[string]any `yaml:"variables,omitempty"`
	Deploy    DeployInfo     `yaml:"deploy,omitempty"`
	Files     []FileInfo     `yaml:"files,omitempty"`

	// Forward-compat: allow apptainer_args in recipes but ignore for now.
	ApptainerArgs any `yaml:"apptainer_args,omitempty"`
}

func (b *BuildFile) Validate(ctx Context) error {
	return v.All(
		v.NotEmpty(b.Name, "name"),
		v.NotEmpty(b.Version, "version"),
		v.SliceHasElements(b.Architectures, []CPUArchitecture{CPUArchAMD64, CPUArchARM64}, "architectures"),
		b.Build.Validate(ctx),
		b.Readme.Validate(),
		// Validate top-level files and variables if present
		v.Map(b.Files, func(fi FileInfo, description string) error {
			return FileDirective(fi).Validate()
		}, "files"),
	)
}

// StagedFile describes a file to materialize in the build context.
type StagedFile struct {
	Name       string
	Executable bool
	// Exactly one source
	HostFilename string
	URL          string
	Contents     string
}

type StagingPlan struct {
	Files []StagedFile
}

func (b *BuildFile) Generate(includeDirs []string) (*ir.Definition, error) {
	def, _, err := b.GenerateWithStaging(includeDirs)
	if err != nil {
		return nil, err
	}
	return def, nil
}

// GenerateWithStaging builds the IR and returns a staging plan for files.
func (b *BuildFile) GenerateWithStaging(includeDirs []string) (*ir.Definition, *StagingPlan, error) {
	ctx := newContext(
		b.Build.PackageManager,
		b.Version,
		includeDirs,
		ir.New(),
		nil,
	)

	// Default architecture: first declared, or x86_64 if unspecified
	if len(b.Architectures) > 0 {
		ctx.Arch = b.Architectures[0]
	}

	// Apply top-level variables early so they are available to directives
	if len(b.Variables) > 0 {
		vars := VariablesDirective(b.Variables)
		if err := vars.Apply(ctx); err != nil {
			return nil, nil, fmt.Errorf("applying top-level variables: %w", err)
		}
	}

	// Register top-level files into the context (for get_file())
	for _, f := range b.Files {
		if err := FileDirective(f).Apply(ctx); err != nil {
			return nil, nil, fmt.Errorf("adding top-level file %q: %w", f.Name, err)
		}
	}

	if err := b.Build.Generate(ctx); err != nil {
		return nil, nil, fmt.Errorf("generating build: %w", err)
	}

	def, err := ctx.Compile()
	if err != nil {
		return nil, nil, err
	}

	// Collect staging files from ctx.files
	plan := &StagingPlan{}
	for name, f := range ctx.files {
		switch t := f.(type) {
		case contextFile:
			plan.Files = append(plan.Files, StagedFile{Name: name, Executable: t.Executable, HostFilename: t.HostFilename})
		case httpFile:
			plan.Files = append(plan.Files, StagedFile{Name: name, Executable: t.Executable, URL: t.URL})
		case literalFile:
			plan.Files = append(plan.Files, StagedFile{Name: name, Executable: t.Executable, Contents: t.Contents})
		}
	}
	// Sort plan for determinism
	sort.Slice(plan.Files, func(i, j int) bool { return plan.Files[i].Name < plan.Files[j].Name })

	return def, plan, nil
}

func LoadBuildFile(path string) (*BuildFile, error) {
	buildYaml := filepath.Join(path, "build.yaml")

	f, err := os.Open(buildYaml)
	if err != nil {
		return nil, err
	}

	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)

	var build BuildFile
	if err := dec.Decode(&build); err != nil {
		return nil, err
	}

	if err := build.Validate(Context{}); err != nil {
		return nil, fmt.Errorf("validating build file %q: %w", path, err)
	}

	return &build, nil
}
