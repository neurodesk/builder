package jinja2

import (
	"fmt"
	"strings"
)

// Parse parses a Jinja2 template string into a Document AST.
// It recognizes text, output expressions, comments, and a subset of block
// statements: if/elif/else/endif, for/else/endfor, set, and raw/endraw.
// Expressions inside tags are preserved as raw strings.
func Parse(src string) (*Document, error) {
	p := &parser{l: newLexer([]byte(src))}
	nodes, _, _, err := p.parseNodes(map[string]bool{})
	if err != nil {
		return nil, err
	}
	return &Document{Nodes: nodes}, nil
}

type parser struct {
	l *lexer
}

// parseNodes parses until an ending statement with a name in `until` is
// encountered. If `until` is empty, parses to EOF.
func (p *parser) parseNodes(until map[string]bool) (nodes []Node, endTag, endArgs string, err error) {
	for {
		tok := p.l.nextTokenOutside()
		switch tok.kind {
		case tokEOF:
			return nodes, "", "", nil
		case tokText:
			if tok.val != "" {
				nodes = append(nodes, &TextNode{Text: tok.val})
			}
		case tokVarStart:
			expr, err := p.readUntilVarEnd()
			if err != nil {
				return nil, "", "", err
			}
			nodes = append(nodes, &OutputNode{Expr: strings.TrimSpace(expr)})
		case tokCommStart:
			if err := p.skipUntilCommentEnd(); err != nil {
				return nil, "", "", err
			}
		case tokStmtStart:
			stmt, err := p.readUntilStmtEnd()
			if err != nil {
				return nil, "", "", err
			}
			name, args := splitNameArgs(stmt)
			if len(until) > 0 && until[name] {
				return nodes, name, args, nil
			}
			switch name {
			case "raw":
				rawText, err := p.readRawUntilEndraw()
				if err != nil {
					return nil, "", "", err
				}
				nodes = append(nodes, &RawNode{Text: rawText})
			case "block":
				bn, err := p.parseBlock(args)
				if err != nil {
					return nil, "", "", err
				}
				nodes = append(nodes, bn)
			case "endblock":
				return nodes, "endblock", args, nil
			case "extends":
				en, err := parseExtends(args)
				if err != nil {
					return nil, "", "", err
				}
				nodes = append(nodes, en)
			case "include":
				in, err := parseInclude(args)
				if err != nil {
					return nil, "", "", err
				}
				nodes = append(nodes, in)
			case "set":
				n, err := parseSet(args)
				if err != nil {
					return nil, "", "", err
				}
				nodes = append(nodes, n)
			case "if":
				n, err := p.parseIf(args)
				if err != nil {
					return nil, "", "", err
				}
				nodes = append(nodes, n)
			case "for":
				n, err := p.parseFor(args)
				if err != nil {
					return nil, "", "", err
				}
				nodes = append(nodes, n)
			default:
				return nil, "", "", fmt.Errorf("unsupported statement: %q", name)
			}
		default:
			return nil, "", "", fmt.Errorf("unexpected token kind outside: %v", tok.kind)
		}
	}
}

func (p *parser) readUntilVarEnd() (string, error) {
	var b strings.Builder
	for {
		t := p.l.nextTokenInside(tokVarEnd)
		switch t.kind {
		case tokContent:
			b.WriteString(t.val)
		case tokVarEnd:
			return b.String(), nil
		case tokEOF:
			return "", fmt.Errorf("unterminated variable tag {{ ... }}")
		default:
			return "", fmt.Errorf("unexpected token inside var: %v", t.kind)
		}
	}
}

func (p *parser) readUntilStmtEnd() (string, error) {
	var b strings.Builder
	for {
		t := p.l.nextTokenInside(tokStmtEnd)
		switch t.kind {
		case tokContent:
			b.WriteString(t.val)
		case tokStmtEnd:
			return strings.TrimSpace(b.String()), nil
		case tokEOF:
			return "", fmt.Errorf("unterminated statement tag {%% ... %%}")
		default:
			return "", fmt.Errorf("unexpected token inside stmt: %v", t.kind)
		}
	}
}

func (p *parser) skipUntilCommentEnd() error {
	for {
		t := p.l.nextTokenInside(tokCommEnd)
		if t.kind == tokCommEnd {
			return nil
		}
		if t.kind == tokEOF {
			return fmt.Errorf("unterminated comment tag {# ... #}")
		}
		// ignore tokContent
	}
}

func splitNameArgs(stmt string) (name, args string) {
	s := strings.TrimSpace(stmt)
	if s == "" {
		return "", ""
	}
	// First word is name.
	i := 0
	for i < len(s) && !isSpace(s[i]) {
		i++
	}
	name = s[:i]
	args = strings.TrimSpace(s[i:])
	return
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

func (p *parser) readRawUntilEndraw() (string, error) {
	var out strings.Builder
	for {
		// Read until next statement start
		t := p.l.nextTokenOutside()
		switch t.kind {
		case tokEOF:
			return "", fmt.Errorf("unterminated raw block; expected {%% endraw %%}")
		case tokText:
			out.WriteString(t.val)
		case tokVarStart:
			// Inside raw, treat everything literally. Reconstruct as {{ ... }}.
			expr, err := p.readUntilVarEnd()
			if err != nil {
				return "", err
			}
			if strings.TrimSpace(expr) == "" {
				out.WriteString("{{}}")
			} else {
				out.WriteString("{{ ")
				out.WriteString(expr)
				out.WriteString(" }}")
			}
		case tokCommStart:
			out.WriteString("{#")
			// Capture comment literally until end
			var b strings.Builder
			for {
				t2 := p.l.nextTokenInside(tokCommEnd)
				if t2.kind == tokContent {
					b.WriteString(t2.val)
					continue
				}
				if t2.kind == tokCommEnd {
					break
				}
				if t2.kind == tokEOF {
					return "", fmt.Errorf("unterminated raw comment")
				}
				return "", fmt.Errorf("unexpected token in raw comment: %v", t2.kind)
			}
			if strings.TrimSpace(b.String()) == "" {
				out.WriteString("#}")
			} else {
				out.WriteString(" ")
				out.WriteString(b.String())
				out.WriteString(" #}")
			}
		case tokStmtStart:
			stmt, err := p.readUntilStmtEnd()
			if err != nil {
				return "", err
			}
			name, args := splitNameArgs(stmt)
			if name == "endraw" {
				if strings.TrimSpace(args) != "" {
					return "", fmt.Errorf("endraw takes no arguments")
				}
				return out.String(), nil
			}
			// Not endraw: keep literally as {% ... %}
			out.WriteString("{% ")
			out.WriteString(stmt)
			out.WriteString(" %}")
		default:
			return "", fmt.Errorf("unexpected token in raw: %v", t.kind)
		}
	}
}

func parseSet(args string) (*SetNode, error) {
	// Split on the first '='
	i := strings.IndexRune(args, '=')
	if i < 0 {
		return nil, fmt.Errorf("invalid set statement, expected '=': %q", args)
	}
	name := strings.TrimSpace(args[:i])
	expr := strings.TrimSpace(args[i+1:])
	if name == "" || expr == "" {
		return nil, fmt.Errorf("invalid set statement, name or expr empty")
	}
	return &SetNode{Name: name, Expr: expr}, nil
}

func (p *parser) parseIf(cond string) (*IfNode, error) {
	n := &IfNode{Cond: strings.TrimSpace(cond)}
	body, endTag, endArgs, err := p.parseNodes(map[string]bool{"elif": true, "else": true, "endif": true})
	if err != nil {
		return nil, err
	}
	n.Then = body
	for endTag == "elif" {
		branch := ElifBranch{Cond: strings.TrimSpace(endArgs)}
		body, endTag, endArgs, err = p.parseNodes(map[string]bool{"elif": true, "else": true, "endif": true})
		if err != nil {
			return nil, err
		}
		branch.Body = body
		n.Elifs = append(n.Elifs, branch)
	}
	if endTag == "else" {
		elseBody, endTag2, _, err := p.parseNodes(map[string]bool{"endif": true})
		if err != nil {
			return nil, err
		}
		if endTag2 != "endif" {
			return nil, fmt.Errorf("expected endif after else, got %q", endTag2)
		}
		n.Else = elseBody
		return n, nil
	}
	if endTag != "endif" {
		return nil, fmt.Errorf("expected endif, got %q", endTag)
	}
	return n, nil
}

func (p *parser) parseFor(args string) (*ForNode, error) {
	// Expect: target in iterable
	parts := strings.SplitN(args, " in ", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid for statement, expected 'target in iterable': %q", args)
	}
	target := strings.TrimSpace(parts[0])
	iterable := strings.TrimSpace(parts[1])
	if target == "" || iterable == "" {
		return nil, fmt.Errorf("invalid for statement, empty target or iterable")
	}
	n := &ForNode{Target: target, Iterable: iterable}
	body, endTag, _, err := p.parseNodes(map[string]bool{"else": true, "endfor": true})
	if err != nil {
		return nil, err
	}
	n.Body = body
	if endTag == "else" {
		elseBody, endTag2, _, err := p.parseNodes(map[string]bool{"endfor": true})
		if err != nil {
			return nil, err
		}
		if endTag2 != "endfor" {
			return nil, fmt.Errorf("expected endfor after else, got %q", endTag2)
		}
		n.Else = elseBody
		return n, nil
	}
	if endTag != "endfor" {
		return nil, fmt.Errorf("expected endfor, got %q", endTag)
	}
	return n, nil
}

func (p *parser) parseBlock(args string) (*BlockNode, error) {
	name := strings.TrimSpace(args)
	if name == "" {
		return nil, fmt.Errorf("block requires a name")
	}
	body, endTag, endArgs, err := p.parseNodes(map[string]bool{"endblock": true})
	if err != nil {
		return nil, err
	}
	if endTag != "endblock" {
		return nil, fmt.Errorf("expected endblock for block %q, got %q", name, endTag)
	}
	endName := strings.TrimSpace(endArgs)
	if endName != "" && endName != name {
		return nil, fmt.Errorf("endblock name %q does not match block name %q", endName, name)
	}
	return &BlockNode{Name: name, Body: body}, nil
}

func parseExtends(args string) (*ExtendsNode, error) {
	t, ok := parseQuoted(args)
	if !ok || t == "" {
		return nil, fmt.Errorf("extends expects a quoted template name")
	}
	return &ExtendsNode{Template: t}, nil
}

func parseInclude(args string) (*IncludeNode, error) {
	t, ok := parseQuoted(args)
	if !ok || t == "" {
		return nil, fmt.Errorf("include expects a quoted template name")
	}
	return &IncludeNode{Template: t}, nil
}

func parseQuoted(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return "", false
	}
	q := s[0]
	if (q != '"' && q != '\'') || s[len(s)-1] != q {
		return "", false
	}
	inner := s[1 : len(s)-1]
	return inner, true
}
