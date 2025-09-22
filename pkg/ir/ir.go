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

// RunWithMountsDirective represents a RUN instruction with BuildKit mount flags.
// Mounts are rendered before the command, e.g.:
//
//	RUN --mount=type=bind,source=cache/xyz,target=/path,readonly ["/bin/bash","-lc", "..."]
type RunWithMountsDirective struct {
	Mounts  []string
	Command string
}

// isDirective implements Directive.
func (r RunWithMountsDirective) isDirective() {}

type CopyDirective struct {
	Parts []string
}

// isDirective implements Directive.
func (c CopyDirective) isDirective() {}

type LiteralFileDirective struct {
	Name       string
	Contents   string
	Executable bool
}

// isDirective implements Directive.
func (l LiteralFileDirective) isDirective() {}

type WorkDirDirective string

// isDirective implements Directive.
func (w WorkDirDirective) isDirective() {}

type UserDirective string

// isDirective implements Directive.
func (u UserDirective) isDirective() {}

type EntryPointDirective string

// isDirective implements Directive.
func (e EntryPointDirective) isDirective() {}

// ExecEntryPointDirective represents an exec-form ENTRYPOINT with argv array.
type ExecEntryPointDirective []string

// isDirective implements Directive.
func (e ExecEntryPointDirective) isDirective() {}

var (
	_ Directive = FromImageDirective("")

	_ Directive = EnvironmentDirective{}
	_ Directive = RunDirective("")
	_ Directive = WorkDirDirective("")
	_ Directive = UserDirective("")
	_ Directive = EntryPointDirective("")
)

type Definition struct {
	Directives []Directive
}

type Builder interface {
	Compile() (*Definition, error)

	AddFromImage(image string) Builder

	AddEnvironment(env map[string]string) Builder
	AddRunCommand(cmd string) Builder
	AddRunWithMounts(mounts []string, cmd string) Builder
	AddCopy(parts ...string) Builder
	AddLiteralFile(name, contents string, executable bool) Builder
	SetWorkingDirectory(dir string) Builder
	SetCurrentUser(user string) Builder
    SetEntryPoint(cmd string) Builder
    SetExecEntryPoint(argv []string) Builder
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

// AddRunWithMounts implements Builder.
func (b *builderImpl) AddRunWithMounts(mounts []string, cmd string) Builder {
	return b.add(RunWithMountsDirective{Mounts: append([]string{}, mounts...), Command: cmd})
}

// AddCopy implements Builder.
func (b *builderImpl) AddCopy(parts ...string) Builder {
	return b.add(CopyDirective{Parts: parts})
}

// AddLiteralFile implements Builder.
func (b *builderImpl) AddLiteralFile(name, contents string, executable bool) Builder {
	return b.add(LiteralFileDirective{
		Name:       name,
		Contents:   contents,
		Executable: executable,
	})
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

// SetExecEntryPoint implements Builder.
func (b *builderImpl) SetExecEntryPoint(argv []string) Builder {
    // make a copy for safety
    out := make([]string, len(argv))
    copy(out, argv)
    return b.add(ExecEntryPointDirective(out))
}

func (b *builderImpl) Compile() (*Definition, error) {
	return b.out, nil
}

func New() Builder {
	return &builderImpl{
		out: &Definition{},
	}
}
