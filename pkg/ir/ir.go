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

type DirectiveWithMetadata struct {
	Directive Directive
	Source    SourceID
}

type Definition struct {
	Directives []DirectiveWithMetadata
}

type SourceID string

type Builder interface {
	Compile() (*Definition, error)

	AddFromImage(src SourceID, image string) Builder

	AddEnvironment(src SourceID, env map[string]string) Builder
	AddRunCommand(src SourceID, cmd string) Builder
	AddRunWithMounts(src SourceID, mounts []string, cmd string) Builder
	AddCopy(src SourceID, parts ...string) Builder
	AddLiteralFile(src SourceID, name, contents string, executable bool) Builder
	SetWorkingDirectory(src SourceID, dir string) Builder
	SetCurrentUser(src SourceID, user string) Builder
	SetEntryPoint(src SourceID, cmd string) Builder
	SetExecEntryPoint(src SourceID, argv []string) Builder
}

type builderImpl struct {
	out *Definition
}

func (b *builderImpl) String() string {
	return fmt.Sprintf("%#v", b.out)
}

func (b *builderImpl) add(src SourceID, d Directive) *builderImpl {
	ret := *b
	ret.out = &Definition{
		Directives: append(append([]DirectiveWithMetadata{}, b.out.Directives...), DirectiveWithMetadata{
			Directive: d,
			Source:    src,
		}),
	}
	return &ret
}

func (b *builderImpl) AddFromImage(src SourceID, image string) Builder {
	return b.add(src, FromImageDirective(image))
}

// AddEnvironment implements Builder.
func (b *builderImpl) AddEnvironment(src SourceID, env map[string]string) Builder {
	return b.add(src, EnvironmentDirective(env))
}

// AddRunCommand implements Builder.
func (b *builderImpl) AddRunCommand(src SourceID, cmd string) Builder {
	return b.add(src, RunDirective(cmd))
}

// AddRunWithMounts implements Builder.
func (b *builderImpl) AddRunWithMounts(src SourceID, mounts []string, cmd string) Builder {
	return b.add(src, RunWithMountsDirective{Mounts: append([]string{}, mounts...), Command: cmd})
}

// AddCopy implements Builder.
func (b *builderImpl) AddCopy(src SourceID, parts ...string) Builder {
	return b.add(src, CopyDirective{Parts: parts})
}

// AddLiteralFile implements Builder.
func (b *builderImpl) AddLiteralFile(src SourceID, name, contents string, executable bool) Builder {
	return b.add(src, LiteralFileDirective{
		Name:       name,
		Contents:   contents,
		Executable: executable,
	})
}

// SetWorkingDirectory implements Builder.
func (b *builderImpl) SetWorkingDirectory(src SourceID, dir string) Builder {
	return b.add(src, WorkDirDirective(dir))
}

// SetCurrentUser implements Builder.
func (b *builderImpl) SetCurrentUser(src SourceID, user string) Builder {
	return b.add(src, UserDirective(user))
}

// SetEntryPoint implements Builder.
func (b *builderImpl) SetEntryPoint(src SourceID, cmd string) Builder {
	return b.add(src, EntryPointDirective(cmd))
}

// SetExecEntryPoint implements Builder.
func (b *builderImpl) SetExecEntryPoint(src SourceID, argv []string) Builder {
	// make a copy for safety
	out := make([]string, len(argv))
	copy(out, argv)
	return b.add(src, ExecEntryPointDirective(out))
}

func (b *builderImpl) Compile() (*Definition, error) {
	return b.out, nil
}

func New() Builder {
	return &builderImpl{
		out: &Definition{},
	}
}
