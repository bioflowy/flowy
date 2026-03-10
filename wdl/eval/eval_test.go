package eval

import (
	"testing"

	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/lexer"
	"github.com/bioflowy/flowy/wdl/types"
	"github.com/bioflowy/flowy/wdl/values"
)

// TestEvalBinary tests binary operator evaluation (EXPR-01).
// Covers: +, -, *, /, %, **, ==, !=, <, <=, >, >=, &&, ||
func TestEvalBinary(t *testing.T) {
	env := NewEnv(nil)
	ctx := ExecContext{}

	cases := []struct {
		name string
		expr ast.Expr
		want values.Value
	}{
		{
			name: "int + int",
			expr: &ast.BinaryExpr{Op: lexer.TokPlus, Left: &ast.IntLit{Value: 2}, Right: &ast.IntLit{Value: 3}},
			want: values.IntValue{Type: types.IntType{}, Value: 5},
		},
		{
			name: "int + float promotes to float",
			expr: &ast.BinaryExpr{Op: lexer.TokPlus, Left: &ast.IntLit{Value: 2}, Right: &ast.FloatLit{Value: 3.0}},
			want: values.FloatValue{Type: types.FloatType{}, Value: 5.0},
		},
		{
			name: "float + int promotes to float",
			expr: &ast.BinaryExpr{Op: lexer.TokPlus, Left: &ast.FloatLit{Value: 1.5}, Right: &ast.IntLit{Value: 2}},
			want: values.FloatValue{Type: types.FloatType{}, Value: 3.5},
		},
		{
			name: "int - int",
			expr: &ast.BinaryExpr{Op: lexer.TokMinus, Left: &ast.IntLit{Value: 10}, Right: &ast.IntLit{Value: 4}},
			want: values.IntValue{Type: types.IntType{}, Value: 6},
		},
		{
			name: "int * int",
			expr: &ast.BinaryExpr{Op: lexer.TokStar, Left: &ast.IntLit{Value: 3}, Right: &ast.IntLit{Value: 7}},
			want: values.IntValue{Type: types.IntType{}, Value: 21},
		},
		{
			name: "int / int truncates",
			expr: &ast.BinaryExpr{Op: lexer.TokSlash, Left: &ast.IntLit{Value: 7}, Right: &ast.IntLit{Value: 2}},
			want: values.IntValue{Type: types.IntType{}, Value: 3},
		},
		{
			name: "int % int",
			expr: &ast.BinaryExpr{Op: lexer.TokPercent, Left: &ast.IntLit{Value: 7}, Right: &ast.IntLit{Value: 3}},
			want: values.IntValue{Type: types.IntType{}, Value: 1},
		},
		{
			name: "int ** int -> float",
			expr: &ast.BinaryExpr{Op: lexer.TokStarStar, Left: &ast.IntLit{Value: 2}, Right: &ast.IntLit{Value: 10}},
			want: values.FloatValue{Type: types.FloatType{}, Value: 1024.0},
		},
		{
			name: "== true",
			expr: &ast.BinaryExpr{Op: lexer.TokEqEq, Left: &ast.IntLit{Value: 3}, Right: &ast.IntLit{Value: 3}},
			want: values.BooleanValue{Type: types.BooleanType{}, Value: true},
		},
		{
			name: "== false",
			expr: &ast.BinaryExpr{Op: lexer.TokEqEq, Left: &ast.IntLit{Value: 3}, Right: &ast.IntLit{Value: 4}},
			want: values.BooleanValue{Type: types.BooleanType{}, Value: false},
		},
		{
			name: "!= true",
			expr: &ast.BinaryExpr{Op: lexer.TokNotEq, Left: &ast.IntLit{Value: 3}, Right: &ast.IntLit{Value: 4}},
			want: values.BooleanValue{Type: types.BooleanType{}, Value: true},
		},
		{
			name: "< true",
			expr: &ast.BinaryExpr{Op: lexer.TokLt, Left: &ast.IntLit{Value: 2}, Right: &ast.IntLit{Value: 5}},
			want: values.BooleanValue{Type: types.BooleanType{}, Value: true},
		},
		{
			name: "<= equal",
			expr: &ast.BinaryExpr{Op: lexer.TokLtEq, Left: &ast.IntLit{Value: 5}, Right: &ast.IntLit{Value: 5}},
			want: values.BooleanValue{Type: types.BooleanType{}, Value: true},
		},
		{
			name: "> true",
			expr: &ast.BinaryExpr{Op: lexer.TokGt, Left: &ast.IntLit{Value: 10}, Right: &ast.IntLit{Value: 3}},
			want: values.BooleanValue{Type: types.BooleanType{}, Value: true},
		},
		{
			name: ">= false",
			expr: &ast.BinaryExpr{Op: lexer.TokGtEq, Left: &ast.IntLit{Value: 2}, Right: &ast.IntLit{Value: 5}},
			want: values.BooleanValue{Type: types.BooleanType{}, Value: false},
		},
		{
			name: "&& both true",
			expr: &ast.BinaryExpr{Op: lexer.TokAnd, Left: &ast.BoolLit{Value: true}, Right: &ast.BoolLit{Value: true}},
			want: values.BooleanValue{Type: types.BooleanType{}, Value: true},
		},
		{
			name: "&& one false",
			expr: &ast.BinaryExpr{Op: lexer.TokAnd, Left: &ast.BoolLit{Value: true}, Right: &ast.BoolLit{Value: false}},
			want: values.BooleanValue{Type: types.BooleanType{}, Value: false},
		},
		{
			name: "|| one true",
			expr: &ast.BinaryExpr{Op: lexer.TokOr, Left: &ast.BoolLit{Value: false}, Right: &ast.BoolLit{Value: true}},
			want: values.BooleanValue{Type: types.BooleanType{}, Value: true},
		},
		{
			name: "|| both false",
			expr: &ast.BinaryExpr{Op: lexer.TokOr, Left: &ast.BoolLit{Value: false}, Right: &ast.BoolLit{Value: false}},
			want: values.BooleanValue{Type: types.BooleanType{}, Value: false},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Eval(tc.expr, env, ctx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}

	// Test literal types evaluated via BinaryExpr subexpressions
	t.Run("BoolLit true", func(t *testing.T) {
		got, err := Eval(&ast.BoolLit{Value: true}, env, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := values.BooleanValue{Type: types.BooleanType{}, Value: true}
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("IntLit", func(t *testing.T) {
		got, err := Eval(&ast.IntLit{Value: 42}, env, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := values.IntValue{Type: types.IntType{}, Value: 42}
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("FloatLit", func(t *testing.T) {
		got, err := Eval(&ast.FloatLit{Value: 3.14}, env, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := values.FloatValue{Type: types.FloatType{}, Value: 3.14}
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("NullLit", func(t *testing.T) {
		got, err := Eval(&ast.NullLit{}, env, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := values.NullValue{}
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("IdentExpr found", func(t *testing.T) {
		e := NewEnv(nil)
		e.Define("x", values.IntValue{Type: types.IntType{}, Value: 42})
		got, err := Eval(&ast.IdentExpr{Name: "x"}, e, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := values.IntValue{Type: types.IntType{}, Value: 42}
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("IdentExpr missing returns error", func(t *testing.T) {
		_, err := Eval(&ast.IdentExpr{Name: "missing"}, env, ctx)
		if err == nil {
			t.Fatal("expected error for missing ident, got nil")
		}
	})
	t.Run("ArrayExpr", func(t *testing.T) {
		expr := &ast.ArrayExpr{
			Items: []ast.Expr{
				&ast.IntLit{Value: 1},
				&ast.IntLit{Value: 2},
			},
		}
		got, err := Eval(expr, env, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		av, ok := got.(values.ArrayValue)
		if !ok {
			t.Fatalf("expected ArrayValue, got %T", got)
		}
		if len(av.Items) != 2 {
			t.Fatalf("expected 2 items, got %d", len(av.Items))
		}
		if av.Items[0] != (values.IntValue{Type: types.IntType{}, Value: 1}) {
			t.Errorf("item[0] got %v, want IntValue{1}", av.Items[0])
		}
		if av.Items[1] != (values.IntValue{Type: types.IntType{}, Value: 2}) {
			t.Errorf("item[1] got %v, want IntValue{2}", av.Items[1])
		}
	})
	t.Run("MapExpr", func(t *testing.T) {
		expr := &ast.MapExpr{
			Items: []ast.MapItem{
				{Key: &ast.IntLit{Value: 1}, Value: &ast.IntLit{Value: 10}},
				{Key: &ast.IntLit{Value: 2}, Value: &ast.IntLit{Value: 20}},
			},
		}
		got, err := Eval(expr, env, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		mv, ok := got.(values.MapValue)
		if !ok {
			t.Fatalf("expected MapValue, got %T", got)
		}
		if len(mv.Entries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(mv.Entries))
		}
	})
	t.Run("PairExpr", func(t *testing.T) {
		expr := &ast.PairExpr{
			Left:  &ast.IntLit{Value: 1},
			Right: &ast.IntLit{Value: 2},
		}
		got, err := Eval(expr, env, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		pv, ok := got.(values.PairValue)
		if !ok {
			t.Fatalf("expected PairValue, got %T", got)
		}
		if pv.Left != (values.IntValue{Type: types.IntType{}, Value: 1}) {
			t.Errorf("pair.Left got %v, want IntValue{1}", pv.Left)
		}
		if pv.Right != (values.IntValue{Type: types.IntType{}, Value: 2}) {
			t.Errorf("pair.Right got %v, want IntValue{2}", pv.Right)
		}
	})
}

// TestEvalUnary tests unary operator evaluation (EXPR-01).
// Covers: unary -, unary !
func TestEvalUnary(t *testing.T) {
	env := NewEnv(nil)
	ctx := ExecContext{}

	cases := []struct {
		name string
		expr ast.Expr
		want values.Value
	}{
		{
			name: "unary minus int",
			expr: &ast.UnaryExpr{Op: lexer.TokMinus, Operand: &ast.IntLit{Value: 5}},
			want: values.IntValue{Type: types.IntType{}, Value: -5},
		},
		{
			name: "unary minus float",
			expr: &ast.UnaryExpr{Op: lexer.TokMinus, Operand: &ast.FloatLit{Value: 3.14}},
			want: values.FloatValue{Type: types.FloatType{}, Value: -3.14},
		},
		{
			name: "unary not true -> false",
			expr: &ast.UnaryExpr{Op: lexer.TokNot, Operand: &ast.BoolLit{Value: true}},
			want: values.BooleanValue{Type: types.BooleanType{}, Value: false},
		},
		{
			name: "unary not false -> true",
			expr: &ast.UnaryExpr{Op: lexer.TokNot, Operand: &ast.BoolLit{Value: false}},
			want: values.BooleanValue{Type: types.BooleanType{}, Value: true},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Eval(tc.expr, env, ctx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}

	t.Run("unary minus on bool returns error", func(t *testing.T) {
		_, err := Eval(&ast.UnaryExpr{Op: lexer.TokMinus, Operand: &ast.BoolLit{Value: true}}, env, ctx)
		if err == nil {
			t.Fatal("expected error for unary minus on bool, got nil")
		}
	})
}

// TestEvalIfThenElse tests conditional expression evaluation (EXPR-02).
// Covers: if Cond then Then else Else
func TestEvalIfThenElse(t *testing.T) {
	env := NewEnv(nil)
	ctx := ExecContext{}

	t.Run("true branch taken", func(t *testing.T) {
		expr := &ast.IfThenElse{
			Cond: &ast.BoolLit{Value: true},
			Then: &ast.IntLit{Value: 1},
			Else: &ast.IntLit{Value: 2},
		}
		got, err := Eval(expr, env, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := values.IntValue{Type: types.IntType{}, Value: 1}
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("false branch taken", func(t *testing.T) {
		expr := &ast.IfThenElse{
			Cond: &ast.BoolLit{Value: false},
			Then: &ast.IntLit{Value: 1},
			Else: &ast.IntLit{Value: 2},
		}
		got, err := Eval(expr, env, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := values.IntValue{Type: types.IntType{}, Value: 2}
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("non-bool cond returns error", func(t *testing.T) {
		expr := &ast.IfThenElse{
			Cond: &ast.IntLit{Value: 1},
			Then: &ast.IntLit{Value: 1},
			Else: &ast.IntLit{Value: 2},
		}
		_, err := Eval(expr, env, ctx)
		if err == nil {
			t.Fatal("expected error for non-bool condition, got nil")
		}
	})
}

// TestEvalGetExpr tests member access expression evaluation (EXPR-03).
// Covers: struct.field, pair.left, pair.right
func TestEvalGetExpr(t *testing.T) {
	env := NewEnv(nil)
	ctx := ExecContext{}

	// Build a StructValue with a "name" field
	structVal := values.StructValue{
		Type: types.StructType{Name: "MyStruct"},
		Fields: map[string]values.Value{
			"name": values.IntValue{Type: types.IntType{}, Value: 99},
		},
	}
	env.Define("s", structVal)

	// Build a PairValue
	pairVal := values.PairValue{
		Type:  types.PairType{LeftType: types.IntType{}, RightType: types.IntType{}},
		Left:  values.IntValue{Type: types.IntType{}, Value: 10},
		Right: values.IntValue{Type: types.IntType{}, Value: 20},
	}
	env.Define("p", pairVal)

	t.Run("struct field access exists", func(t *testing.T) {
		expr := &ast.GetExpr{
			Object: &ast.IdentExpr{Name: "s"},
			Member: "name",
		}
		got, err := Eval(expr, env, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := values.IntValue{Type: types.IntType{}, Value: 99}
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("struct field missing returns error", func(t *testing.T) {
		expr := &ast.GetExpr{
			Object: &ast.IdentExpr{Name: "s"},
			Member: "x",
		}
		_, err := Eval(expr, env, ctx)
		if err == nil {
			t.Fatal("expected error for missing struct field, got nil")
		}
	})

	t.Run("pair left", func(t *testing.T) {
		expr := &ast.GetExpr{
			Object: &ast.IdentExpr{Name: "p"},
			Member: "left",
		}
		got, err := Eval(expr, env, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := values.IntValue{Type: types.IntType{}, Value: 10}
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("pair right", func(t *testing.T) {
		expr := &ast.GetExpr{
			Object: &ast.IdentExpr{Name: "p"},
			Member: "right",
		}
		got, err := Eval(expr, env, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := values.IntValue{Type: types.IntType{}, Value: 20}
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("pair invalid field returns error", func(t *testing.T) {
		expr := &ast.GetExpr{
			Object: &ast.IdentExpr{Name: "p"},
			Member: "other",
		}
		_, err := Eval(expr, env, ctx)
		if err == nil {
			t.Fatal("expected error for invalid pair field, got nil")
		}
	})

	t.Run("get on non-struct/pair returns error", func(t *testing.T) {
		env2 := NewEnv(nil)
		env2.Define("n", values.IntValue{Type: types.IntType{}, Value: 5})
		expr := &ast.GetExpr{
			Object: &ast.IdentExpr{Name: "n"},
			Member: "field",
		}
		_, err := Eval(expr, env2, ctx)
		if err == nil {
			t.Fatal("expected error for get on non-struct/pair, got nil")
		}
	})
}

// TestEvalIndexExpr tests index access expression evaluation (EXPR-04).
// Covers: array[i] and map[k]
func TestEvalIndexExpr(t *testing.T) {
	env := NewEnv(nil)
	ctx := ExecContext{}

	arrVal := values.ArrayValue{
		Type:  types.ArrayType{ItemType: types.IntType{}},
		Items: []values.Value{
			values.IntValue{Type: types.IntType{}, Value: 10},
			values.IntValue{Type: types.IntType{}, Value: 20},
			values.IntValue{Type: types.IntType{}, Value: 30},
		},
	}
	env.Define("arr", arrVal)

	mapVal := values.MapValue{
		Type: types.MapType{KeyType: types.IntType{}, ValType: types.IntType{}},
		Entries: []values.MapEntry{
			{Key: values.IntValue{Type: types.IntType{}, Value: 1}, Value: values.IntValue{Type: types.IntType{}, Value: 100}},
			{Key: values.IntValue{Type: types.IntType{}, Value: 2}, Value: values.IntValue{Type: types.IntType{}, Value: 200}},
		},
	}
	env.Define("m", mapVal)

	t.Run("array in-bounds index 0", func(t *testing.T) {
		expr := &ast.IndexExpr{
			Collection: &ast.IdentExpr{Name: "arr"},
			Index:      &ast.IntLit{Value: 0},
		}
		got, err := Eval(expr, env, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := values.IntValue{Type: types.IntType{}, Value: 10}
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("array in-bounds index 2", func(t *testing.T) {
		expr := &ast.IndexExpr{
			Collection: &ast.IdentExpr{Name: "arr"},
			Index:      &ast.IntLit{Value: 2},
		}
		got, err := Eval(expr, env, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := values.IntValue{Type: types.IntType{}, Value: 30}
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("array out-of-bounds returns error", func(t *testing.T) {
		expr := &ast.IndexExpr{
			Collection: &ast.IdentExpr{Name: "arr"},
			Index:      &ast.IntLit{Value: 10},
		}
		_, err := Eval(expr, env, ctx)
		if err == nil {
			t.Fatal("expected error for out-of-bounds, got nil")
		}
	})

	t.Run("array negative index returns error", func(t *testing.T) {
		expr := &ast.IndexExpr{
			Collection: &ast.IdentExpr{Name: "arr"},
			Index:      &ast.IntLit{Value: -1},
		}
		_, err := Eval(expr, env, ctx)
		if err == nil {
			t.Fatal("expected error for negative index, got nil")
		}
	})

	t.Run("map key found", func(t *testing.T) {
		expr := &ast.IndexExpr{
			Collection: &ast.IdentExpr{Name: "m"},
			Index:      &ast.IntLit{Value: 1},
		}
		got, err := Eval(expr, env, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := values.IntValue{Type: types.IntType{}, Value: 100}
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("map key not found returns error", func(t *testing.T) {
		expr := &ast.IndexExpr{
			Collection: &ast.IdentExpr{Name: "m"},
			Index:      &ast.IntLit{Value: 99},
		}
		_, err := Eval(expr, env, ctx)
		if err == nil {
			t.Fatal("expected error for missing map key, got nil")
		}
	})

	t.Run("index into non-array/map returns error", func(t *testing.T) {
		env2 := NewEnv(nil)
		env2.Define("n", values.IntValue{Type: types.IntType{}, Value: 5})
		expr := &ast.IndexExpr{
			Collection: &ast.IdentExpr{Name: "n"},
			Index:      &ast.IntLit{Value: 0},
		}
		_, err := Eval(expr, env2, ctx)
		if err == nil {
			t.Fatal("expected error for index into non-array/map, got nil")
		}
	})
}

// TestEvalApply tests function call expression evaluation (EXPR-05).
// Calls a registered stub function to verify dispatch mechanism.
func TestEvalApply(t *testing.T) {
	t.Skip("not yet implemented")
}

// TestEvalCommand tests command template expansion (EXPR-06).
// Covers: placeholder interpolation within command blocks.
// Implementation in command_test.go and eval_test.go (see Task 2).
func TestEvalCommand(t *testing.T) {
	t.Skip("not yet implemented")
}
