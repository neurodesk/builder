package recipe

import (
	"fmt"

	"github.com/neurodesk/builder/pkg/jinja2"
	"github.com/neurodesk/builder/pkg/templates"
	v "github.com/neurodesk/builder/pkg/validator"
)

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

	Executable string      `yaml:"executable,omitempty"`
	Script     string      `yaml:"script,omitempty"`
	Builtin    TestBuiltin `yaml:"builtin,omitempty"`
}

type BuildKind string

const (
	BuildKindNeuroDocker BuildKind = "neurodocker"
)

type PackageManager string

const (
	PkgManagerApt PackageManager = "apt"
	PkgManagerYum PackageManager = "yum"
)

type GroupDirective []Directive

func (g GroupDirective) Validate() error {
	return v.Each(g)
}

type RunDirective []jinja2.TemplateString

func (r RunDirective) Validate() error {
	return v.Map(r, func(cmd jinja2.TemplateString, description string) error {
		return cmd.Validate()
	}, "run")
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

type InstallDirective any // string or []string

type UserDirective jinja2.TemplateString

func (u UserDirective) Validate() error {
	return jinja2.TemplateString(u).Validate()
}

type WorkDirDirective jinja2.TemplateString

func (w WorkDirDirective) Validate() error {
	return jinja2.TemplateString(w).Validate()
}

type EntryPointDirective jinja2.TemplateString

func (e EntryPointDirective) Validate() error {
	return jinja2.TemplateString(e).Validate()
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
	)
}

type TemplateDirective struct {
	Name   string           `yaml:"name"`
	Params templates.Params `yaml:",inline,omitempty"`
}

func (t TemplateDirective) Validate() error {
	if err := v.NotEmpty(t.Name, "template.name"); err != nil {
		return err
	}

	tpl, err := templates.Get(t.Name)
	if err != nil {
		return fmt.Errorf("template %q not found", t.Name)
	}

	result, err := tpl.Execute(t.Params)
	if err != nil {
		return fmt.Errorf("failed to render template %q: %w", t.Name, err)
	}
	_ = result

	return nil
}

type IncludeDirective string

func (i IncludeDirective) Validate() error {
	return v.HasNoJinja(string(i), "include")
}

type CopyDirective any // string or []string

type VariablesDirective map[string]any

func (v VariablesDirective) Validate() error {
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

	// Optional condition for this directive to be applied.
	Condition string `yaml:"condition,omitempty"`

	// Variables for the group.
	With map[string]any `yaml:"with,omitempty"`

	Custom       string         `yaml:"custom,omitempty"`
	CustomParams map[string]any `yaml:"customParams,omitempty"`
}

func (d Directive) Validate() error {
	if d.Group != nil {
		return d.Group.Validate()
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
		return d.Template.Validate()
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
	}
	return fmt.Errorf("directive must have exactly one action")
}

type BuildRecipe struct {
	Kind BuildKind `yaml:"kind"`

	BaseImage      string         `yaml:"base-image"`
	PackageManager PackageManager `yaml:"pkg-manager,omitempty"`

	Directives []Directive `yaml:"directives,omitempty"`

	AddDefaultTemplate *bool `yaml:"add-default-template,omitempty"`
	AddTzdata          *bool `yaml:"add-tzdata,omitempty"`
	FixLocaleDef       *bool `yaml:"fix-locale-def,omitempty"`
}

func (b BuildRecipe) Validate() error {
	return v.All(
		v.MatchesAllowed(b.Kind, []BuildKind{BuildKindNeuroDocker}, "build.kind"),
		v.NotEmpty(b.BaseImage, "build.base-image"),
		v.MatchesAllowed(b.PackageManager, []PackageManager{PkgManagerApt, PkgManagerYum}, "build.pkg-manager"),
		v.Each(b.Directives),
	)
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

	// Deprecated
	Draft     bool           `yaml:"draft,omitempty"`
	Variables map[string]any `yaml:"variables,omitempty"`
	Deploy    DeployInfo     `yaml:"deploy,omitempty"`
	Files     []FileInfo     `yaml:"files,omitempty"`
}

func (b *BuildFile) Validate() error {
	return v.All(
		v.NotEmpty(b.Name, "name"),
		v.NotEmpty(b.Version, "version"),
		v.SliceHasElements(b.Architectures, []CPUArchitecture{CPUArchAMD64, CPUArchARM64}, "architectures"),
		b.Build.Validate(),
		b.Readme.Validate(),
	)
}
