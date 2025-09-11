package jinja2

// Node is any AST node in a parsed Jinja2 template.
type Node interface {
	node()
}

// Document is the root node produced by Parse.
type Document struct {
	Nodes []Node
}

func (*Document) node() {}

// TextNode represents literal text between tags.
type TextNode struct {
	Text string
}

func (*TextNode) node() {}

// OutputNode represents a variable/output expression: {{ expr }}
type OutputNode struct {
	Expr string
}

func (*OutputNode) node() {}

// SetNode represents a simple assignment: {% set name = expr %}
type SetNode struct {
	Name string
	Expr string
}

func (*SetNode) node() {}

// IfNode represents an if/elif/else block.
type IfNode struct {
	Cond  string
	Then  []Node
	Elifs []ElifBranch
	Else  []Node
}

func (*IfNode) node() {}

// ElifBranch is a single elif condition with its body.
type ElifBranch struct {
	Cond string
	Body []Node
}

// ForNode represents a for loop: {% for target in iterable %}
type ForNode struct {
	Target   string
	Iterable string
	Body     []Node
	Else     []Node
}

func (*ForNode) node() {}

// RawNode represents a raw block where delimiters are not parsed.
// It is produced by: {% raw %}...{% endraw %}
type RawNode struct {
	Text string
}

func (*RawNode) node() {}

// BlockNode represents a named block for template inheritance.
type BlockNode struct {
	Name string
	Body []Node
}

func (*BlockNode) node() {}

// ExtendsNode declares that this template extends a parent template.
type ExtendsNode struct {
	Template string
}

func (*ExtendsNode) node() {}

// IncludeNode includes another template by name.
type IncludeNode struct {
	Template string
}

func (*IncludeNode) node() {}
