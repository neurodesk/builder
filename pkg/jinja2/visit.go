package jinja2

import (
    "bytes"
    "fmt"
)

type Visitor interface {
    Visit(n Node) error
}

func Walk(v Visitor, n Node) error {
    if err := v.Visit(n); err != nil { return err }
    switch t := n.(type) {
    case *Document:
        for _, c := range t.Nodes { if err := Walk(v, c); err != nil { return err } }
    case *IfNode:
        for _, c := range t.Then { if err := Walk(v, c); err != nil { return err } }
        for _, e := range t.Elifs {
            for _, c := range e.Body { if err := Walk(v, c); err != nil { return err } }
        }
        for _, c := range t.Else { if err := Walk(v, c); err != nil { return err } }
    case *ForNode:
        for _, c := range t.Body { if err := Walk(v, c); err != nil { return err } }
        for _, c := range t.Else { if err := Walk(v, c); err != nil { return err } }
    case *BlockNode:
        for _, c := range t.Body { if err := Walk(v, c); err != nil { return err } }
    }
    return nil
}

// Pretty returns a line-oriented string representation of the AST.
func Pretty(doc *Document) string {
    var buf bytes.Buffer
    ppNode(&buf, 0, doc)
    return buf.String()
}

func ppNode(buf *bytes.Buffer, indent int, n Node) {
    ind := func() { for i := 0; i < indent; i++ { buf.WriteByte(' ') } }
    switch t := n.(type) {
    case *Document:
        ind(); buf.WriteString("Document\n")
        for _, c := range t.Nodes { ppNode(buf, indent+2, c) }
    case *TextNode:
        ind(); fmt.Fprintf(buf, "Text(%q)\n", t.Text)
    case *OutputNode:
        ind(); fmt.Fprintf(buf, "Output(%q)\n", t.Expr)
    case *SetNode:
        ind(); fmt.Fprintf(buf, "Set(%s = %q)\n", t.Name, t.Expr)
    case *IfNode:
        ind(); fmt.Fprintf(buf, "If(%q)\n", t.Cond)
        for _, c := range t.Then { ppNode(buf, indent+2, c) }
        for _, e := range t.Elifs {
            ind(); fmt.Fprintf(buf, "Elif(%q)\n", e.Cond)
            for _, c := range e.Body { ppNode(buf, indent+2, c) }
        }
        if len(t.Else) > 0 { ind(); buf.WriteString("Else\n"); for _, c := range t.Else { ppNode(buf, indent+2, c) } }
    case *ForNode:
        ind(); fmt.Fprintf(buf, "For(%s in %q)\n", t.Target, t.Iterable)
        for _, c := range t.Body { ppNode(buf, indent+2, c) }
        if len(t.Else) > 0 { ind(); buf.WriteString("Else\n"); for _, c := range t.Else { ppNode(buf, indent+2, c) } }
    case *RawNode:
        ind(); fmt.Fprintf(buf, "Raw(%q)\n", t.Text)
    case *BlockNode:
        ind(); fmt.Fprintf(buf, "Block(%s)\n", t.Name)
        for _, c := range t.Body { ppNode(buf, indent+2, c) }
    case *ExtendsNode:
        ind(); fmt.Fprintf(buf, "Extends(%q)\n", t.Template)
    case *IncludeNode:
        ind(); fmt.Fprintf(buf, "Include(%q)\n", t.Template)
    }
}

