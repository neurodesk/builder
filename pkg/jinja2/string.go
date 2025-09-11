package jinja2

import (
	"fmt"
)

type TemplateString string

func (t TemplateString) Validate() error {
	doc, err := Parse(string(t))
	if err != nil {
		return fmt.Errorf("invalid jinja template: %w", err)
	}
	_ = doc
	return nil
}

func (t TemplateString) Render(ctx Context) (string, error) {
	doc, err := Parse(string(t))
	if err != nil {
		return "", fmt.Errorf("parsing jinja template: %w", err)
	}
	render := NewRenderer(nil)
	return render.Render(doc, ctx)
}
