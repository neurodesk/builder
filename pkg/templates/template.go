package templates

import (
	"embed"
	"fmt"
	"log/slog"
	"strings"

	"github.com/neurodesk/builder/pkg/jinja2"
	v "github.com/neurodesk/builder/pkg/validator"

	"go.yaml.in/yaml/v4"
)

type Params map[string]any

func (p Params) GetString(key string, defValue string) (string, error) {
	if val, ok := p[key]; ok {
		if strVal, ok := val.(string); ok {
			return strVal, nil
		}
		return "", fmt.Errorf("parameter %q is not a string", key)
	}
	return defValue, nil
}

type templateSelf struct {
}

// OnLookup implements jinja2.LookupHook.
func (t *templateSelf) OnLookup(key string) (jinja2.Value, bool) {
	switch key {
	default:
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

type TemplateResult struct {
	Instructions string
}

type Depends struct {
	Apt  []string `yaml:"apt,omitempty"`
	Yum  []string `yaml:"yum,omitempty"`
	Debs []string `yaml:"debs,omitempty"`
}

func (d *Depends) Validate() error {
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

type Arguments struct {
	Optional map[string]jinja2.TemplateString `yaml:"optional,omitempty"`
	Required []string                         `yaml:"required,omitempty"`
}

func (t *Arguments) Validate() error {
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

type RecipeTemplate struct {
	Arguments    Arguments                        `yaml:"arguments,omitempty"`
	Dependencies Depends                          `yaml:"dependencies,omitempty"`
	Urls         map[string]jinja2.TemplateString `yaml:"urls,omitempty"`
	Env          map[string]jinja2.TemplateString `yaml:"env,omitempty"`
	Instructions jinja2.TemplateString            `yaml:"instructions,omitempty"`
}

func (t *RecipeTemplate) Validate() error {
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

func (t *RecipeTemplate) Execute(params Params) (*TemplateResult, error) {
	ctx := jinja2.Context{
		"self": &templateSelf{},
	}

	result, err := t.Instructions.Render(ctx)
	if err != nil {
		return nil, fmt.Errorf("rendering instructions: %w", err)
	}

	fmt.Printf("Rendered instructions:\n%s\n", result)

	return &TemplateResult{
		Instructions: result,
	}, nil
}

type Template struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`

	Alert string `yaml:"alert,omitempty"`

	Source   *RecipeTemplate `yaml:"source,omitempty"`
	Binaries *RecipeTemplate `yaml:"binaries,omitempty"`
}

func (t Template) GetMethodTemplate(method string) (*RecipeTemplate, error) {
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

func (t Template) Validate() error {
	return v.All(
		v.NotEmpty(t.Name, "name"),
		v.NotEmpty(t.URL, "url"),
		t.Source.Validate(),
		t.Binaries.Validate(),
	)
}

func (t Template) Execute(params Params) (*TemplateResult, error) {
	method, err := params.GetString("method", "binaries")
	if err != nil {
		return nil, fmt.Errorf("getting method parameter: %w", err)
	}

	tpl, err := t.GetMethodTemplate(method)
	if err != nil {
		return nil, fmt.Errorf("getting method template: %w", err)
	}

	return tpl.Execute(params)
}

//go:embed *.yaml
var Files embed.FS

var templates = map[string]Template{}

func Get(name string) (Template, error) {
	if tpl, ok := templates[name]; ok {
		return tpl, nil
	}
	return Template{}, fmt.Errorf("template %q not found", name)
}

func init() {
	entries, err := Files.ReadDir(".")
	if err != nil {
		panic(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		content, err := Files.ReadFile(name)
		if err != nil {
			panic(err)
		}
		var tpl Template
		dec := yaml.NewDecoder(strings.NewReader(string(content)))
		dec.KnownFields(true)
		if err := dec.Decode(&tpl); err != nil {
			panic(fmt.Errorf("failed to decode template %q: %w", name, err))
		}
		if err := tpl.Validate(); err != nil {
			panic(fmt.Errorf("invalid template %q: %w", name, err))
		}
		templates[strings.TrimSuffix(name, ".yaml")] = tpl
	}
}
