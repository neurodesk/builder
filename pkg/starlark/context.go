package starlark

import (
	"fmt"
	"strings"

	"github.com/neurodesk/builder/pkg/jinja2"
	"go.starlark.net/starlark"
)

// RecipeContext provides an interface to recipe context for Starlark scripts
type RecipeContext interface {
	InstallPackages(pkgs ...string) error
	SetVariable(key string, value any)
	EvaluateValue(value any) (any, error)
	// AddRunCommand allows Starlark to append shell commands to the build.
	AddRunCommand(cmd string)
}

// NewEvaluatorWithStarlarkContext creates a Starlark evaluator with enhanced context
// that provides access to directive functions like installing packages
func NewEvaluatorWithStarlarkContext(ctx RecipeContext) *Evaluator {
	thread := &starlark.Thread{Name: "neurodesk-builder"}
	builtins := CreateBuiltinsWithContext(ctx)

	return &Evaluator{
		thread:   thread,
		builtins: builtins,
		globals:  make(starlark.StringDict),
	}
}

// CreateBuiltinsWithContext creates Starlark built-in functions with recipe context access
func CreateBuiltinsWithContext(ctx RecipeContext) starlark.StringDict {
	builtins := starlark.StringDict{
		"print": starlark.NewBuiltin("print", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			var buf []string
			for i := 0; i < len(args); i++ {
				buf = append(buf, args[i].String())
			}
			fmt.Println(strings.Join(buf, " "))
			return starlark.None, nil
		}),

		"install_packages": starlark.NewBuiltin("install_packages", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			if len(args) == 0 {
				return starlark.None, fmt.Errorf("install_packages requires at least one package name")
			}

			var pkgs []string
			for i := 0; i < len(args); i++ {
				pkgs = append(pkgs, args[i].String())
			}

			err := ctx.InstallPackages(pkgs...)
			if err != nil {
				return starlark.None, fmt.Errorf("installing packages: %w", err)
			}

			return starlark.None, nil
		}),

		"set_variable": starlark.NewBuiltin("set_variable", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			if len(args) != 2 {
				return starlark.None, fmt.Errorf("set_variable requires exactly 2 arguments: name, value")
			}

			// Get the name, handling string values correctly
			var name string
			if strVal, ok := args[0].(starlark.String); ok {
				name = string(strVal) // This gives us the string without quotes
			} else {
				name = args[0].String() // Fallback for other types
			}

			value := ConvertFromStarlark(args[1])

			// Convert Jinja2.Value to a Go value recursively, preserving types.
			var toGo func(jinja2.Value) any
			toGo = func(v jinja2.Value) any {
				switch t := v.(type) {
				case jinja2.StringValue:
					return string(t)
				case jinja2.IntValue:
					return int64(t)
				case jinja2.FloatValue:
					return float64(t)
				case jinja2.BoolValue:
					return bool(t)
				case jinja2.ListValue:
					out := make([]any, 0, len(t))
					for _, it := range t {
						out = append(out, toGo(it))
					}
					return out
				case jinja2.DictValue:
					out := make(map[string]any, len(t))
					for k, vv := range t {
						out[k] = toGo(vv)
					}
					return out
				case jinja2.NoneValue:
					return nil
				default:
					return v.String()
				}
			}
			goValue := toGo(value)

			ctx.SetVariable(name, goValue)
			return starlark.None, nil
		}),

		"run_command": starlark.NewBuiltin("run_command", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			if len(args) != 1 {
				return starlark.None, fmt.Errorf("run_command requires exactly 1 argument: command")
			}

			var command string
			if strVal, ok := args[0].(starlark.String); ok {
				command = string(strVal)
			} else {
				command = args[0].String()
			}

			// Append to the build via the context hook
			ctx.AddRunCommand(command)

			return starlark.None, nil
		}),

		"set_environment": starlark.NewBuiltin("set_environment", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			if len(args) != 2 {
				return starlark.None, fmt.Errorf("set_environment requires exactly 2 arguments: key, value")
			}

			var key, value string
			if strVal, ok := args[0].(starlark.String); ok {
				key = string(strVal)
			} else {
				key = args[0].String()
			}

			if strVal, ok := args[1].(starlark.String); ok {
				value = string(strVal)
			} else {
				value = args[1].String()
			}

			// Store environment variable for later processing
			envKey := "_starlark_env_" + key
			ctx.SetVariable(envKey, value)

			return starlark.None, nil
		}),
	}

	return builtins
}
