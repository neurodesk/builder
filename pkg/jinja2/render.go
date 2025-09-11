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
			if err != nil {
				return "", err
			}
			parent, err = Parse(src)
			if err != nil {
				return "", err
			}
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

// setVar stores a value in the current context, invoking any set hook.
func (r *Renderer) setVar(ctx Context, name string, val Value) error {
	// Notify via Value-level hook on the top-level context, if present
	if sh, ok := any(ContextRef{Ctx: ctx}).(SetHook); ok {
		return sh.OnSet(name, val)
	}
	return setVar(ctx, name, val)
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
			if err != nil {
				return err
			}
			// NoneValue.String() is empty, others produce their textual form.
			fmt.Fprintf(buf, "%s", v.String())
		case *SetNode:
			v, err := r.Evaluator.Eval(t.Expr, ctx)
			if err != nil {
				return err
			}
			// route through renderer-level setter to allow interception
			if err := r.setVar(ctx, t.Name, v); err != nil {
				return err
			}
		case *IfNode:
			b, err := r.Evaluator.Truthy(t.Cond, ctx)
			if err != nil {
				return err
			}
			if b {
				if err := r.renderNodes(buf, t.Then, ctx, overrides); err != nil {
					return err
				}
				break
			}
			done := false
			for _, e := range t.Elifs {
				b, err := r.Evaluator.Truthy(e.Cond, ctx)
				if err != nil {
					return err
				}
				if b {
					if err := r.renderNodes(buf, e.Body, ctx, overrides); err != nil {
						return err
					}
					done = true
					break
				}
			}
			if done {
				break
			}
			if len(t.Else) > 0 {
				if err := r.renderNodes(buf, t.Else, ctx, overrides); err != nil {
					return err
				}
			}
		case *ForNode:
			items, err := r.Evaluator.Eval(t.Iterable, ctx)
			if err != nil {
				return err
			}
			arr, err := iterateValue(items)
			if err != nil {
				return err
			}
			if len(arr) == 0 && len(t.Else) > 0 {
				if err := r.renderNodes(buf, t.Else, ctx, overrides); err != nil {
					return err
				}
				break
			}
			// Support one or two targets "a" or "k, v"
			trg := strings.Split(t.Target, ",")
			for i := range trg {
				trg[i] = strings.TrimSpace(trg[i])
			}
    for idx, it := range arr {
        if len(trg) == 1 {
            if err := r.setVar(ctx, trg[0], it); err != nil {
                return err
            }
        } else if len(trg) >= 2 {
            if err := r.setVar(ctx, trg[0], it); err != nil {
                return err
            }
            // no value part unless item is pair like [k v]; keep simple
        }
        // Provide minimal loop variables (loop.last)
        // Save previous loop if any
        prevLoop, hadPrevLoop := ctx["loop"]
        loopObj := DictValue{"last": BoolValue(idx == len(arr)-1)}
        if err := r.setVar(ctx, "loop", loopObj); err != nil {
            return err
        }
        if err := r.renderNodes(buf, t.Body, ctx, overrides); err != nil {
            return err
        }
        // restore previous loop
        if hadPrevLoop {
            ctx["loop"] = prevLoop
        } else {
            delete(ctx, "loop")
        }
    }
		case *BlockNode:
			if overrides != nil {
				if ov, ok := overrides[t.Name]; ok {
					if err := r.renderNodes(buf, ov.Body, ctx, overrides); err != nil {
						return err
					}
					break
				}
			}
			if err := r.renderNodes(buf, t.Body, ctx, overrides); err != nil {
				return err
			}
		case *ExtendsNode:
			// Handled at top-level.
		case *IncludeNode:
			if r.Loader == nil {
				return fmt.Errorf("include requires a loader")
			}
			src, err := r.Loader.Load(t.Template)
			if err != nil {
				return err
			}
			doc, err := Parse(src)
			if err != nil {
				return err
			}
			if err := r.renderNodes(buf, doc.Nodes, ctx, overrides); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unhandled node type: %T", n)
		}
	}
	return nil
}
