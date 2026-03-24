package recipe

import (
	"embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/neurodesk/builder/pkg/common"
	"github.com/neurodesk/builder/pkg/jinja2"
	v "github.com/neurodesk/builder/pkg/validator"

	"go.yaml.in/yaml/v4"
)

type templateContext struct {
	PackageManager common.PackageManager
	Arch           string
}

type templateParams func(k string) (any, bool, error)

func (p templateParams) GetString(key string, defValue string) (string, error) {
	if val, ok, err := p(key); ok {
		if strVal, ok := val.(string); ok {
			return strVal, nil
		}
		return "", fmt.Errorf("parameter %q is not a string", key)
	} else if err != nil {
		return "", fmt.Errorf("getting parameter %q: %w", key, err)
	}
	return defValue, nil
}

type templateSelf struct {
	context  templateContext
	params   templateParams
	template *recipeTemplateSpec
}

func (t *templateSelf) install(mgr common.PackageManager, args []string) (string, error) {
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

func (t *templateSelf) getArgument(key string) (jinja2.Value, bool, error) {
	// if the argument exists in params, return it
	if val, ok, err := t.params(key); ok {
		return jinja2.FromGo(val), true, nil
	} else if err != nil {
		return nil, false, fmt.Errorf("getting parameter %q: %w", key, err)
	}

	// if the argument has a default value, return it
	defVal, ok := t.template.Arguments.Optional[key]
	if ok {
		// otherwise, render the default value template
		val, err := defVal.Render(jinja2.Context{
			"self": t,
		})
		if err != nil {
			return nil, false, fmt.Errorf("rendering optional argument %q: %w", key, err)
		}
		return jinja2.StringValue(val), true, nil
	}

	// argument not found
	return nil, false, nil
}

// OnLookup implements jinja2.LookupHook.
func (t *templateSelf) OnLookup(key string) (jinja2.Value, bool) {
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
					// For debs, we only support apt for now
					for _, deb := range t.template.Dependencies.Debs {
						installs = append(installs, fmt.Sprintf("dpkg -i %s || apt-get -f install -y", deb))
					}
				}

				return jinja2.StringValue(strings.Join(installs, " && ")), nil
			},
		}, true
	case "urls":
		ret := jinja2.DictValue{}
		for key, tpl := range t.template.Urls {
			val, err := tpl.Render(jinja2.Context{
				"self": t,
			})
			if err != nil {
				slog.Error("error rendering url template", "key", key, "error", err)
				continue
			}
			ret[key] = jinja2.StringValue(val)
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
		for key, tpl := range t.template.Env {
			val, err := tpl.Render(jinja2.Context{
				"self": t,
			})
			if err != nil {
				slog.Error("error rendering env template", "key", key, "error", err)
				continue
			}
			ret[key] = jinja2.StringValue(val)
		}
		return ret, true
	default:
		arg, ok, err := t.getArgument(key)
		if err != nil {
			slog.Error("error getting self attribute", "key", key, "error", err)
			return nil, false
		}
		if ok {
			return arg, true
		}

		slog.Warn("unknown self attribute", "key", key)
		return nil, false
	}
}

// String implements jinja2.Value.
func (t *templateSelf) String() string { return "<self>" }

// Truth implements jinja2.Value.
func (t *templateSelf) Truth() bool {
	return true
}

var (
	_ jinja2.Value      = &templateSelf{}
	_ jinja2.LookupHook = &templateSelf{}
)

type templateResult struct {
	Instructions string
	Environment  map[string]string
}

type templateDepends struct {
	Apt  []string `yaml:"apt,omitempty"`
	Yum  []string `yaml:"yum,omitempty"`
	Debs []string `yaml:"debs,omitempty"`
}

func (d *templateDepends) Validate() error {
	return v.All(
		v.Map(d.Apt, func(item string, key string) error {
			return v.All(
				v.NotEmpty(item, fmt.Sprintf("apt dependency %q", key)),
				v.HasNoJinja(item, fmt.Sprintf("apt dependency %q", key)),
			)
		}, "apt dependencies"),
		v.Map(d.Yum, func(item string, key string) error {
			return v.All(
				v.NotEmpty(item, fmt.Sprintf("yum dependency %q", key)),
				v.HasNoJinja(item, fmt.Sprintf("yum dependency %q", key)),
			)
		}, "yum dependencies"),
		v.Map(d.Debs, func(item string, key string) error {
			return v.All(
				v.NotEmpty(item, fmt.Sprintf("debs dependency %q", key)),
				v.HasNoJinja(item, fmt.Sprintf("debs dependency %q", key)),
			)
		}, "debs dependencies"),
	)
}

type templateArguments struct {
	Optional map[string]jinja2.TemplateString `yaml:"optional,omitempty"`
	Required []string                         `yaml:"required,omitempty"`
}

func (t *templateArguments) Validate() error {
	return v.All(
		v.MapDict(t.Optional, func(key string, value jinja2.TemplateString) error {
			return v.All(
				v.NotEmpty(key, "argument key"),
				v.HasNoJinja(key, "argument key"),
				value.Validate(),
			)
		}, "optional arguments"),
		v.NoDuplicates(t.Required, "required arguments"),
	)
}

type recipeTemplateSpec struct {
	Arguments    templateArguments                `yaml:"arguments,omitempty"`
	Dependencies templateDepends                  `yaml:"dependencies,omitempty"`
	Urls         map[string]jinja2.TemplateString `yaml:"urls,omitempty"`
	Env          map[string]jinja2.TemplateString `yaml:"env,omitempty"`
	Instructions jinja2.TemplateString            `yaml:"instructions,omitempty"`
}

func (t *recipeTemplateSpec) Validate() error {
	if t == nil {
		return nil
	}
	return v.All(
		t.Arguments.Validate(),
		t.Dependencies.Validate(),
		v.MapDict(t.Urls, func(key string, value jinja2.TemplateString) error {
			return v.All(
				v.NotEmpty(key, "url key"),
				v.HasNoJinja(key, "url key"),
				value.Validate(),
			)
		}, "urls"),
		v.MapDict(t.Env, func(key string, value jinja2.TemplateString) error {
			return v.All(
				v.NotEmpty(key, "env key"),
				v.HasNoJinja(key, "env key"),
				value.Validate(),
			)
		}, "env"),
		t.Instructions.Validate(),
	)
}

func (t *recipeTemplateSpec) Execute(name string, context templateContext, params templateParams) (*templateResult, error) {
	key := "self"

	if name == "_header" {
		key = "_header"
	}

	ctx := jinja2.Context{
		key: &templateSelf{
			context:  context,
			params:   params,
			template: t,
		},
	}

	result, err := t.Instructions.Render(ctx)
	if err != nil {
		return nil, fmt.Errorf("rendering instructions: %w", err)
	}

	ret := &templateResult{
		Instructions: result,
		Environment:  map[string]string{},
	}

	for key, value := range t.Env {
		val, err := value.Render(ctx)
		if err != nil {
			return nil, fmt.Errorf("rendering env %q: %w", key, err)
		}
		ret.Environment[key] = val
	}

	return ret, nil
}

type templateSpec struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`

	Alert string `yaml:"alert,omitempty"`

	Source   *recipeTemplateSpec `yaml:"source,omitempty"`
	Binaries *recipeTemplateSpec `yaml:"binaries,omitempty"`
}

func (t templateSpec) GetMethodTemplate(method string) (*recipeTemplateSpec, error) {
	switch method {
	case "source":
		if t.Source == nil {
			return nil, fmt.Errorf("template %q does not support method 'source'", t.Name)
		}
		return t.Source, nil
	case "binaries":
		if t.Binaries == nil {
			return nil, fmt.Errorf("template %q does not support method 'binaries'", t.Name)
		}
		return t.Binaries, nil
	default:
		return nil, fmt.Errorf("unknown method %q", method)
	}
}

func (t templateSpec) Validate() error {
	return v.All(
		v.NotEmpty(t.Name, "name"),
		v.NotEmpty(t.URL, "url"),
		t.Source.Validate(),
		t.Binaries.Validate(),
	)
}

func (t templateSpec) Execute(ctx templateContext, params templateParams) (*templateResult, error) {
	method, err := params.GetString("method", "binaries")
	if err != nil {
		return nil, fmt.Errorf("getting method parameter: %w", err)
	}

	tpl, err := t.GetMethodTemplate(method)
	if err != nil {
		return nil, fmt.Errorf("getting method template: %w", err)
	}

	return tpl.Execute(t.Name, ctx, params)
}

//go:embed template_specs/*.yaml
var templateSpecFiles embed.FS

var embeddedTemplateSpecs = map[string]templateSpec{}
var templateSpecDir string

func loadTemplateSpecFromDir(name, dir string) (templateSpec, error) {
	templatePath := filepath.Join(dir, name+".yaml")

	content, err := os.ReadFile(templatePath)
	if err != nil {
		return templateSpec{}, err
	}

	var tpl templateSpec
	dec := yaml.NewDecoder(strings.NewReader(string(content)))
	dec.KnownFields(true)
	if err := dec.Decode(&tpl); err != nil {
		return templateSpec{}, fmt.Errorf("failed to decode template %q: %w", name, err)
	}

	if err := tpl.Validate(); err != nil {
		return templateSpec{}, fmt.Errorf("invalid template %q: %w", name, err)
	}

	return tpl, nil
}

func SetTemplateSpecDir(dir string) {
	templateSpecDir = dir
}

func getTemplateSpec(name string) (templateSpec, error) {
	if templateSpecDir != "" {
		if tpl, err := loadTemplateSpecFromDir(name, templateSpecDir); err == nil {
			return tpl, nil
		}
	}

	if tpl, ok := embeddedTemplateSpecs[name]; ok {
		return tpl, nil
	}
	return templateSpec{}, fmt.Errorf("template %q not found", name)
}

func ReadEmbeddedTemplateTestSpecs() ([]byte, error) {
	return templateSpecFiles.ReadFile(filepath.Join("template_specs", "test_all.yaml"))
}

func init() {
	entries, err := templateSpecFiles.ReadDir("template_specs")
	if err != nil {
		panic(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == "test_all.yaml" {
			continue
		}
		content, err := templateSpecFiles.ReadFile(filepath.Join("template_specs", name))
		if err != nil {
			panic(err)
		}
		var tpl templateSpec
		dec := yaml.NewDecoder(strings.NewReader(string(content)))
		dec.KnownFields(true)
		if err := dec.Decode(&tpl); err != nil {
			panic(fmt.Errorf("failed to decode template %q: %w", name, err))
		}
		if err := tpl.Validate(); err != nil {
			panic(fmt.Errorf("invalid template %q: %w", name, err))
		}
		embeddedTemplateSpecs[strings.TrimSuffix(name, ".yaml")] = tpl
	}
}
