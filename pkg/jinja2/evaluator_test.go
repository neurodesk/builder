package jinja2

import (
	"testing"
)

func renderHelper(t *testing.T, tpl string, ctx Context) (string, error) {
	t.Helper()
	ts := TemplateString(tpl)
	return ts.Render(ctx)
}

func TestMapIntListComparison(t *testing.T) {
	cases := []struct {
		version string
		want    string
	}{
		{"6.0.6", "Y"},
		{"6.0.5", "N"},
		{"5.0.9", "N"},
	}
	tpl := "{% if self.version.split('.') | map('int') | list >= [6, 0, 6] %}Y{% else %}N{% endif %}"
	for _, tc := range cases {
		ctx := Context{
			"self": DictValue{
				"version": StringValue(tc.version),
			},
		}
		got, err := renderHelper(t, tpl, ctx)
		if err != nil {
			t.Fatalf("render error: %v", err)
		}
		if got != tc.want {
			t.Fatalf("version %q: got %q, want %q", tc.version, got, tc.want)
		}
	}
}

func TestIndexingAndStringMethods(t *testing.T) {
	ctx := Context{
		"self": DictValue{
			"version": StringValue("1.6"),
			"urls": DictValue{
				"1.6": StringValue("https://example.com/jq-1.6"),
			},
		},
	}
	tpl := "{{ self.urls[self.version] }}"
	got, err := renderHelper(t, tpl, ctx)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	want := "https://example.com/jq-1.6"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestLoopLastAndSplit(t *testing.T) {
	ctx := Context{
		"self": DictValue{
			"p": StringValue("one two three"),
		},
	}
	tpl := "{% for x in self.p.split() %}{% if not loop.last -%}{{ x }},{% else -%}{{ x }}{% endif %}{% endfor %}"
	got, err := renderHelper(t, tpl, ctx)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	want := "one,two,three"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestListSplitIdentity(t *testing.T) {
	// Allow calling .split() on a list to return itself.
	ctx := Context{
		"self": DictValue{
			"packages": ListValue{StringValue("a"), StringValue("b")},
		},
	}
	tpl := "{% for x in self.packages.split() %}{{ x }}{% endfor %}"
	got, err := renderHelper(t, tpl, ctx)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	want := "ab"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRaiseFunction(t *testing.T) {
	ctx := Context{}
	tpl := "{{ raise('boom') }}"
	if _, err := renderHelper(t, tpl, ctx); err == nil {
		t.Fatalf("expected error from raise, got nil")
	}
}

func TestInNotInTuple(t *testing.T) {
	tpl := "{% if self.v not in ('5.0.9', '5.0.8') %}OK{% else %}BAD{% endif %}"
	ctx := Context{"self": DictValue{"v": StringValue("6.0.1")}}
	got, err := renderHelper(t, tpl, ctx)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if got != "OK" {
		t.Fatalf("got %q, want OK", got)
	}
	ctx = Context{"self": DictValue{"v": StringValue("5.0.9")}}
	got, err = renderHelper(t, tpl, ctx)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if got != "BAD" {
		t.Fatalf("got %q, want BAD", got)
	}
}

func TestLogicalAndOr(t *testing.T) {
	tpl := "{% if self.version == \"2.4.1\" or self.version == \"2.4.2\" or self.version == \"2.4.3\" %}ZIP{% else %}TAR{% endif %}"
	ctx := Context{"self": DictValue{"version": StringValue("2.4.3")}}
	got, err := renderHelper(t, tpl, ctx)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if got != "ZIP" {
		t.Fatalf("got %q, want ZIP", got)
	}
	tpl = "{% if self.a and self.b or self.c %}TRUE{% else %}FALSE{% endif %}"
	ctx = Context{"self": DictValue{
		"a": BoolValue(true),
		"b": BoolValue(false),
		"c": BoolValue(true),
	}}
	got, err = renderHelper(t, tpl, ctx)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if got != "TRUE" {
		t.Fatalf("got %q, want TRUE", got)
	}
}
