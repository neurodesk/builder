package jinja2

import (
	"fmt"
	"reflect"
	"unicode/utf8"
)

// Value is an abstract value used by the Jinja evaluator, inspired by Starlark.
// It defines string conversion and truthiness semantics.
type Value interface {
	String() string
	Truth() bool
}

// LookupHook can be optionally implemented by Value containers to observe
// attribute/key lookups performed by the evaluator.
type LookupHook interface {
	OnLookup(key string) (Value, bool)
}

// SetHook can be optionally implemented to observe when a template sets
// a variable in the current context.
type SetHook interface {
	OnSet(name string, val Value) error
}

// CallableValue wraps a callable function that can be invoked from templates.
// It is used to model function values and methods.
type CallableValue struct {
	Fn func(args []Value) (Value, error)
}

func (c CallableValue) String() string { return "<function>" }
func (c CallableValue) Truth() bool    { return true }

// NoneValue represents the absence of a value.
type NoneValue struct{}

func (NoneValue) String() string { return "" }
func (NoneValue) Truth() bool    { return false }

// BoolValue wraps a boolean.
type BoolValue bool

func (b BoolValue) String() string {
	if b {
		return "true"
	}
	return "false"
}
func (b BoolValue) Truth() bool { return bool(b) }

// IntValue wraps an integer (64-bit).
type IntValue int64

func (i IntValue) String() string { return fmt.Sprintf("%d", int64(i)) }
func (i IntValue) Truth() bool    { return int64(i) != 0 }

// FloatValue wraps a float (64-bit).
type FloatValue float64

func (f FloatValue) String() string { return fmt.Sprintf("%v", float64(f)) }
func (f FloatValue) Truth() bool    { return float64(f) != 0 }

// StringValue wraps a string.
type StringValue string

func (s StringValue) String() string { return string(s) }
func (s StringValue) Truth() bool    { return len(string(s)) > 0 }

// ListValue wraps a list of values.
type ListValue []Value

func (l ListValue) String() string {
	// Join by space for a simple representation
	out := ""
	for i, v := range l {
		if i > 0 {
			out += " "
		}
		out += v.String()
	}
	return out
}
func (l ListValue) Truth() bool { return len(l) > 0 }

// DictValue wraps a string-keyed dictionary of values.
type DictValue map[string]Value

func (d DictValue) String() string { return "{...}" }
func (d DictValue) Truth() bool    { return len(d) > 0 }

// NewContext creates an empty context.
type Context map[string]Value

// ContextRef is a lightweight Value wrapper used to signal that a lookup
// is occurring against the top-level context. This enables callback hooks
// to uniformly receive a Value for the container being accessed.
type ContextRef struct{ Ctx Context }

func (ContextRef) String() string { return "<context>" }
func (ContextRef) Truth() bool    { return true }

// OnLookup implements LookupHook for ContextRef as a no-op by default.
// External callers may choose to wrap Context or introduce custom Value
// types that implement LookupHook to observe lookups.
func (ContextRef) OnLookup(key string) {}

// OnSet implements SetHook for ContextRef as a no-op by default.
// External callers may choose to wrap Context or introduce custom Value
// types that implement SetHook to observe assignments.
func (ContextRef) OnSet(name string, val Value) {}

// NewContextFromAny converts a map[string]any into a Value-based Context.
// It recursively converts nested maps/slices into DictValue/ListValue.
func NewContextFromAny(m map[string]any) Context {
	ctx := Context{}
	for k, v := range m {
		ctx[k] = FromGo(v)
	}
	return ctx
}

// FromGo converts a Go value to a Value.
func FromGo(v any) Value {
	if v == nil {
		return NoneValue{}
	}
	switch t := v.(type) {
	case Value:
		return t
	case string:
		return StringValue(t)
	case bool:
		return BoolValue(t)
	case int:
		return IntValue(int64(t))
	case int32:
		return IntValue(int64(t))
	case int64:
		return IntValue(t)
	case float32:
		return FloatValue(float64(t))
	case float64:
		return FloatValue(t)
	case []byte:
		return StringValue(string(t))
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		n := rv.Len()
		out := make(ListValue, 0, n)
		for i := 0; i < n; i++ {
			out = append(out, FromGo(rv.Index(i).Interface()))
		}
		return out
	case reflect.Map:
		// Only support string keys for simplicity
		if rv.Type().Key().Kind() == reflect.String {
			out := DictValue{}
			it := rv.MapRange()
			for it.Next() {
				out[it.Key().Interface().(string)] = FromGo(it.Value().Interface())
			}
			return out
		}
	case reflect.Pointer, reflect.Interface:
		if rv.IsNil() {
			return NoneValue{}
		}
		return FromGo(rv.Elem().Interface())
	}
	// Fallback: string formatting
	return StringValue(fmt.Sprintf("%v", v))
}

// iterateValue converts a Value into a []Value for iteration semantics.
func iterateValue(v Value) ([]Value, error) {
	switch t := v.(type) {
	case NoneValue:
		return nil, nil
	case StringValue:
		s := string(t)
		var out []Value
		for len(s) > 0 {
			r, size := utf8.DecodeRuneInString(s)
			s = s[size:]
			out = append(out, StringValue(string(r)))
		}
		return out, nil
	case ListValue:
		// Copy to avoid mutating underlying array
		out := make([]Value, len(t))
		copy(out, t)
		return out, nil
	case DictValue:
		out := make([]Value, 0, len(t))
		for k := range t {
			out = append(out, StringValue(k))
		}
		return out, nil
	default:
		// Allow Go slices/arrays/maps that slipped through
		rv := reflect.ValueOf(v)
		switch rv.Kind() {
		case reflect.Slice, reflect.Array:
			var out []Value
			for i := 0; i < rv.Len(); i++ {
				out = append(out, FromGo(rv.Index(i).Interface()))
			}
			return out, nil
		case reflect.Map:
			var out []Value
			it := rv.MapRange()
			for it.Next() {
				out = append(out, FromGo(it.Key().Interface()))
			}
			return out, nil
		}
	}
	return nil, fmt.Errorf("not iterable: %T", v)
}
