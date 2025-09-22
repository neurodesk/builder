package ir

import (
	"fmt"
	"maps"
	"path/filepath"
	"strings"

	docker "github.com/neurodesk/builder/pkg/ir/docker"
)

// GenerateDockerfile converts the intermediate representation into a Dockerfile
// string by mapping IR directives to the lightweight Docker AST in pkg/ir/docker
// and rendering it. Unsupported directives are ignored at this stage.
func GenerateDockerfile(ir *Definition) (string, error) {
	if ir == nil {
		return "", fmt.Errorf("nil ir definition")
	}

	var out []docker.Directive
	for _, d := range ir.Directives {
		switch v := d.(type) {
		case FromImageDirective:
			out = append(out, docker.From{Image: string(v)})
		case EnvironmentDirective:
			// Emit as a single ENV block to keep related vars together
			env := docker.Env{}
			maps.Copy(env, v)
			out = append(out, env)
		case RunDirective:
			out = append(out, docker.Run{Command: string(v)})
		case CopyDirective:
			if len(v.Parts) < 2 {
				return "", fmt.Errorf("COPY directive requires at least two parts")
			}
			srcs := v.Parts[:len(v.Parts)-1]
			dest := v.Parts[len(v.Parts)-1]
			out = append(out, docker.Copy{Src: srcs, Dest: dest})
		case WorkDirDirective:
			out = append(out, docker.Workdir(string(v)))
		case UserDirective:
			out = append(out, docker.User(string(v)))
		case EntryPointDirective:
			out = append(out, docker.EntryPoint(string(v)))
		case RunWithMountsDirective:
			out = append(out, docker.RunWithMounts{Mounts: v.Mounts, Command: v.Command})
		case LiteralFileDirective:
			// Materialize inline file contents inside the image using a safe heredoc.
			// Use a single RUN with bash -lc to reliably handle newlines and quoting.
			name := v.Name
			contents := v.Contents
			// Ensure parent dir exists, then write file via heredoc.
			var b strings.Builder
			dir := filepath.Dir(name)
			if dir != "." && dir != "/" {
				b.WriteString("mkdir -p ")
				b.WriteString(dir)
				b.WriteString("\n")
			}
			b.WriteString("cat > ")
			b.WriteString(name)
			b.WriteString(" << 'EOF'\n")
			b.WriteString(contents)
			if !strings.HasSuffix(contents, "\n") {
				b.WriteString("\n")
			}
			b.WriteString("EOF\n")
			if v.Executable {
				b.WriteString("chmod +x ")
				b.WriteString(name)
				b.WriteString("\n")
			}
			out = append(out, docker.Run{Command: b.String()})
		default:
			return "", fmt.Errorf("unsupported directive: %T", d)
		}
	}

	return docker.RenderDockerfile(out)
}
