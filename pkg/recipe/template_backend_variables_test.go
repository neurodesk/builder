package recipe

import (
	"testing"

	"github.com/neurodesk/builder/pkg/common"
	"github.com/neurodesk/builder/pkg/ir"
	"github.com/neurodesk/builder/pkg/jinja2"
)

func TestEvaluateValueStringCanReferenceInjectedVariable(t *testing.T) {
	ctx := newContext(common.PkgManagerApt, "1.0.0", nil, ir.New(), nil)
	ctx.variables["self"] = &macroTemplateSelf{
		context: templateContext{
			PackageManager: common.PkgManagerApt,
		},
		params: templateParams(func(k string) (any, bool, error) {
			if k == "version" {
				return "latest", true, nil
			}
			return nil, false, nil
		}),
		template: &recipeTemplateSpec{
			Urls: map[string]jinja2.TemplateString{},
		},
	}

	got, err := ctx.evaluateValue(`{{ self.version }}`)
	if err != nil {
		t.Fatalf("evaluateValue returned error: %v", err)
	}
	if got != "latest" {
		t.Fatalf("evaluateValue = %v, want latest", got)
	}
}
