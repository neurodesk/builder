package jinja2

import (
    "fmt"
    "reflect"
    "strconv"
    "strings"
    "unicode/utf8"
)

type Context map[string]any

// Filters is a registry of filter functions.
type Filters map[string]func(val any, args []any) (any, error)

// DefaultFilters provides a small set of common filters.
func DefaultFilters() Filters {
    return Filters{
        "upper": func(val any, _ []any) (any, error) { return strings.ToUpper(toStringNilEmpty(val)), nil },
        "lower": func(val any, _ []any) (any, error) { return strings.ToLower(toStringNilEmpty(val)), nil },
        "trim": func(val any, _ []any) (any, error) { return strings.TrimSpace(toStringNilEmpty(val)), nil },
        "default": func(val any, args []any) (any, error) {
            if len(args) < 1 {
                return val, nil
            }
            if isTruthy(val) {
                return val, nil
            }
            return args[0], nil
        },
        "join": func(val any, args []any) (any, error) {
            sep := ","
            if len(args) > 0 {
                sep = toString(args[0])
            }
            switch v := val.(type) {
            case []string:
                return strings.Join(v, sep), nil
            default:
                rv := reflect.ValueOf(val)
                if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
                    var parts []string
                    for i := 0; i < rv.Len(); i++ {
                        parts = append(parts, toString(rv.Index(i).Interface()))
                    }
                    return strings.Join(parts, sep), nil
                }
            }
            return toString(val), nil
        },
        "length": func(val any, _ []any) (any, error) {
            rv := reflect.ValueOf(val)
            switch rv.Kind() {
            case reflect.Slice, reflect.Array, reflect.Map, reflect.String:
                return rv.Len(), nil
            }
            return 0, nil
        },
    }
}

type Evaluator struct {
    Filters Filters
}

func NewEvaluator() *Evaluator { return &Evaluator{Filters: DefaultFilters()} }

// Eval evaluates a minimal expression language for variable lookup, string and
// numeric literals, and a simple filter pipeline (e.g., name|upper|default("x")).
func (e *Evaluator) Eval(expr string, ctx Context) (any, error) {
    expr = strings.TrimSpace(expr)
    if expr == "" {
        return "", nil
    }
    parts, err := splitPipes(expr)
    if err != nil {
        return nil, err
    }
    val, err := evalAtom(parts[0], ctx)
    if err != nil {
        return nil, err
    }
    for _, f := range parts[1:] {
        name, args, err := parseFilterCall(f, ctx)
        if err != nil {
            return nil, err
        }
        fn := e.Filters[name]
        if fn == nil {
            return nil, fmt.Errorf("unknown filter: %s", name)
        }
        val, err = fn(val, args)
        if err != nil {
            return nil, err
        }
    }
    return val, nil
}

// Truthy evaluates an expression and returns its truthiness.
func (e *Evaluator) Truthy(expr string, ctx Context) (bool, error) {
    s := strings.TrimSpace(expr)
    if s == "" {
        return false, nil
    }
    if strings.HasPrefix(s, "not ") {
        b, err := e.Truthy(strings.TrimSpace(s[4:]), ctx)
        if err != nil {
            return false, err
        }
        return !b, nil
    }
    if i := strings.Index(s, "=="); i >= 0 {
        a1 := strings.TrimSpace(s[:i])
        a2 := strings.TrimSpace(s[i+2:])
        v1, err := e.Eval(a1, ctx)
        if err != nil { return false, err }
        v2, err := e.Eval(a2, ctx)
        if err != nil { return false, err }
        return equal(v1, v2), nil
    }
    if i := strings.Index(s, "!="); i >= 0 {
        a1 := strings.TrimSpace(s[:i])
        a2 := strings.TrimSpace(s[i+2:])
        v1, err := e.Eval(a1, ctx)
        if err != nil { return false, err }
        v2, err := e.Eval(a2, ctx)
        if err != nil { return false, err }
        return !equal(v1, v2), nil
    }
    v, err := e.Eval(s, ctx)
    if err != nil {
        return false, err
    }
    return isTruthy(v), nil
}

func equal(a, b any) bool {
    return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func splitPipes(s string) ([]string, error) {
    var parts []string
    var b strings.Builder
    depth := 0
    inStr := byte(0)
    for i := 0; i < len(s); i++ {
        c := s[i]
        if inStr != 0 {
            b.WriteByte(c)
            if c == inStr {
                inStr = 0
            }
            continue
        }
        switch c {
        case '\'', '"':
            inStr = c
            b.WriteByte(c)
        case '(':
            depth++
            b.WriteByte(c)
        case ')':
            if depth > 0 { depth-- }
            b.WriteByte(c)
        case '|':
            if depth == 0 {
                parts = append(parts, strings.TrimSpace(b.String()))
                b.Reset()
            } else {
                b.WriteByte(c)
            }
        default:
            b.WriteByte(c)
        }
    }
    if b.Len() > 0 {
        parts = append(parts, strings.TrimSpace(b.String()))
    }
    if len(parts) == 0 {
        return []string{s}, nil
    }
    return parts, nil
}

func parseFilterCall(s string, ctx Context) (string, []any, error) {
    s = strings.TrimSpace(s)
    if s == "" { return "", nil, fmt.Errorf("empty filter") }
    name := s
    args := []any{}
    if i := strings.IndexByte(s, '('); i >= 0 && strings.HasSuffix(s, ")") {
        name = strings.TrimSpace(s[:i])
        argStr := strings.TrimSpace(s[i+1:len(s)-1])
        if argStr != "" {
            split, err := splitArgs(argStr)
            if err != nil { return "", nil, err }
            for _, a := range split {
                v, err := evalAtom(a, ctx)
                if err != nil { return "", nil, err }
                args = append(args, v)
            }
        }
    }
    return name, args, nil
}

func splitArgs(s string) ([]string, error) {
    var parts []string
    var b strings.Builder
    depth := 0
    inStr := byte(0)
    for i := 0; i < len(s); i++ {
        c := s[i]
        if inStr != 0 {
            b.WriteByte(c)
            if c == inStr { inStr = 0 }
            continue
        }
        switch c {
        case '\'', '"':
            inStr = c
            b.WriteByte(c)
        case '(':
            depth++
            b.WriteByte(c)
        case ')':
            if depth > 0 { depth-- }
            b.WriteByte(c)
        case ',':
            if depth == 0 {
                parts = append(parts, strings.TrimSpace(b.String()))
                b.Reset()
            } else {
                b.WriteByte(c)
            }
        default:
            b.WriteByte(c)
        }
    }
    if b.Len() > 0 { parts = append(parts, strings.TrimSpace(b.String())) }
    return parts, nil
}

func evalAtom(s string, ctx Context) (any, error) {
    s = strings.TrimSpace(s)
    if s == "" {
        return "", nil
    }
    if (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"') {
        return s[1 : len(s)-1], nil
    }
    if n, err := strconv.ParseInt(s, 10, 64); err == nil {
        return n, nil
    }
    if s == "true" { return true, nil }
    if s == "false" { return false, nil }
    if s == "none" || s == "nil" || s == "null" { return nil, nil }
    // dotted identifier lookup
    parts := strings.Split(s, ".")
    var cur any = ctx
    for _, p := range parts {
        v, ok := lookup(cur, p)
        if !ok {
            return nil, nil
        }
        cur = v
    }
    return cur, nil
}

func lookup(v any, key string) (any, bool) {
    if v == nil { return nil, false }
    rv := reflect.ValueOf(v)
    switch rv.Kind() {
    case reflect.Map:
        mv := rv.MapIndex(reflect.ValueOf(key))
        if mv.IsValid() { return mv.Interface(), true }
        // string key fallback
        if rv.Type().Key().Kind() == reflect.String {
            mv := rv.MapIndex(reflect.ValueOf(key))
            if mv.IsValid() { return mv.Interface(), true }
        }
        return nil, false
    case reflect.Struct:
        f := rv.FieldByNameFunc(func(n string) bool { return strings.EqualFold(n, key) })
        if f.IsValid() { return f.Interface(), true }
        return nil, false
    case reflect.Pointer, reflect.Interface:
        if rv.IsNil() { return nil, false }
        return lookup(rv.Elem().Interface(), key)
    default:
        return nil, false
    }
}

func setVar(ctx Context, name string, val any) {
    ctx[name] = val
}

func isTruthy(v any) bool {
    if v == nil { return false }
    switch t := v.(type) {
    case bool:
        return t
    case int, int64, int32:
        return toInt(v) != 0
    case float32, float64:
        return toFloat(v) != 0
    case string:
        return strings.TrimSpace(t) != ""
    default:
        rv := reflect.ValueOf(v)
        switch rv.Kind() {
        case reflect.Slice, reflect.Array, reflect.Map, reflect.String:
            return rv.Len() > 0
        case reflect.Pointer, reflect.Interface:
            return !rv.IsNil()
        }
        return true
    }
}

func toString(v any) string { return fmt.Sprintf("%v", v) }
func toStringNilEmpty(v any) string {
    if v == nil { return "" }
    return fmt.Sprintf("%v", v)
}
func toInt(v any) int64 {
    switch t := v.(type) {
    case int: return int64(t)
    case int64: return t
    case int32: return int64(t)
    case float32: return int64(t)
    case float64: return int64(t)
    default:
        i, _ := strconv.ParseInt(fmt.Sprintf("%v", v), 10, 64)
        return i
    }
}
func toFloat(v any) float64 {
    switch t := v.(type) {
    case float64: return t
    case float32: return float64(t)
    case int: return float64(t)
    case int64: return float64(t)
    case int32: return float64(t)
    default:
        f, _ := strconv.ParseFloat(fmt.Sprintf("%v", v), 64)
        return f
    }
}

func iterate(v any) ([]any, error) {
    if v == nil { return nil, nil }
    switch t := v.(type) {
    case string:
        var out []any
        for len(t) > 0 {
            r, size := utf8.DecodeRuneInString(t)
            t = t[size:]
            out = append(out, string(r))
        }
        return out, nil
    }
    rv := reflect.ValueOf(v)
    switch rv.Kind() {
    case reflect.Slice, reflect.Array:
        var out []any
        for i := 0; i < rv.Len(); i++ {
            out = append(out, rv.Index(i).Interface())
        }
        return out, nil
    case reflect.Map:
        it := rv.MapRange()
        var out []any
        for it.Next() { out = append(out, it.Key().Interface()) }
        return out, nil
    }
    return nil, fmt.Errorf("not iterable: %T", v)
}
