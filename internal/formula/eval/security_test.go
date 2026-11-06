package eval

import (
	"testing"
	"ww-excel/internal/formula/ast"
	"ww-excel/internal/model"
)

func TestFormulaPanicBecomesErrorAndRestoresEvaluator(t *testing.T) {
	const name = "SECURITY_TEST_PANIC"
	Register(Def{Name: name, ArrayArgs: true, Fn: func(*Args) Value { panic("injected evaluator failure") }})
	t.Cleanup(func() { mu.Lock(); delete(registry, name); mu.Unlock() })
	evaluator := Evaluator{}
	for _, run := range []struct {
		name string
		fn   func(ast.Node) Value
	}{{"scalar", evaluator.Eval}, {"reference", evaluator.Reference}} {
		t.Run(run.name, func(t *testing.T) {
			got := run.fn(ast.Call{Name: name})
			if got.Kind != KError || got.Str != model.ErrValue {
				t.Fatalf("panic result: %+v", got)
			}
			if evaluator.arrayMode != 0 {
				t.Fatal("panic leaked array mode")
			}
			got = run.fn(ast.Number{V: 42})
			if got.Kind != KNumber || got.Num != 42 {
				t.Fatal("evaluator stopped working after panic")
			}
		})
	}
}
