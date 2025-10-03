package jinja2

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// Filters is a registry of filter functions.
type Filters map[string]func(val Value, args []Value) (Value, error)

// DefaultFilters provides a small set of common filters.
func DefaultFilters() Filters {
	return Filters{
		"upper": func(val Value, _ []Value) (Value, error) { return StringValue(strings.ToUpper(val.String())), nil },
		"lower": func(val Value, _ []Value) (Value, error) { return StringValue(strings.ToLower(val.String())), nil },
		"trim":  func(val Value, _ []Value) (Value, error) { return StringValue(strings.TrimSpace(val.String())), nil },
		"list": func(val Value, _ []Value) (Value, error) {
			items, err := iterateValue(val)
			if err != nil {
				// Treat as single-item list if not iterable
				return ListValue{StringValue(val.String())}, nil
			}
			out := make(ListValue, 0, len(items))
			out = append(out, items...)
			return out, nil
		},
		"map": func(val Value, args []Value) (Value, error) {
			if len(args) < 1 {
				return val, nil
			}
			name := args[0].String()
			items, err := iterateValue(val)
			if err != nil {
				return nil, fmt.Errorf("map expects an iterable")
			}
			out := make(ListValue, 0, len(items))
			switch name {
			case "int":
				for _, it := range items {
					s := strings.TrimSpace(it.String())
					if s == "" {
						out = append(out, IntValue(0))
						continue
					}
					n, err := strconv.ParseInt(s, 10, 64)
					if err != nil {
						return nil, fmt.Errorf("map('int'): cannot parse %q as int", s)
					}
					out = append(out, IntValue(n))
				}
			case "string", "str":
				for _, it := range items {
					out = append(out, StringValue(it.String()))
				}
			default:
				// Unknown mapping; pass through unchanged
				out = append(out, items...)
			}
			return out, nil
		},
		"default": func(val Value, args []Value) (Value, error) {
			if len(args) < 1 {
				return val, nil
			}
			if val.Truth() {
				return val, nil
			}
			return args[0], nil
		},
		"join": func(val Value, args []Value) (Value, error) {
			sep := ","
			if len(args) > 0 {
				sep = args[0].String()
			}
			switch v := val.(type) {
			case ListValue:
				parts := make([]string, 0, len(v))
				for _, it := range v {
					parts = append(parts, it.String())
				}
				return StringValue(strings.Join(parts, sep)), nil
			default:
				return StringValue(val.String()), nil
			}
		},
		"length": func(val Value, _ []Value) (Value, error) {
			switch v := val.(type) {
			case StringValue:
				return IntValue(int64(len(string(v)))), nil
			case ListValue:
				return IntValue(int64(len(v))), nil
			case DictValue:
				return IntValue(int64(len(v))), nil
			default:
				rv := reflect.ValueOf(val)
				switch rv.Kind() {
				case reflect.Slice, reflect.Array, reflect.Map, reflect.String:
					return IntValue(int64(rv.Len())), nil
				}
			}
			return IntValue(0), nil
		},
	}
}

type Evaluator struct {
	Filters Filters
	Funcs   map[string]func(args []Value) (Value, error)
}

func NewEvaluator() *Evaluator {
	return &Evaluator{
		Filters: DefaultFilters(),
		Funcs: map[string]func(args []Value) (Value, error){
			"raise": func(args []Value) (Value, error) {
				if len(args) == 0 {
					return nil, fmt.Errorf("raise requires a message")
				}
				return nil, errors.New(args[0].String())
			},
		},
	}
}

// Eval evaluates a minimal expression language for variable lookup, string and
// numeric literals, and a simple filter pipeline (e.g., name|upper|default("x")).
func (e *Evaluator) Eval(expr string, ctx Context) (Value, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return StringValue(""), nil
	}
	parts, err := splitPipes(expr)
	if err != nil {
		return nil, err
	}
	val, err := e.evalAtom(parts[0], ctx)
	if err != nil {
		return nil, err
	}
	for _, f := range parts[1:] {
		name, args, err := e.parseFilterCall(f, ctx)
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
	// Strip redundant outer parentheses for grouping, e.g. (a == b)
	for hasOuterParens(s) {
		s = trimOuterParens(s)
	}
	if strings.HasPrefix(s, "not ") {
		b, err := e.Truthy(strings.TrimSpace(s[4:]), ctx)
		if err != nil {
			return false, err
		}
		return !b, nil
	}
	if parts, ok := splitLogical(s, "or"); ok && len(parts) > 1 {
		for _, part := range parts {
			if part == "" {
				continue
			}
			b, err := e.Truthy(part, ctx)
			if err != nil {
				return false, err
			}
			if b {
				return true, nil
			}
		}
		return false, nil
	}
	if parts, ok := splitLogical(s, "and"); ok && len(parts) > 1 {
		for _, part := range parts {
			if part == "" {
				return false, nil
			}
			b, err := e.Truthy(part, ctx)
			if err != nil {
				return false, err
			}
			if !b {
				return false, nil
			}
		}
		return true, nil
	}
	// handle 'not in' and 'in' operators
	if op, lhs, rhs, ok := findInOperator(s); ok {
		lv, err := e.Eval(lhs, ctx)
		if err != nil {
			return false, err
		}
		list, err := parseListLiteral(rhs)
		if err != nil {
			return false, err
		}
		contains := false
		for _, it := range list {
			if it.String() == lv.String() {
				contains = true
				break
			}
		}
		if op == "in" {
			return contains, nil
		}
		if op == "not in" {
			return !contains, nil
		}
	}
	// handle ordering comparisons >, >=, <, <=
	if op, a1, a2, ok := splitComparison(s); ok {
		v1, err := e.Eval(a1, ctx)
		if err != nil {
			return false, err
		}
		v2, err := e.Eval(a2, ctx)
		if err != nil {
			return false, err
		}
		c := compareValues(v1, v2)
		switch op {
		case ">":
			return c > 0, nil
		case ">=":
			return c >= 0, nil
		case "<":
			return c < 0, nil
		case "<=":
			return c <= 0, nil
		}
	}
	if i := strings.Index(s, "=="); i >= 0 {
		a1 := strings.TrimSpace(s[:i])
		a2 := strings.TrimSpace(s[i+2:])
		v1, err := e.Eval(a1, ctx)
		if err != nil {
			return false, err
		}
		v2, err := e.Eval(a2, ctx)
		if err != nil {
			return false, err
		}
		return v1.String() == v2.String(), nil
	}
	if i := strings.Index(s, "!="); i >= 0 {
		a1 := strings.TrimSpace(s[:i])
		a2 := strings.TrimSpace(s[i+2:])
		v1, err := e.Eval(a1, ctx)
		if err != nil {
			return false, err
		}
		v2, err := e.Eval(a2, ctx)
		if err != nil {
			return false, err
		}
		return v1.String() != v2.String(), nil
	}
	v, err := e.Eval(s, ctx)
	if err != nil {
		return false, err
	}
	return v.Truth(), nil
}

func splitLogical(s, op string) ([]string, bool) {
	lower := strings.ToLower(s)
	depthParen, depthBracket, depthBrace := 0, 0, 0
	inStr := byte(0)
	last := 0
	var parts []string
	for i := 0; i <= len(lower)-len(op); i++ {
		c := s[i]
		if inStr != 0 {
			if c == inStr {
				inStr = 0
			}
			continue
		}
		switch c {
		case '\'', '"':
			inStr = c
		case '(':
			depthParen++
		case ')':
			if depthParen > 0 {
				depthParen--
			}
		case '[':
			depthBracket++
		case ']':
			if depthBracket > 0 {
				depthBracket--
			}
		case '{':
			depthBrace++
		case '}':
			if depthBrace > 0 {
				depthBrace--
			}
		}
		if depthParen == 0 && depthBracket == 0 && depthBrace == 0 && strings.HasPrefix(lower[i:], op) {
			if logicalBoundary(lower, i, op) {
				segment := strings.TrimSpace(s[last:i])
				parts = append(parts, segment)
				i += len(op)
				last = i
				i--
			}
		}
	}
	if len(parts) > 0 {
		segment := strings.TrimSpace(s[last:])
		parts = append(parts, segment)
		return parts, true
	}
	return nil, false
}

func logicalBoundary(s string, idx int, op string) bool {
	if idx > 0 {
		if isIdentifierChar(rune(s[idx-1])) {
			return false
		}
	}
	after := idx + len(op)
	if after < len(s) {
		if isIdentifierChar(rune(s[after])) {
			return false
		}
	}
	return true
}

func isIdentifierChar(r rune) bool {
	if r >= 'a' && r <= 'z' {
		return true
	}
	if r >= '0' && r <= '9' {
		return true
	}
	return r == '_' || r == '.'
}

// findInOperator finds top-level 'in' or 'not in' in s, ignoring brackets/quotes.
func findInOperator(s string) (string, string, string, bool) {
	depth := 0
	inStr := byte(0)
	for _, pat := range []string{" not in ", " in "} {
		depth = 0
		inStr = 0
		for i := 0; i+len(pat) <= len(s); i++ {
			c := s[i]
			if inStr != 0 {
				if c == inStr {
					inStr = 0
				}
				continue
			}
			switch c {
			case '\'', '"':
				inStr = c
			case '[', '(':
				depth++
			case ']', ')':
				if depth > 0 {
					depth--
				}
			}
			if depth == 0 && strings.HasPrefix(s[i:], pat) {
				lhs := strings.TrimSpace(s[:i])
				rhs := strings.TrimSpace(s[i+len(pat):])
				return strings.TrimSpace(pat), lhs, rhs, true
			}
		}
	}
	return "", "", "", false
}

// parseListLiteral parses a list like ["a", "b"].
func parseListLiteral(s string) (ListValue, error) {
	s = strings.TrimSpace(s)
	if !(strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]")) && !(strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")")) {
		return nil, fmt.Errorf("invalid list literal: %s", s)
	}
	inner := strings.TrimSpace(s[1 : len(s)-1])
	if inner == "" {
		return ListValue{}, nil
	}
	parts, err := splitArgs(inner)
	if err != nil {
		return nil, err
	}
	out := make(ListValue, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if (strings.HasPrefix(p, "\"") && strings.HasSuffix(p, "\"")) || (strings.HasPrefix(p, "'") && strings.HasSuffix(p, "'")) {
			out = append(out, StringValue(p[1:len(p)-1]))
		} else {
			out = append(out, StringValue(p))
		}
	}
	return out, nil
}

// parseDictLiteral parses a simple dict like {"a": "b", "n": 1}.
// Keys should be quoted strings. Values may be quoted strings, numbers,
// or simple names (treated as strings).
func parseDictLiteral(s string) (DictValue, error) {
	s = strings.TrimSpace(s)
	if !(strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")) {
		return nil, fmt.Errorf("invalid dict literal: %s", s)
	}
	inner := strings.TrimSpace(s[1 : len(s)-1])
	out := DictValue{}
	if inner == "" {
		return out, nil
	}
	// split top-level commas
	parts, err := splitTopLevel(inner, ',')
	if err != nil {
		return nil, err
	}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// split by first ':' at top level
		kvParts, err := splitTopLevel(p, ':')
		if err != nil {
			return nil, err
		}
		if len(kvParts) < 2 {
			return nil, fmt.Errorf("invalid dict item: %s", p)
		}
		keyStr := strings.TrimSpace(kvParts[0])
		valStr := strings.TrimSpace(strings.Join(kvParts[1:], ":"))
		// keys must be quoted strings
		if len(keyStr) >= 2 && ((keyStr[0] == '"' && keyStr[len(keyStr)-1] == '"') || (keyStr[0] == '\'' && keyStr[len(keyStr)-1] == '\'')) {
			keyStr = keyStr[1 : len(keyStr)-1]
		} else {
			return nil, fmt.Errorf("dict key must be quoted string: %s", kvParts[0])
		}
		// parse value using existing atom evaluation on a temporary evaluator
		// Note: avoid filters/pipes here; values are simple literals typically.
		// Reuse parse of numbers/strings/tuples/lists.
		if (len(valStr) >= 2 && ((valStr[0] == '"' && valStr[len(valStr)-1] == '"') || (valStr[0] == '\'' && valStr[len(valStr)-1] == '\''))) ||
			(strings.HasPrefix(valStr, "[") && strings.HasSuffix(valStr, "]")) ||
			(strings.HasPrefix(valStr, "(") && strings.HasSuffix(valStr, ")")) ||
			(strings.HasPrefix(valStr, "{") && strings.HasSuffix(valStr, "}")) {
			// literal forms we can parse via evalAtom without context
			v, err := NewEvaluator().evalAtom(valStr, Context{})
			if err != nil {
				return nil, err
			}
			out[keyStr] = v
			continue
		}
		// try numeric
		if n, err := strconv.ParseInt(valStr, 10, 64); err == nil {
			out[keyStr] = IntValue(n)
			continue
		}
		if f, err := strconv.ParseFloat(valStr, 64); err == nil {
			out[keyStr] = FloatValue(f)
			continue
		}
		// fallback: treat as string literal
		out[keyStr] = StringValue(valStr)
	}
	return out, nil
}

// splitTopLevel splits s by sep rune, ignoring separators inside (), [], {}, and quotes.
func splitTopLevel(s string, sep rune) ([]string, error) {
	var parts []string
	var b strings.Builder
	depthParen, depthBracket, depthBrace := 0, 0, 0
	inStr := byte(0)
	for _, c := range s {
		if inStr != 0 {
			b.WriteRune(c)
			if byte(c) == inStr {
				inStr = 0
			}
			continue
		}
		switch c {
		case '\'', '"':
			inStr = byte(c)
			b.WriteRune(c)
		case '(':
			depthParen++
			b.WriteRune(c)
		case ')':
			if depthParen > 0 {
				depthParen--
			}
			b.WriteRune(c)
		case '[':
			depthBracket++
			b.WriteRune(c)
		case ']':
			if depthBracket > 0 {
				depthBracket--
			}
			b.WriteRune(c)
		case '{':
			depthBrace++
			b.WriteRune(c)
		case '}':
			if depthBrace > 0 {
				depthBrace--
			}
			b.WriteRune(c)
		default:
			if c == sep && depthParen == 0 && depthBracket == 0 && depthBrace == 0 {
				parts = append(parts, strings.TrimSpace(b.String()))
				b.Reset()
			} else {
				b.WriteRune(c)
			}
		}
	}
	if b.Len() > 0 {
		parts = append(parts, strings.TrimSpace(b.String()))
	}
	return parts, nil
}

// splitComparison splits an expression by a top-level comparison operator
// among >=, <=, >, < while ignoring brackets and quoted strings.
func splitComparison(s string) (op, a1, a2 string, ok bool) {
	ops := []string{">=", "<=", ">", "<"}
	depth := 0
	inStr := byte(0)
	for _, o := range ops {
		depth = 0
		inStr = 0
		for i := 0; i+len(o) <= len(s); i++ {
			c := s[i]
			if inStr != 0 {
				if c == inStr {
					inStr = 0
				}
				continue
			}
			switch c {
			case '\'', '"':
				inStr = c
			case '(', '[', '{':
				depth++
			case ')', ']', '}':
				if depth > 0 {
					depth--
				}
			}
			if depth == 0 && strings.HasPrefix(s[i:], o) {
				return o, strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+len(o):]), true
			}
		}
	}
	return "", "", "", false
}

// compareValues compares values across basic types: ints, floats, strings,
// and lists (lexicographically). Falls back to string comparison.
func compareValues(a, b Value) int {
	switch av := a.(type) {
	case IntValue:
		switch bv := b.(type) {
		case IntValue:
			if av < bv {
				return -1
			}
			if av > bv {
				return 1
			}
			return 0
		case FloatValue:
			af := float64(av)
			bf := float64(bv)
			if af < bf {
				return -1
			}
			if af > bf {
				return 1
			}
			return 0
		}
	case FloatValue:
		af := float64(av)
		switch bv := b.(type) {
		case FloatValue:
			bf := float64(bv)
			if af < bf {
				return -1
			}
			if af > bf {
				return 1
			}
			return 0
		case IntValue:
			bf := float64(bv)
			if af < bf {
				return -1
			}
			if af > bf {
				return 1
			}
			return 0
		}
	case StringValue:
		as := string(av)
		if bs, ok := b.(StringValue); ok {
			if as < string(bs) {
				return -1
			}
			if as > string(bs) {
				return 1
			}
			return 0
		}
	case ListValue:
		if bl, ok := b.(ListValue); ok {
			n := len(av)
			if len(bl) < n {
				n = len(bl)
			}
			for i := 0; i < n; i++ {
				c := compareValues(av[i], bl[i])
				if c != 0 {
					return c
				}
			}
			if len(av) < len(bl) {
				return -1
			}
			if len(av) > len(bl) {
				return 1
			}
			return 0
		}
	}
	as := a.String()
	bs := b.String()
	if as < bs {
		return -1
	}
	if as > bs {
		return 1
	}
	return 0
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
			if depth > 0 {
				depth--
			}
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

func (e *Evaluator) parseFilterCall(s string, ctx Context) (string, []Value, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil, fmt.Errorf("empty filter")
	}
	name := s
	args := []Value{}
	if i := strings.IndexByte(s, '('); i >= 0 && strings.HasSuffix(s, ")") {
		name = strings.TrimSpace(s[:i])
		argStr := strings.TrimSpace(s[i+1 : len(s)-1])
		if argStr != "" {
			split, err := splitArgs(argStr)
			if err != nil {
				return "", nil, err
			}
			for _, a := range split {
				v, err := e.evalAtom(a, ctx)
				if err != nil {
					return "", nil, err
				}
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
			if depth > 0 {
				depth--
			}
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
	if b.Len() > 0 {
		parts = append(parts, strings.TrimSpace(b.String()))
	}
	return parts, nil
}

func (e *Evaluator) evalAtom(s string, ctx Context) (Value, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return StringValue(""), nil
	}
	// Handle grouping parentheses: (expr)
	if hasOuterParens(s) {
		inner := trimOuterParens(s)
		return e.Eval(inner, ctx)
	}
	if (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"') {
		return StringValue(s[1 : len(s)-1]), nil
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return IntValue(n), nil
	}
	if s == "true" {
		return BoolValue(true), nil
	}
	if s == "false" {
		return BoolValue(false), nil
	}
	if s == "none" || s == "nil" || s == "null" {
		return NoneValue{}, nil
	}
	// list or tuple literal
	if (strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]")) || (strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")")) {
		lv, err := parseListLiteral(s)
		if err != nil {
			return nil, err
		}
		return lv, nil
	}
	// dict literal: {"k": "v", ...}
	if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") {
		dv, err := parseDictLiteral(s)
		if err != nil {
			return nil, err
		}
		return dv, nil
	}
	// Complex reference evaluation: supports dotted lookup, calls, and indexing.
	if strings.ContainsAny(s, ".([") {
		return e.evalRef(s, ctx)
	}
	// Simple name lookup
	if lh, ok := any(ContextRef{Ctx: ctx}).(LookupHook); ok {
		lh.OnLookup(s)
	}
	if v, ok := ctx[s]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("undefined variable: %s", s)
}

// hasOuterParens reports whether s is wrapped by a single pair of matching
// parentheses that enclose the entire expression (i.e., grouping, not tuple).
func hasOuterParens(s string) bool {
	if len(s) < 2 || s[0] != '(' || s[len(s)-1] != ')' {
		return false
	}
	depth := 0
	inStr := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inStr != 0 {
			if c == inStr {
				inStr = 0
			}
			continue
		}
		switch c {
		case '\'', '"':
			inStr = c
		case '(':
			depth++
		case ')':
			depth--
			// If we reached the final closing paren before the end, then
			// outer parentheses do not wrap the whole string.
			if depth == 0 && i != len(s)-1 {
				return false
			}
		}
	}
	return true
}

// trimOuterParens returns s without a single outer pair of parentheses.
func trimOuterParens(s string) string {
	if hasOuterParens(s) {
		return strings.TrimSpace(s[1 : len(s)-1])
	}
	return s
}

// evalRef parses and evaluates a reference expression like:
//
//	name
//	name(arg1, "x")
//	obj.attr
//	obj.method(arg)
//	obj[key]
//	obj[expr]
func (e *Evaluator) evalRef(s string, ctx Context) (Value, error) {
	i := 0
	readIdent := func() (string, error) {
		start := i
		for i < len(s) {
			c := s[i]
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
				i++
				continue
			}
			break
		}
		if i == start {
			return "", fmt.Errorf("expected identifier in %q at %d", s, i)
		}
		return s[start:i], nil
	}
	skipSpaces := func() {
		for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
			i++
		}
	}
	parseArgs := func() ([]string, error) {
		if i >= len(s) || s[i] != '(' {
			return nil, fmt.Errorf("expected '('")
		}
		// find matching ) considering nested parentheses and quotes
		depth := 0
		inStr := byte(0)
		start := i + 1
		i++
		for i < len(s) {
			c := s[i]
			if inStr != 0 {
				if c == inStr {
					inStr = 0
				}
				i++
				continue
			}
			switch c {
			case '\'', '"':
				inStr = c
			case '(':
				depth++
			case ')':
				if depth == 0 {
					argsStr := s[start:i]
					i++
					// reuse splitArgs on substring
					return splitArgs(argsStr)
				}
				depth--
			}
			i++
		}
		return nil, fmt.Errorf("unterminated call in %q", s)
	}
	parseIndex := func() (string, error) {
		if i >= len(s) || s[i] != '[' {
			return "", fmt.Errorf("expected '['")
		}
		depth := 0
		inStr := byte(0)
		start := i + 1
		i++
		for i < len(s) {
			c := s[i]
			if inStr != 0 {
				if c == inStr {
					inStr = 0
				}
				i++
				continue
			}
			switch c {
			case '\'', '"':
				inStr = c
			case '[':
				depth++
			case ']':
				if depth == 0 {
					idxExpr := strings.TrimSpace(s[start:i])
					i++
					return idxExpr, nil
				}
				depth--
			}
			i++
		}
		return "", fmt.Errorf("unterminated index in %q", s)
	}

	skipSpaces()
	// first identifier
	name, err := readIdent()
	if err != nil {
		return nil, err
	}
	skipSpaces()
	var cur Value
	// possible global function call
	if i < len(s) && s[i] == '(' {
		argStrs, err := parseArgs()
		if err != nil {
			return nil, err
		}
		var args []Value
		for _, as := range argStrs {
			v, err := e.evalAtom(as, ctx)
			if err != nil {
				return nil, err
			}
			args = append(args, v)
		}
		if fn, ok := e.Funcs[name]; ok {
			v, err := fn(args)
			if err != nil {
				return nil, err
			}
			cur = v
		} else if v0, ok := ctx[name]; ok {
			// a callable value in context
			switch cv := v0.(type) {
			case CallableValue:
				v, err := cv.Fn(args)
				if err != nil {
					return nil, err
				}
				cur = v
			default:
				return nil, fmt.Errorf("%s is not callable", name)
			}
		} else {
			return nil, fmt.Errorf("undefined function: %s", name)
		}
	} else {
		// simple variable
		if lh, ok := any(ContextRef{Ctx: ctx}).(LookupHook); ok {
			lh.OnLookup(name)
		}
		v0, ok := ctx[name]
		if !ok {
			return nil, fmt.Errorf("undefined variable: %s", name)
		}
		cur = v0
	}

	// trailers: .name [index] (call)
	for i < len(s) {
		skipSpaces()
		if i >= len(s) {
			break
		}
		if s[i] == '.' {
			i++
			skipSpaces()
			attr, err := readIdent()
			if err != nil {
				return nil, err
			}
			// attribute lookup or method binding
			if nv, ok := e.lookupOrMethod(cur, attr); ok {
				cur = nv
			} else {
				return nil, fmt.Errorf("undefined attribute: %s on %T", attr, cur)
			}
			skipSpaces()
			// optional call immediately after attribute
			if i < len(s) && s[i] == '(' {
				argStrs, err := parseArgs()
				if err != nil {
					return nil, err
				}
				var args []Value
				for _, as := range argStrs {
					v, err := e.evalAtom(as, ctx)
					if err != nil {
						return nil, err
					}
					args = append(args, v)
				}
				cv, ok := cur.(CallableValue)
				if !ok {
					return nil, fmt.Errorf("%s is not callable", attr)
				}
				v, err := cv.Fn(args)
				if err != nil {
					return nil, err
				}
				cur = v
			}
			continue
		}
		if s[i] == '[' {
			idxExpr, err := parseIndex()
			if err != nil {
				return nil, err
			}
			kv, err := e.Eval(idxExpr, ctx)
			if err != nil {
				return nil, err
			}
			// perform indexing
			switch base := cur.(type) {
			case DictValue:
				key := kv.String()
				if vv, ok := base[key]; ok {
					cur = vv
				} else {
					return nil, fmt.Errorf("key not found: %s", key)
				}
			case ListValue:
				// list/tuple indexing by integer
				idx, ok := asInt(kv)
				if !ok {
					return nil, fmt.Errorf("list index must be integer, got %T", kv)
				}
				if idx < 0 || idx >= len(base) {
					return nil, fmt.Errorf("list index out of range: %d", idx)
				}
				cur = base[idx]
			default:
				rv := reflect.ValueOf(cur)
				switch rv.Kind() {
				case reflect.Map:
					mk := reflect.ValueOf(kv.String())
					mv := rv.MapIndex(mk)
					if mv.IsValid() {
						cur = FromGo(mv.Interface())
					} else {
						return nil, fmt.Errorf("key not found: %s", kv.String())
					}
				case reflect.Slice, reflect.Array:
					idx, ok := asInt(kv)
					if !ok {
						return nil, fmt.Errorf("index must be integer, got %T", kv)
					}
					if idx < 0 || idx >= rv.Len() {
						return nil, fmt.Errorf("index out of range: %d", idx)
					}
					cur = FromGo(rv.Index(idx).Interface())
				default:
					return nil, fmt.Errorf("type %T not indexable", cur)
				}
			}
			continue
		}
		// unexpected char
		return nil, fmt.Errorf("unexpected token at %q (pos %d)", s[i:], i)
	}
	return cur, nil
}

// lookupOrMethod attempts attribute lookup, then falls back to method binding.
func (e *Evaluator) lookupOrMethod(v Value, key string) (Value, bool) {
	if vv, ok := e.lookupValue(v, key); ok {
		return vv, true
	}
	// string methods
	switch s := v.(type) {
	case StringValue:
		switch key {
		case "lower":
			return CallableValue{Fn: func(args []Value) (Value, error) {
				return StringValue(strings.ToLower(string(s))), nil
			}}, true
		case "split":
			return CallableValue{Fn: func(args []Value) (Value, error) {
				str := string(s)
				if len(args) == 0 {
					fields := strings.Fields(str)
					out := make(ListValue, 0, len(fields))
					for _, f := range fields {
						out = append(out, StringValue(f))
					}
					return out, nil
				}
				sep := args[0].String()
				parts := strings.Split(str, sep)
				out := make(ListValue, 0, len(parts))
				for _, p := range parts {
					out = append(out, StringValue(p))
				}
				return out, nil
			}}, true
		}
	case ListValue:
		// Allow .split() on lists to be a no-op identity, so templates
		// can accept either strings or lists for inputs like package lists.
		if key == "split" {
			return CallableValue{Fn: func(args []Value) (Value, error) {
				// ignore args; just return the list itself
				return s, nil
			}}, true
		}
	}
	return nil, false
}

// asInt attempts to convert a Value into an int index.
func asInt(v Value) (int, bool) {
	switch t := v.(type) {
	case IntValue:
		return int(int64(t)), true
	case StringValue:
		if t == "" {
			return 0, false
		}
		if n, err := strconv.ParseInt(string(t), 10, 64); err == nil {
			return int(n), true
		}
		return 0, false
	default:
		// Attempt reflection for numeric Go values wrapped via FromGo
		rv := reflect.ValueOf(v)
		switch rv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return int(rv.Int()), true
		}
	}
	return 0, false
}

// setVar stores a value in the current context.
func setVar(ctx Context, name string, val Value) error {
	ctx[name] = val
	return nil
}

// lookupValue retrieves a nested value by key from a Value.
func (e *Evaluator) lookupValue(v Value, key string) (Value, bool) {
	// Give containers a chance to intercept/handle the lookup first.
	if lh, ok := v.(LookupHook); ok {
		if vv, handled := lh.OnLookup(key); handled {
			return vv, true
		}
	}
	switch t := v.(type) {
	case DictValue:
		if vv, ok := t[key]; ok {
			return vv, true
		}
		return nil, false
	default:
		rv := reflect.ValueOf(v)
		switch rv.Kind() {
		case reflect.Map:
			mv := rv.MapIndex(reflect.ValueOf(key))
			if mv.IsValid() {
				return FromGo(mv.Interface()), true
			}
			return nil, false
		case reflect.Struct:
			f := rv.FieldByNameFunc(func(n string) bool { return strings.EqualFold(n, key) })
			if f.IsValid() {
				return FromGo(f.Interface()), true
			}
			return nil, false
		case reflect.Pointer, reflect.Interface:
			if rv.IsNil() {
				return nil, false
			}
			// Recurse on the underlying value (after giving the original value a chance above).
			return e.lookupValue(FromGo(rv.Elem().Interface()), key)
		}
	}
	return nil, false
}
