package jinja2

import (
    "bytes"
    "fmt"
    "strings"
)

type Loader interface {
    Load(name string) (string, error)
}

type Renderer struct {
    Loader    Loader
    Evaluator *Evaluator
}

func NewRenderer(loader Loader) *Renderer {
    return &Renderer{Loader: loader, Evaluator: NewEvaluator()}
}

func (r *Renderer) Render(doc *Document, ctx Context) (string, error) {
    // Check for extends
    var parent *Document
    overrides := map[string]*BlockNode{}
    for _, n := range doc.Nodes {
        if en, ok := n.(*ExtendsNode); ok {
            if r.Loader == nil {
                return "", fmt.Errorf("extends requires a loader")
            }
            src, err := r.Loader.Load(en.Template)
            if err != nil { return "", err }
            parent, err = Parse(src)
            if err != nil { return "", err }
        }
        if bn, ok := n.(*BlockNode); ok {
            overrides[bn.Name] = bn
        }
    }
    if parent != nil {
        return r.renderWithParent(parent, overrides, ctx)
    }
    var buf bytes.Buffer
    if err := r.renderNodes(&buf, doc.Nodes, ctx, nil); err != nil {
        return "", err
    }
    return buf.String(), nil
}

func (r *Renderer) renderWithParent(parent *Document, overrides map[string]*BlockNode, ctx Context) (string, error) {
    var buf bytes.Buffer
    if err := r.renderNodes(&buf, parent.Nodes, ctx, overrides); err != nil {
        return "", err
    }
    return buf.String(), nil
}

func (r *Renderer) renderNodes(buf *bytes.Buffer, nodes []Node, ctx Context, overrides map[string]*BlockNode) error {
    for _, n := range nodes {
        switch t := n.(type) {
        case *TextNode:
            buf.WriteString(t.Text)
        case *RawNode:
            buf.WriteString(t.Text)
        case *OutputNode:
            v, err := r.Evaluator.Eval(t.Expr, ctx)
            if err != nil { return err }
            if v != nil {
                buf.WriteString(fmt.Sprintf("%v", v))
            }
        case *SetNode:
            v, err := r.Evaluator.Eval(t.Expr, ctx)
            if err != nil { return err }
            setVar(ctx, t.Name, v)
        case *IfNode:
            b, err := r.Evaluator.Truthy(t.Cond, ctx)
            if err != nil { return err }
            if b {
                if err := r.renderNodes(buf, t.Then, ctx, overrides); err != nil { return err }
                break
            }
            done := false
            for _, e := range t.Elifs {
                b, err := r.Evaluator.Truthy(e.Cond, ctx)
                if err != nil { return err }
                if b {
                    if err := r.renderNodes(buf, e.Body, ctx, overrides); err != nil { return err }
                    done = true
                    break
                }
            }
            if done { break }
            if len(t.Else) > 0 {
                if err := r.renderNodes(buf, t.Else, ctx, overrides); err != nil { return err }
            }
        case *ForNode:
            items, err := r.Evaluator.Eval(t.Iterable, ctx)
            if err != nil { return err }
            arr, err := iterate(items)
            if err != nil { return err }
            if len(arr) == 0 && len(t.Else) > 0 {
                if err := r.renderNodes(buf, t.Else, ctx, overrides); err != nil { return err }
                break
            }
            // Support one or two targets "a" or "k, v"
            trg := strings.Split(t.Target, ",")
            for i := range trg { trg[i] = strings.TrimSpace(trg[i]) }
            for _, it := range arr {
                if len(trg) == 1 {
                    setVar(ctx, trg[0], it)
                } else if len(trg) >= 2 {
                    setVar(ctx, trg[0], it)
                    // no value part unless item is pair like [k v]; keep simple
                }
                if err := r.renderNodes(buf, t.Body, ctx, overrides); err != nil { return err }
            }
        case *BlockNode:
            if overrides != nil {
                if ov, ok := overrides[t.Name]; ok {
                    if err := r.renderNodes(buf, ov.Body, ctx, overrides); err != nil { return err }
                    break
                }
            }
            if err := r.renderNodes(buf, t.Body, ctx, overrides); err != nil { return err }
        case *ExtendsNode:
            // Handled at top-level.
        case *IncludeNode:
            if r.Loader == nil { return fmt.Errorf("include requires a loader") }
            src, err := r.Loader.Load(t.Template)
            if err != nil { return err }
            doc, err := Parse(src)
            if err != nil { return err }
            if err := r.renderNodes(buf, doc.Nodes, ctx, overrides); err != nil { return err }
        default:
            return fmt.Errorf("unhandled node type: %T", n)
        }
    }
    return nil
}
