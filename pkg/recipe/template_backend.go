package recipe

import (
	"embed"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/neurodesk/builder/pkg/common"
	"github.com/neurodesk/builder/pkg/ir"
	"github.com/neurodesk/builder/pkg/jinja2"
	"go.yaml.in/yaml/v4"
)

type TemplateBackend string

const (
	TemplateBackendMacro TemplateBackend = "macro"
)

var templateBackend = TemplateBackendMacro

func SetTemplateBackend(backend string) error {
	switch TemplateBackend(backend) {
	case "", TemplateBackendMacro:
		templateBackend = TemplateBackendMacro
		return nil
	default:
		return fmt.Errorf("unknown template backend %q", backend)
	}
}

func currentTemplateBackend() TemplateBackend {
	return templateBackend
}

type templateMacroFile struct {
	Builder    BuildKind   `yaml:"builder"`
	Directives []Directive `yaml:"directives,omitempty"`
}

//go:embed template_macros/*.yaml
var macroTemplateFiles embed.FS

var templateMacros = map[string]templateMacroFile{}

func init() {
	entries, err := macroTemplateFiles.ReadDir("template_macros")
	if err != nil {
		panic(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		content, err := macroTemplateFiles.ReadFile(filepath.Join("template_macros", name))
		if err != nil {
			panic(err)
		}

		var macro templateMacroFile
		dec := yaml.NewDecoder(strings.NewReader(string(content)))
		dec.KnownFields(true)
		if err := dec.Decode(&macro); err != nil {
			panic(fmt.Errorf("decoding macro template %q: %w", name, err))
		}
		if macro.Builder != BuildKindNeuroDocker {
			panic(fmt.Errorf("macro template %q uses unsupported builder %q", name, macro.Builder))
		}
		templateMacros[strings.TrimSuffix(name, ".yaml")] = macro
	}
}

func loadTemplateMacro(name, method string) (templateMacroFile, error) {
	key := name + "__" + method
	macro, ok := templateMacros[key]
	if !ok {
		return templateMacroFile{}, fmt.Errorf("macro template %q not found", key)
	}
	return macro, nil
}

type macroTemplateSelf struct {
	context  templateContext
	params   templateParams
	template *recipeTemplateSpec
}

func (t *macroTemplateSelf) install(mgr common.PackageManager, args []string) (string, error) {
	switch mgr {
	case common.PkgManagerApt:
		if len(args) == 0 {
			return "", fmt.Errorf("no packages specified for apt")
		}
		return fmt.Sprintf(
			"apt-get -o Acquire::Retries=3 update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends %s",
			strings.Join(args, " "),
		), nil
	case common.PkgManagerYum:
		if len(args) == 0 {
			return "", fmt.Errorf("no packages specified for yum")
		}
		return fmt.Sprintf("yum install -y %s", strings.Join(args, " ")), nil
	default:
		return "", fmt.Errorf("unknown package manager: %s", mgr)
	}
}

func (t *macroTemplateSelf) getArgument(key string) (jinja2.Value, bool, error) {
	if val, ok, err := t.params(key); ok {
		return jinja2.FromGo(val), true, nil
	} else if err != nil {
		return nil, false, fmt.Errorf("getting parameter %q: %w", key, err)
	}

	defVal, ok := t.template.Arguments.Optional[key]
	if ok {
		val, err := defVal.Render(jinja2.Context{
			"self": t,
		})
		if err != nil {
			return nil, false, fmt.Errorf("rendering optional argument %q: %w", key, err)
		}
		return jinja2.StringValue(val), true, nil
	}

	return nil, false, nil
}

func (t *macroTemplateSelf) OnLookup(key string) (jinja2.Value, bool) {
	switch key {
	case "install_dependencies":
		return jinja2.CallableValue{
			Fn: func(args []jinja2.Value) (jinja2.Value, error) {
				if len(args) != 0 {
					return nil, fmt.Errorf("install_dependencies takes no arguments")
				}

				var installs []string
				if len(t.template.Dependencies.Apt) > 0 && t.context.PackageManager == common.PkgManagerApt {
					cmd, err := t.install(common.PkgManagerApt, t.template.Dependencies.Apt)
					if err != nil {
						return nil, err
					}
					installs = append(installs, cmd)
				}
				if len(t.template.Dependencies.Yum) > 0 && t.context.PackageManager == common.PkgManagerYum {
					cmd, err := t.install(common.PkgManagerYum, t.template.Dependencies.Yum)
					if err != nil {
						return nil, err
					}
					installs = append(installs, cmd)
				}
				if len(t.template.Dependencies.Debs) > 0 && t.context.PackageManager == common.PkgManagerApt {
					for _, deb := range t.template.Dependencies.Debs {
						installs = append(installs, fmt.Sprintf("dpkg -i %s || apt-get -f install -y", deb))
					}
				}
				return jinja2.StringValue(strings.Join(installs, " && ")), nil
			},
		}, true
	case "urls":
		ret := jinja2.DictValue{}
		for k, tpl := range t.template.Urls {
			val, err := tpl.Render(jinja2.Context{"self": t})
			if err != nil {
				continue
			}
			ret[k] = jinja2.StringValue(val)
		}
		return ret, true
	case "pkg_manager":
		return jinja2.StringValue(string(t.context.PackageManager)), true
	case "arch":
		return jinja2.StringValue(t.context.Arch), true
	case "install":
		return jinja2.CallableValue{
			Fn: func(args []jinja2.Value) (jinja2.Value, error) {
				if len(args) < 1 {
					return nil, fmt.Errorf("install requires at least one argument")
				}
				var pkgs []string
				for _, arg := range args {
					pkgs = append(pkgs, arg.String())
				}
				cmd, err := t.install(t.context.PackageManager, pkgs)
				if err != nil {
					return nil, err
				}
				return jinja2.StringValue(cmd), nil
			},
		}, true
	case "_env":
		ret := jinja2.DictValue{}
		for k, tpl := range t.template.Env {
			val, err := tpl.Render(jinja2.Context{"self": t})
			if err != nil {
				continue
			}
			ret[k] = jinja2.StringValue(val)
		}
		return ret, true
	default:
		arg, ok, err := t.getArgument(key)
		if err != nil {
			return nil, false
		}
		return arg, ok
	}
}

func (t *macroTemplateSelf) String() string { return "<self>" }

func (t *macroTemplateSelf) Truth() bool { return true }

func applyTemplateMacro(ctx *Context, src ir.SourceID, name string, params templateParams) error {
	templateSpec, err := getTemplateSpec(name)
	if err != nil {
		return fmt.Errorf("loading template metadata for %q: %w", name, err)
	}

	method, err := params.GetString("method", "binaries")
	if err != nil {
		return fmt.Errorf("getting method parameter: %w", err)
	}

	methodTemplate, err := templateSpec.GetMethodTemplate(method)
	if err != nil {
		return fmt.Errorf("getting method template: %w", err)
	}

	macro, err := loadTemplateMacro(name, method)
	if err != nil {
		return err
	}

	child := ctx.childContext()
	lookupKey := "self"
	if name == "_header" {
		lookupKey = "_header"
	}
	child.variables[lookupKey] = &macroTemplateSelf{
		context: templateContext{
			PackageManager: ctx.PackageManager,
			Arch:           string(ctx.Arch),
		},
		params:   params,
		template: methodTemplate,
	}

	for _, directive := range macro.Directives {
		if directive.Source == "" {
			directive.Source = src
		}
		if err := directive.Apply(child); err != nil {
			return fmt.Errorf("applying macro template %q: %w", name, err)
		}
	}

	delete(child.variables, lookupKey)

	ctx.builder = child.builder
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
	if len(child.runCommands) > 0 {
		ctx.runCommands = append(ctx.runCommands, child.runCommands...)
	}

	return nil
}
