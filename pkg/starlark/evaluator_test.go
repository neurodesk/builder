package starlark

import (
	"testing"

	"github.com/neurodesk/builder/pkg/jinja2"
	"go.starlark.net/starlark"
)

func TestConvertToStarlark(t *testing.T) {
	tests := []struct {
		name     string
		input    jinja2.Value
		expected starlark.Value
	}{
		{
			name:     "string value",
			input:    jinja2.StringValue("hello"),
			expected: starlark.String("hello"),
		},
		{
			name:     "int value",
			input:    jinja2.IntValue(42),
			expected: starlark.MakeInt64(42),
		},
		{
			name:     "float value",
			input:    jinja2.FloatValue(3.14),
			expected: starlark.Float(3.14),
		},
		{
			name:     "bool value true",
			input:    jinja2.BoolValue(true),
			expected: starlark.Bool(true),
		},
		{
			name:     "bool value false",
			input:    jinja2.BoolValue(false),
			expected: starlark.Bool(false),
		},
		{
			name:     "none value",
			input:    jinja2.NoneValue{},
			expected: starlark.None,
		},
		{
			name:     "nil value",
			input:    nil,
			expected: starlark.None,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertToStarlark(tt.input)
			
			// Compare string representations for simplicity
			if result.String() != tt.expected.String() {
				t.Errorf("ConvertToStarlark() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestConvertFromStarlark(t *testing.T) {
	tests := []struct {
		name     string
		input    starlark.Value
		expected string // Using string comparison for simplicity
	}{
		{
			name:     "string value",
			input:    starlark.String("hello"),
			expected: "hello",
		},
		{
			name:     "int value",
			input:    starlark.MakeInt64(42),
			expected: "42",
		},
		{
			name:     "float value",
			input:    starlark.Float(3.14),
			expected: "3.14",
		},
		{
			name:     "bool value true",
			input:    starlark.Bool(true),
			expected: "true",
		},
		{
			name:     "bool value false",
			input:    starlark.Bool(false),
			expected: "false",
		},
		{
			name:     "none value",
			input:    starlark.None,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertFromStarlark(tt.input)
			
			if result.String() != tt.expected {
				t.Errorf("ConvertFromStarlark() = %v, want %v", result.String(), tt.expected)
			}
		})
	}
}

func TestListConversion(t *testing.T) {
	// Test Jinja2 list to Starlark list
	jinjaList := jinja2.ListValue{
		jinja2.StringValue("a"),
		jinja2.IntValue(1),
		jinja2.BoolValue(true),
	}

	starlarkList := ConvertToStarlark(jinjaList)
	list, ok := starlarkList.(*starlark.List)
	if !ok {
		t.Fatalf("Expected starlark.List, got %T", starlarkList)
	}

	if list.Len() != 3 {
		t.Errorf("Expected list length 3, got %d", list.Len())
	}

	// Convert back to Jinja2
	backToJinja := ConvertFromStarlark(list)
	jinjaList2, ok := backToJinja.(jinja2.ListValue)
	if !ok {
		t.Fatalf("Expected jinja2.ListValue, got %T", backToJinja)
	}

	if len(jinjaList2) != 3 {
		t.Errorf("Expected list length 3, got %d", len(jinjaList2))
	}

	if jinjaList2[0].String() != "a" {
		t.Errorf("Expected first element 'a', got %v", jinjaList2[0].String())
	}
}

func TestDictConversion(t *testing.T) {
	// Test Jinja2 dict to Starlark dict
	jinjaDict := jinja2.DictValue{
		"key1": jinja2.StringValue("value1"),
		"key2": jinja2.IntValue(42),
	}

	starlarkDict := ConvertToStarlark(jinjaDict)
	dict, ok := starlarkDict.(*starlark.Dict)
	if !ok {
		t.Fatalf("Expected starlark.Dict, got %T", starlarkDict)
	}

	if dict.Len() != 2 {
		t.Errorf("Expected dict length 2, got %d", dict.Len())
	}

	// Convert back to Jinja2
	backToJinja := ConvertFromStarlark(dict)
	jinjaDict2, ok := backToJinja.(jinja2.DictValue)
	if !ok {
		t.Fatalf("Expected jinja2.DictValue, got %T", backToJinja)
	}

	if len(jinjaDict2) != 2 {
		t.Errorf("Expected dict length 2, got %d", len(jinjaDict2))
	}

	if jinjaDict2["key1"].String() != "value1" {
		t.Errorf("Expected key1='value1', got %v", jinjaDict2["key1"].String())
	}
}

func TestEvaluatorBasic(t *testing.T) {
	eval := NewEvaluator()

	// Test simple expression evaluation
	result, err := eval.Eval("2 + 3")
	if err != nil {
		t.Fatalf("Eval error: %v", err)
	}

	if result.String() != "5" {
		t.Errorf("Expected '5', got %v", result.String())
	}
}

func TestEvaluatorWithGlobals(t *testing.T) {
	eval := NewEvaluator()

	// Set a global variable
	eval.SetGlobal("test_var", jinja2.StringValue("hello"))

	// Evaluate expression using the global
	result, err := eval.Eval("test_var + ' world'")
	if err != nil {
		t.Fatalf("Eval error: %v", err)
	}

	if result.String() != "hello world" {
		t.Errorf("Expected 'hello world', got %v", result.String())
	}
}

func TestEvaluatorScript(t *testing.T) {
	eval := NewEvaluator()

	script := `
x = 10
y = 20
result = x + y
`

	globals, err := eval.ExecString(script)
	if err != nil {
		t.Fatalf("ExecString error: %v", err)
	}

	// Check that variables were set
	if _, ok := globals["result"]; !ok {
		t.Error("Expected 'result' variable to be set")
	}

	// Check the value through the evaluator
	result, ok := eval.GetGlobal("result")
	if !ok {
		t.Error("Expected 'result' to be accessible via GetGlobal")
	}

	if result.String() != "30" {
		t.Errorf("Expected result='30', got %v", result.String())
	}
}

func TestJinja2ContextIntegration(t *testing.T) {
	eval := NewEvaluator()

	// Create a Jinja2 context
	ctx := jinja2.Context{
		"package_manager": jinja2.StringValue("apt"),
		"version":         jinja2.StringValue("1.0.0"),
		"debug":           jinja2.BoolValue(true),
	}

	// Load context into evaluator
	eval.LoadJinja2Context(ctx)

	// Use context variables in a script
	script := `
def build_cmd():
    if debug:
        return package_manager + " install -y package-" + version
    else:
        return package_manager + " install package-" + version

install_cmd = build_cmd()
`

	_, err := eval.ExecString(script)
	if err != nil {
		t.Fatalf("ExecString error: %v", err)
	}

	// Check the result
	result, ok := eval.GetGlobal("install_cmd")
	if !ok {
		t.Error("Expected 'install_cmd' to be set")
	}

	expected := "apt install -y package-1.0.0"
	if result.String() != expected {
		t.Errorf("Expected %q, got %q", expected, result.String())
	}

	// Export back to Jinja2 context
	exportedCtx := eval.ExportToJinja2Context()
	if _, ok := exportedCtx["install_cmd"]; !ok {
		t.Error("Expected 'install_cmd' to be exported to Jinja2 context")
	}
}