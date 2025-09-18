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

			// Convert Jinja2 value to Go interface for the context
			var goValue any
			switch v := value.(type) {
			case jinja2.StringValue:
				goValue = string(v)
			case jinja2.IntValue:
				goValue = int64(v)
			case jinja2.FloatValue:
				goValue = float64(v)
			case jinja2.BoolValue:
				goValue = bool(v)
			case jinja2.ListValue:
				items := make([]any, len(v))
				for i, item := range v {
					items[i] = item.String() // Simplify to strings for now
				}
				goValue = items
			case jinja2.DictValue:
				dict := make(map[string]any)
				for k, v := range v {
					dict[k] = v.String() // Simplify to strings for now
				}
				goValue = dict
			default:
				goValue = value.String()
			}

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

			// Set a special variable that the recipe context can interpret
			ctx.SetVariable("_starlark_run_command", command)

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
