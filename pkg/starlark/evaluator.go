package starlark

import (
	"fmt"

	"github.com/neurodesk/builder/pkg/jinja2"
	"go.starlark.net/starlark"
)

// Evaluator provides Starlark evaluation capabilities with access to the 
// existing Jinja2 value system and recipe context
type Evaluator struct {
	thread   *starlark.Thread
	builtins starlark.StringDict
	globals  starlark.StringDict
}

// NewEvaluator creates a new Starlark evaluator
func NewEvaluator() *Evaluator {
	thread := &starlark.Thread{Name: "neurodesk-builder"}
	builtins := CreateBuiltins(nil) // No context initially
	
	return &Evaluator{
		thread:   thread,
		builtins: builtins,
		globals:  make(starlark.StringDict),
	}
}

// NewEvaluatorWithContext creates a new Starlark evaluator with access to a recipe context
func NewEvaluatorWithContext(ctx interface{}) *Evaluator {
	thread := &starlark.Thread{Name: "neurodesk-builder"}
	builtins := CreateBuiltins(ctx)
	
	return &Evaluator{
		thread:   thread,
		builtins: builtins,
		globals:  make(starlark.StringDict),
	}
}

// SetGlobal sets a global variable in the Starlark environment
func (e *Evaluator) SetGlobal(name string, value jinja2.Value) {
	e.globals[name] = ConvertToStarlark(value)
}

// SetGlobalStarlark sets a global variable using a native Starlark value
func (e *Evaluator) SetGlobalStarlark(name string, value starlark.Value) {
	e.globals[name] = value
}

// Eval evaluates a Starlark expression and returns the result as a Jinja2 Value
func (e *Evaluator) Eval(expr string) (jinja2.Value, error) {
	// Combine builtins and globals for evaluation
	predeclared := make(starlark.StringDict)
	for k, v := range e.builtins {
		predeclared[k] = v
	}
	for k, v := range e.globals {
		predeclared[k] = v
	}

	// Evaluate the expression
	val, err := starlark.Eval(e.thread, "<eval>", expr, predeclared)
	if err != nil {
		return nil, fmt.Errorf("starlark evaluation error: %w", err)
	}

	return ConvertFromStarlark(val), nil
}

// ExecFile executes a Starlark file and returns any globals that were modified
func (e *Evaluator) ExecFile(filename string, src interface{}) (starlark.StringDict, error) {
	// Combine builtins and globals for execution
	predeclared := make(starlark.StringDict)
	for k, v := range e.builtins {
		predeclared[k] = v
	}
	for k, v := range e.globals {
		predeclared[k] = v
	}

	// Execute the file
	globals, err := starlark.ExecFile(e.thread, filename, src, predeclared)
	if err != nil {
		return nil, fmt.Errorf("starlark execution error: %w", err)
	}

	// Update our globals with any new values
	for k, v := range globals {
		e.globals[k] = v
	}

	return globals, nil
}

// ExecString executes a Starlark script from a string
func (e *Evaluator) ExecString(script string) (starlark.StringDict, error) {
	return e.ExecFile("<script>", script)
}

// GetGlobal retrieves a global variable as a Jinja2 Value
func (e *Evaluator) GetGlobal(name string) (jinja2.Value, bool) {
	if val, ok := e.globals[name]; ok {
		return ConvertFromStarlark(val), true
	}
	return nil, false
}

// LoadJinja2Context loads variables from a Jinja2 context into the Starlark globals
func (e *Evaluator) LoadJinja2Context(ctx jinja2.Context) {
	for key, value := range ctx {
		e.SetGlobal(key, value)
	}
}

// ExportToJinja2Context exports current Starlark globals to a Jinja2 context
func (e *Evaluator) ExportToJinja2Context() jinja2.Context {
	ctx := make(jinja2.Context)
	for key, value := range e.globals {
		// Skip built-in functions and internal variables
		if !isExportableKey(key) {
			continue
		}
		ctx[key] = ConvertFromStarlark(value)
	}
	return ctx
}

// isExportableKey determines if a global variable should be exported to Jinja2
func isExportableKey(key string) bool {
	// Skip built-in functions and variables starting with underscore
	switch key {
	case "print", "install_packages", "add_directive", "get_parameter":
		return false
	}
	return key[0] != '_'
}