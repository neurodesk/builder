package starlark

import (
	"fmt"

	"github.com/neurodesk/builder/pkg/jinja2"
	"go.starlark.net/starlark"
)

// ConvertToStarlark converts a Jinja2 Value to a Starlark value
func ConvertToStarlark(val jinja2.Value) starlark.Value {
	if val == nil {
		return starlark.None
	}

	switch v := val.(type) {
	case jinja2.StringValue:
		return starlark.String(string(v))
	case jinja2.IntValue:
		return starlark.MakeInt64(int64(v))
	case jinja2.FloatValue:
		return starlark.Float(float64(v))
	case jinja2.BoolValue:
		return starlark.Bool(bool(v))
	case jinja2.ListValue:
		items := make([]starlark.Value, len(v))
		for i, item := range v {
			items[i] = ConvertToStarlark(item)
		}
		return starlark.NewList(items)
	case jinja2.DictValue:
		dict := starlark.NewDict(len(v))
		for key, value := range v {
			dict.SetKey(starlark.String(key), ConvertToStarlark(value))
		}
		return dict
	case jinja2.NoneValue:
		return starlark.None
	default:
		// For unknown types, convert to string
		return starlark.String(val.String())
	}
}

// ConvertFromStarlark converts a Starlark value to a Jinja2 Value
func ConvertFromStarlark(val starlark.Value) jinja2.Value {
	if val == nil || val == starlark.None {
		return jinja2.NoneValue{}
	}

	switch v := val.(type) {
	case starlark.String:
		return jinja2.StringValue(string(v))
	case starlark.Int:
		if i, ok := v.Int64(); ok {
			return jinja2.IntValue(i)
		}
		// For very large integers, convert to string
		return jinja2.StringValue(v.String())
	case starlark.Float:
		return jinja2.FloatValue(float64(v))
	case starlark.Bool:
		return jinja2.BoolValue(bool(v))
	case *starlark.List:
		items := make(jinja2.ListValue, v.Len())
		for i := 0; i < v.Len(); i++ {
			items[i] = ConvertFromStarlark(v.Index(i))
		}
		return items
	case *starlark.Dict:
		dict := make(jinja2.DictValue)
		for _, item := range v.Items() {
			key := item[0]
			value := item[1]
			if keyStr, ok := key.(starlark.String); ok {
				dict[string(keyStr)] = ConvertFromStarlark(value)
			} else {
				dict[key.String()] = ConvertFromStarlark(value)
			}
		}
		return dict
	default:
		// For unknown types, convert to string
		return jinja2.StringValue(val.String())
	}
}

// StarlarkValueWrapper wraps a Starlark value to implement jinja2.Value
type StarlarkValueWrapper struct {
	Value starlark.Value
}

func (w StarlarkValueWrapper) String() string {
	return w.Value.String()
}

func (w StarlarkValueWrapper) Truth() bool {
	if w.Value == nil {
		return false
	}
	switch v := w.Value.(type) {
	case starlark.Bool:
		return bool(v)
	case starlark.String:
		return string(v) != ""
	case starlark.Int:
		return v.Sign() != 0
	case starlark.Float:
		return float64(v) != 0.0
	case *starlark.List:
		return v.Len() > 0
	case *starlark.Dict:
		return v.Len() > 0
	default:
		return w.Value != starlark.None
	}
}

// Ensure StarlarkValueWrapper implements jinja2.Value
var _ jinja2.Value = StarlarkValueWrapper{}

// ContextObject implements a Starlark object that provides access to context variables
type ContextObject struct {
	variables map[string]starlark.Value
}

// NewContextObject creates a new context object from a Jinja2 context
func NewContextObject(ctx jinja2.Context) *ContextObject {
	variables := make(map[string]starlark.Value)
	for key, value := range ctx {
		variables[key] = ConvertToStarlark(value)
	}
	return &ContextObject{variables: variables}
}

// String implements starlark.Value
func (c *ContextObject) String() string {
	return "<context>"
}

// Type implements starlark.Value
func (c *ContextObject) Type() string {
	return "context"
}

// Freeze implements starlark.Value
func (c *ContextObject) Freeze() {
	// Context objects are immutable
}

// Truth implements starlark.Value
func (c *ContextObject) Truth() starlark.Bool {
	return starlark.True
}

// Hash implements starlark.Value
func (c *ContextObject) Hash() (uint32, error) {
	return 0, fmt.Errorf("context object is not hashable")
}

// Attr implements starlark.HasAttrs
func (c *ContextObject) Attr(name string) (starlark.Value, error) {
	if val, ok := c.variables[name]; ok {
		return val, nil
	}
	return nil, nil // Return nil for missing attributes (Starlark will handle this)
}

// AttrNames implements starlark.HasAttrs
func (c *ContextObject) AttrNames() []string {
	names := make([]string, 0, len(c.variables))
	for name := range c.variables {
		names = append(names, name)
	}
	return names
}

// SetField implements starlark.HasSetField (making it read-only by returning error)
func (c *ContextObject) SetField(name string, val starlark.Value) error {
	return fmt.Errorf("context object is read-only")
}

// Ensure ContextObject implements required interfaces
var _ starlark.Value = (*ContextObject)(nil)
var _ starlark.HasAttrs = (*ContextObject)(nil)
var _ starlark.HasSetField = (*ContextObject)(nil)

// CreateBuiltins creates Starlark built-in functions that provide access to
// the directive system and parameter access
func CreateBuiltins(ctx interface{}) starlark.StringDict {
	builtins := starlark.StringDict{
		"print": starlark.NewBuiltin("print", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			var buf []string
			for i := 0; i < len(args); i++ {
				buf = append(buf, args[i].String())
			}
			fmt.Println(buf)
			return starlark.None, nil
		}),
	}

	// Add context-specific functions if context is provided
	if ctx != nil {
		// These will be implemented when we integrate with the recipe context
		builtins["install_packages"] = starlark.NewBuiltin("install_packages", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			return starlark.None, fmt.Errorf("install_packages not yet implemented")
		})

		builtins["add_directive"] = starlark.NewBuiltin("add_directive", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			return starlark.None, fmt.Errorf("add_directive not yet implemented")
		})

		builtins["get_parameter"] = starlark.NewBuiltin("get_parameter", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			return starlark.None, fmt.Errorf("get_parameter not yet implemented")
		})
	}

	return builtins
}

// WrapJinja2Context wraps a Jinja2 context for use in Starlark
func WrapJinja2Context(ctx jinja2.Context) starlark.StringDict {
	wrapped := make(starlark.StringDict)
	for key, value := range ctx {
		wrapped[key] = ConvertToStarlark(value)
	}
	return wrapped
}
