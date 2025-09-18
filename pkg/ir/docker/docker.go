package docker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Directive represents a single Dockerfile directive in a tiny AST.
// Implementations below intentionally keep just the data needed.
type Directive interface{ isDirective() }

// From emits `FROM <Image>`
type From struct{ Image string }

func (From) isDirective() {}

// Env emits a single grouped ENV block. Keys are rendered in sorted order
// for determinism.
type Env map[string]string

func (Env) isDirective() {}

// Run emits a RUN instruction. We render using exec form with
// ["/bin/bash", "-lc", <Command>] to preserve shell semantics and
// avoid fragile quoting/word-splitting.
type Run struct{ Command string }

func (Run) isDirective() {}

// Copy emits `COPY <srcs...> <dest>`
type Copy struct {
	Src  []string
	Dest string
}

func (Copy) isDirective() {}

// Workdir emits `WORKDIR <dir>`
type Workdir string

func (Workdir) isDirective() {}

// User emits `USER <user>`
type User string

func (User) isDirective() {}

// EntryPoint emits `ENTRYPOINT <cmd>`
type EntryPoint string

func (EntryPoint) isDirective() {}

// RenderDockerfile converts the directive list into a Dockerfile string.
func RenderDockerfile(dirs []Directive) (string, error) {
	var buf bytes.Buffer

	writeLine := func(format string, a ...any) {
		fmt.Fprintf(&buf, format+"\n", a...)
	}

	for _, d := range dirs {
		switch v := d.(type) {
		case From:
			if v.Image == "" {
				return "", fmt.Errorf("FROM: empty image")
			}
			writeLine("FROM %s", v.Image)

		case Env:
			if len(v) == 0 {
				// Skip empty ENV blocks
				continue
			}
			// Stable key order for deterministic output.
			keys := make([]string, 0, len(v))
			for k := range v {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			// Render as a single grouped ENV with continuations.
			// Values are quoted to be safe for spaces/special chars.
			for i, k := range keys {
				val := v[k]
				// Minimal escaping for double quotes and backslashes.
				esc := make([]rune, 0, len(val))
				for _, r := range val {
					switch r {
					case '"':
						esc = append(esc, '\\', '"')
					case '\\':
						esc = append(esc, '\\', '\\')
					default:
						esc = append(esc, r)
					}
				}
				if i == 0 {
					if len(keys) == 1 {
						writeLine("ENV %s=\"%s\"", k, string(esc))
					} else {
						writeLine("ENV %s=\"%s\" \\", k, string(esc))
					}
				} else if i == len(keys)-1 {
					writeLine("    %s=\"%s\"", k, string(esc))
				} else {
					writeLine("    %s=\"%s\" \\", k, string(esc))
				}
			}

		case Run:
			// Use exec form to ensure correct shell parsing and robust handling
			// of quotes, newlines, and operators. JSON-encode the argv array
			// without HTML escaping so special characters remain as-is.
			argv := []string{"/bin/bash", "-lc", v.Command}
			var jbuf bytes.Buffer
			enc := json.NewEncoder(&jbuf)
			enc.SetEscapeHTML(false)
			if err := enc.Encode(argv); err != nil {
				return "", fmt.Errorf("encoding RUN argv: %w", err)
			}
			jb := jbuf.Bytes()
			if len(jb) > 0 && jb[len(jb)-1] == '\n' {
				jb = jb[:len(jb)-1]
			}
			writeLine("RUN %s", string(jb))

		case Copy:
			if len(v.Src) == 0 {
				return "", fmt.Errorf("COPY: no source paths")
			}
			if v.Dest == "" {
				return "", fmt.Errorf("COPY: empty destination path")
			}
			// Quote each path to handle special chars robustly.
			srcs := make([]string, len(v.Src))
			for i, s := range v.Src {
				srcs[i] = fmt.Sprintf("%q", s)
			}
			dest := fmt.Sprintf("%q", v.Dest)
			writeLine("COPY %s %s", strings.Join(srcs, " "), dest)

		case Workdir:
			if v == "" {
				return "", fmt.Errorf("WORKDIR: empty path")
			}
			writeLine("WORKDIR %s", string(v))

		case User:
			if v == "" {
				return "", fmt.Errorf("USER: empty user")
			}
			writeLine("USER %s", string(v))

		case EntryPoint:
			if v == "" {
				return "", fmt.Errorf("ENTRYPOINT: empty command")
			}
			// Use exec form with JSON encoding to handle special chars robustly.
			argv := []string{"/bin/bash", "-lc", string(v)}
			var jbuf bytes.Buffer
			enc := json.NewEncoder(&jbuf)
			enc.SetEscapeHTML(false)
			if err := enc.Encode(argv); err != nil {
				return "", fmt.Errorf("encoding ENTRYPOINT argv: %w", err)
			}
			jb := jbuf.Bytes()
			if len(jb) > 0 && jb[len(jb)-1] == '\n' {
				jb = jb[:len(jb)-1]
			}
			writeLine("ENTRYPOINT %s", string(jb))
		default:
			return "", fmt.Errorf("unknown directive type: %T", d)
		}
	}

	return buf.String(), nil
}
