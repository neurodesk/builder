package jinja2

import (
    "strings"
    "testing"
)

func TestParseTextAndOutput(t *testing.T) {
    doc, err := Parse("Hello {{ name }}!")
    if err != nil { t.Fatalf("parse error: %v", err) }
    if len(doc.Nodes) != 3 { t.Fatalf("want 3 nodes, got %d", len(doc.Nodes)) }
    if tn, ok := doc.Nodes[0].(*TextNode); !ok || tn.Text != "Hello " {
        t.Fatalf("node0 not Text('Hello '): %#v", doc.Nodes[0])
    }
    if on, ok := doc.Nodes[1].(*OutputNode); !ok || strings.TrimSpace(on.Expr) != "name" {
        t.Fatalf("node1 not Output(name): %#v", doc.Nodes[1])
    }
    if tn, ok := doc.Nodes[2].(*TextNode); !ok || tn.Text != "!" {
        t.Fatalf("node2 not Text('!'): %#v", doc.Nodes[2])
    }
}

func TestRenderSimple(t *testing.T) {
    doc, err := Parse("Hello {{ name|upper|default('Anon') }}!")
    if err != nil { t.Fatalf("parse error: %v", err) }
    r := NewRenderer(nil)
    out, err := r.Render(doc, Context{"name": "world"})
    if err != nil { t.Fatalf("render error: %v", err) }
    if out != "Hello WORLD!" { t.Fatalf("got %q", out) }
    out, err = r.Render(doc, Context{})
    if err != nil { t.Fatalf("render error: %v", err) }
    if out != "Hello Anon!" { t.Fatalf("got %q", out) }
}

func TestIfElifElse(t *testing.T) {
    tpl := "{% if a %}A{% elif b %}B{% else %}C{% endif %}"
    doc, err := Parse(tpl)
    if err != nil { t.Fatalf("parse error: %v", err) }
    r := NewRenderer(nil)
    out, _ := r.Render(doc, Context{"a": true})
    if out != "A" { t.Fatalf("a=true got %q", out) }
    out, _ = r.Render(doc, Context{"b": true})
    if out != "B" { t.Fatalf("b=true got %q", out) }
    out, _ = r.Render(doc, Context{})
    if out != "C" { t.Fatalf("else got %q", out) }
}

func TestForElse(t *testing.T) {
    tpl := "{% for x in items %}-{{ x }}{% else %}empty{% endfor %}"
    doc, err := Parse(tpl)
    if err != nil { t.Fatalf("parse error: %v", err) }
    r := NewRenderer(nil)
    out, _ := r.Render(doc, Context{"items": []int{1, 2}})
    if out != "-1-2" { t.Fatalf("got %q", out) }
    out, _ = r.Render(doc, Context{"items": []int{}})
    if out != "empty" { t.Fatalf("empty got %q", out) }
}

func TestSetAndUse(t *testing.T) {
    tpl := "{% set greeting = 'hi' %}{{ greeting }}"
    doc, err := Parse(tpl)
    if err != nil { t.Fatalf("parse error: %v", err) }
    r := NewRenderer(nil)
    out, err := r.Render(doc, Context{})
    if err != nil { t.Fatalf("render error: %v", err) }
    if out != "hi" { t.Fatalf("got %q", out) }
}

func TestRawAndComments(t *testing.T) {
    tpl := "A{# comment #}B{% raw %} {{ not_parsed }} {% endraw %}C"
    doc, err := Parse(tpl)
    if err != nil { t.Fatalf("parse error: %v", err) }
    r := NewRenderer(nil)
    out, err := r.Render(doc, Context{})
    if err != nil { t.Fatalf("render error: %v", err) }
    // Raw block outputs literally; comment removed (whitespace may vary)
    if !(strings.Contains(out, "{{") && strings.Contains(out, "not_parsed") && strings.Contains(out, "}}")) || out[0:1] != "A" || !strings.Contains(out, "B") || !strings.HasSuffix(out, "C") {
        t.Fatalf("unexpected raw/comment rendering: %q", out)
    }
}

func TestInclude(t *testing.T) {
    tpl := "X[{% include 'p' %}]Y"
    doc, err := Parse(tpl)
    if err != nil { t.Fatalf("parse error: %v", err) }
    ldr := MemoryLoader{"p": "P{{ x }}"}
    r := NewRenderer(ldr)
    out, err := r.Render(doc, Context{"x": 5})
    if err != nil { t.Fatalf("render error: %v", err) }
    if out != "X[P5]Y" { t.Fatalf("got %q", out) }
}

func TestExtendsAndBlocks(t *testing.T) {
    base := "Header-[{% block content %}Base{% endblock %}]-Footer"
    child := "{% extends 'base' %}{% block content %}Child {{ name }}{% endblock %}"
    ldr := MemoryLoader{"base": base}
    childDoc, err := Parse(child)
    if err != nil { t.Fatalf("child parse error: %v", err) }
    r := NewRenderer(ldr)
    out, err := r.Render(childDoc, Context{"name": "Neo"})
    if err != nil { t.Fatalf("render error: %v", err) }
    want := "Header-[Child Neo]-Footer"
    if out != want { t.Fatalf("want %q got %q", want, out) }
}

func TestPretty(t *testing.T) {
    doc, err := Parse("A{{ x }}B")
    if err != nil { t.Fatalf("parse error: %v", err) }
    s := Pretty(doc)
    if !strings.Contains(s, "Document") || !strings.Contains(s, "Output(") {
        t.Fatalf("pretty printer missing expected content:\n%s", s)
    }
}
