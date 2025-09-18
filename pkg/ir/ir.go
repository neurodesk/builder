package ir

import "fmt"

type Directive interface {
	isDirective()
}

type FromImageDirective string

// isDirective implements Directive.
func (f FromImageDirective) isDirective() {}

type EnvironmentDirective map[string]string

// isDirective implements Directive.
func (e EnvironmentDirective) isDirective() {}

type RunDirective string

// isDirective implements Directive.
func (r RunDirective) isDirective() {}

type CopyDirective struct {
	Parts []string
}

// isDirective implements Directive.
func (c CopyDirective) isDirective() {}

type WorkDirDirective string

// isDirective implements Directive.
func (w WorkDirDirective) isDirective() {}

type UserDirective string

// isDirective implements Directive.
func (u UserDirective) isDirective() {}

type EntryPointDirective string

// isDirective implements Directive.
func (e EntryPointDirective) isDirective() {}

type DeployBinsDirective []string

// isDirective implements Directive.
func (d DeployBinsDirective) isDirective() {}

type DeployPathsDirective []string

// isDirective implements Directive.
func (d DeployPathsDirective) isDirective() {}

type ScriptTestDirective struct {
	Name       string
	Manual     bool
	Executable string
	Script     string
}

// isDirective implements Directive.
func (s ScriptTestDirective) isDirective() {}

type BuiltinTestDirective struct {
	Name    string
	Manual  bool
	Builtin string
}

// isDirective implements Directive.
func (b BuiltinTestDirective) isDirective() {}

var (
	_ Directive = FromImageDirective("")

	_ Directive = EnvironmentDirective{}
	_ Directive = RunDirective("")
	_ Directive = WorkDirDirective("")
	_ Directive = UserDirective("")
	_ Directive = EntryPointDirective("")

	_ Directive = DeployBinsDirective{}
	_ Directive = DeployPathsDirective{}
	_ Directive = ScriptTestDirective{}
	_ Directive = BuiltinTestDirective{}
)

type Definition struct {
	Directives []Directive
}

type Builder interface {
	Compile() (*Definition, error)

	AddFromImage(image string) Builder

	AddEnvironment(env map[string]string) Builder
	AddRunCommand(cmd string) Builder
	AddCopy(parts ...string) Builder
	SetWorkingDirectory(dir string) Builder
	SetCurrentUser(user string) Builder
	SetEntryPoint(cmd string) Builder

	AddDeployBins(bins ...string) Builder
	AddDeployPaths(paths ...string) Builder
	AddScriptTest(
		name string,
		manual bool,
		executable string,
		script string,
	) Builder
	AddBuiltinTest(
		name string,
		manual bool,
		builtin string,
	) Builder
}

type builderImpl struct {
	out *Definition
}

func (b *builderImpl) String() string {
	return fmt.Sprintf("%#v", b.out)
}

func (b *builderImpl) add(d Directive) *builderImpl {
	ret := *b
	ret.out = &Definition{
		Directives: append(append([]Directive{}, b.out.Directives...), d),
	}
	return &ret
}

func (b *builderImpl) AddFromImage(image string) Builder {
	return b.add(FromImageDirective(image))
}

// AddEnvironment implements Builder.
func (b *builderImpl) AddEnvironment(env map[string]string) Builder {
	return b.add(EnvironmentDirective(env))
}

// AddRunCommand implements Builder.
func (b *builderImpl) AddRunCommand(cmd string) Builder {
	return b.add(RunDirective(cmd))
}

// AddCopy implements Builder.
func (b *builderImpl) AddCopy(parts ...string) Builder {
	return b.add(CopyDirective{Parts: parts})
}

// SetWorkingDirectory implements Builder.
func (b *builderImpl) SetWorkingDirectory(dir string) Builder {
	return b.add(WorkDirDirective(dir))
}

// SetCurrentUser implements Builder.
func (b *builderImpl) SetCurrentUser(user string) Builder {
	return b.add(UserDirective(user))
}

// SetEntryPoint implements Builder.
func (b *builderImpl) SetEntryPoint(cmd string) Builder {
	return b.add(EntryPointDirective(cmd))
}

// AddDeployBins implements Builder.
func (b *builderImpl) AddDeployBins(bins ...string) Builder {
	return b.add(DeployBinsDirective(bins))
}

// AddDeployPaths implements Builder.
func (b *builderImpl) AddDeployPaths(paths ...string) Builder {
	return b.add(DeployPathsDirective(paths))
}

// AddScriptTest implements Builder.
func (b *builderImpl) AddScriptTest(
	name string,
	manual bool,
	executable string,
	script string,
) Builder {
	return b.add(ScriptTestDirective{
		Name:       name,
		Manual:     manual,
		Executable: executable,
		Script:     script,
	})
}

// AddBuiltinTest implements Builder.
func (b *builderImpl) AddBuiltinTest(
	name string,
	manual bool,
	builtin string,
) Builder {
	return b.add(BuiltinTestDirective{
		Name:    name,
		Manual:  manual,
		Builtin: builtin,
	})
}

func (b *builderImpl) Compile() (*Definition, error) {
	return b.out, nil
}

func New() Builder {
	return &builderImpl{
		out: &Definition{},
	}
}
