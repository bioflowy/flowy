// Package eval implements the WDL 1.2 expression evaluator.
// Eval() is the single entry point; it dispatches over all 14 ast.Expr concrete types.
package eval

import (
	"fmt"
	"math"

	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/lexer"
	"github.com/bioflowy/flowy/wdl/types"
	"github.com/bioflowy/flowy/wdl/values"
)

// Eval evaluates a WDL expression in the given environment and execution context.
// Returns the resulting value or an EvalError describing the failure.
func Eval(expr ast.Expr, env *Env, ctx ExecContext) (values.Value, error) {
	switch e := expr.(type) {
	case *ast.BoolLit:
		return values.BooleanValue{Type: types.BooleanType{}, Value: e.Value}, nil
	case *ast.IntLit:
		return values.IntValue{Type: types.IntType{}, Value: e.Value}, nil
	case *ast.FloatLit:
		return values.FloatValue{Type: types.FloatType{}, Value: e.Value}, nil
	case *ast.NullLit:
		return values.NullValue{}, nil
	case *ast.StringExpr:
		s, err := evalString(e.Parts, env, ctx)
		if err != nil {
			return nil, err
		}
		return values.StringValue{Type: types.StringType{}, Value: s}, nil
	case *ast.CommandExpr:
		return evalCommand(e, env, ctx)
	case *ast.ArrayExpr:
		items := make([]values.Value, len(e.Items))
		for i, item := range e.Items {
			v, err := Eval(item, env, ctx)
			if err != nil {
				return nil, err
			}
			items[i] = v
		}
		var itemType types.Type = types.AnyType{}
		if len(items) > 0 {
			itemType = items[0].ValueType()
		}
		return values.ArrayValue{
			Type:  types.ArrayType{ItemType: itemType},
			Items: items,
		}, nil
	case *ast.MapExpr:
		entries := make([]values.MapEntry, len(e.Items))
		for i, item := range e.Items {
			k, err := Eval(item.Key, env, ctx)
			if err != nil {
				return nil, err
			}
			v, err := Eval(item.Value, env, ctx)
			if err != nil {
				return nil, err
			}
			entries[i] = values.MapEntry{Key: k, Value: v}
		}
		var keyType, valType types.Type = types.AnyType{}, types.AnyType{}
		if len(entries) > 0 {
			keyType = entries[0].Key.ValueType()
			valType = entries[0].Value.ValueType()
		}
		return values.MapValue{
			Type:    types.MapType{KeyType: keyType, ValType: valType},
			Entries: entries,
		}, nil
	case *ast.PairExpr:
		left, err := Eval(e.Left, env, ctx)
		if err != nil {
			return nil, err
		}
		right, err := Eval(e.Right, env, ctx)
		if err != nil {
			return nil, err
		}
		return values.PairValue{
			Type:  types.PairType{LeftType: left.ValueType(), RightType: right.ValueType()},
			Left:  left,
			Right: right,
		}, nil
	case *ast.StructExpr:
		fields := make(map[string]values.Value, len(e.Members))
		for _, m := range e.Members {
			v, err := Eval(m.Value, env, ctx)
			if err != nil {
				return nil, err
			}
			fields[m.Name] = v
		}
		return values.StructValue{
			Type:   types.StructType{Name: e.TypeName},
			Fields: fields,
		}, nil
	case *ast.IfThenElse:
		cond, err := Eval(e.Cond, env, ctx)
		if err != nil {
			return nil, err
		}
		bv, ok := cond.(values.BooleanValue)
		if !ok {
			return nil, &EvalError{Pos: e.Pos, Message: fmt.Sprintf("if condition must be Boolean, got %T", cond)}
		}
		if bv.Value {
			return Eval(e.Then, env, ctx)
		}
		return Eval(e.Else, env, ctx)
	case *ast.BinaryExpr:
		return evalBinary(e, env, ctx)
	case *ast.UnaryExpr:
		return evalUnary(e, env, ctx)
	case *ast.IndexExpr:
		return evalIndex(e, env, ctx)
	case *ast.GetExpr:
		return evalGet(e, env, ctx)
	case *ast.ApplyExpr:
		return evalApply(e, env, ctx)
	case *ast.IdentExpr:
		v, ok := env.Lookup(e.Name)
		if !ok {
			return nil, &EvalError{Pos: e.Pos, Message: fmt.Sprintf("undefined variable %q", e.Name)}
		}
		return v, nil
	default:
		return nil, &EvalError{Message: "unknown expression type"}
	}
}

// evalBinary evaluates a binary operator expression.
func evalBinary(e *ast.BinaryExpr, env *Env, ctx ExecContext) (values.Value, error) {
	left, err := Eval(e.Left, env, ctx)
	if err != nil {
		return nil, err
	}
	right, err := Eval(e.Right, env, ctx)
	if err != nil {
		return nil, err
	}

	switch e.Op {
	case lexer.TokPlus, lexer.TokMinus, lexer.TokStar, lexer.TokSlash, lexer.TokPercent:
		return evalArith(e.Op, left, right, e.Pos)

	case lexer.TokStarStar:
		// ** always promotes to Float
		lf, err := toFloat(left, e.Pos)
		if err != nil {
			return nil, err
		}
		rf, err := toFloat(right, e.Pos)
		if err != nil {
			return nil, err
		}
		return values.FloatValue{Type: types.FloatType{}, Value: math.Pow(lf, rf)}, nil

	case lexer.TokEqEq, lexer.TokNotEq, lexer.TokLt, lexer.TokLtEq, lexer.TokGt, lexer.TokGtEq:
		return evalComparison(e.Op, left, right, e.Pos)

	case lexer.TokAnd:
		lb, ok1 := left.(values.BooleanValue)
		rb, ok2 := right.(values.BooleanValue)
		if !ok1 || !ok2 {
			return nil, &EvalError{Pos: e.Pos, Message: "operator && requires Boolean operands"}
		}
		return values.BooleanValue{Type: types.BooleanType{}, Value: lb.Value && rb.Value}, nil

	case lexer.TokOr:
		lb, ok1 := left.(values.BooleanValue)
		rb, ok2 := right.(values.BooleanValue)
		if !ok1 || !ok2 {
			return nil, &EvalError{Pos: e.Pos, Message: "operator || requires Boolean operands"}
		}
		return values.BooleanValue{Type: types.BooleanType{}, Value: lb.Value || rb.Value}, nil

	default:
		return nil, &EvalError{Pos: e.Pos, Message: fmt.Sprintf("unknown binary operator %d", e.Op)}
	}
}

// evalArith evaluates arithmetic operators +, -, *, /, % with WDL promotion rules.
func evalArith(op int, left, right values.Value, pos ast.Pos) (values.Value, error) {
	// WDL 1.2: + operator supports String concatenation.
	// If either operand is a String (or one is String and the other is None),
	// treat as string concatenation. None propagates: String + None = None.
	if op == lexer.TokPlus {
		_, leftIsNull := left.(values.NullValue)
		_, rightIsNull := right.(values.NullValue)
		ls, leftIsString := left.(values.StringValue)
		rs, rightIsString := right.(values.StringValue)
		if leftIsString && rightIsString {
			return values.StringValue{Type: types.StringType{}, Value: ls.Value + rs.Value}, nil
		}
		if (leftIsString && rightIsNull) || (leftIsNull && rightIsString) {
			return values.NullValue{}, nil
		}
	}

	// Determine if either operand is Float (mixed arithmetic promotes to Float).
	lf, leftIsFloat := asFloat(left)
	rf, rightIsFloat := asFloat(right)
	li, leftIsInt := asInt(left)
	ri, rightIsInt := asInt(right)

	if !leftIsFloat && !leftIsInt {
		return nil, &EvalError{Pos: pos, Message: fmt.Sprintf("arithmetic operand must be Int or Float, got %T", left)}
	}
	if !rightIsFloat && !rightIsInt {
		return nil, &EvalError{Pos: pos, Message: fmt.Sprintf("arithmetic operand must be Int or Float, got %T", right)}
	}

	if leftIsFloat || rightIsFloat {
		// Float arithmetic
		if leftIsInt {
			lf = float64(li)
		}
		if rightIsInt {
			rf = float64(ri)
		}
		switch op {
		case lexer.TokPlus:
			return values.FloatValue{Type: types.FloatType{}, Value: lf + rf}, nil
		case lexer.TokMinus:
			return values.FloatValue{Type: types.FloatType{}, Value: lf - rf}, nil
		case lexer.TokStar:
			return values.FloatValue{Type: types.FloatType{}, Value: lf * rf}, nil
		case lexer.TokSlash:
			return values.FloatValue{Type: types.FloatType{}, Value: lf / rf}, nil
		case lexer.TokPercent:
			return values.FloatValue{Type: types.FloatType{}, Value: math.Mod(lf, rf)}, nil
		}
	}

	// Integer arithmetic
	switch op {
	case lexer.TokPlus:
		return values.IntValue{Type: types.IntType{}, Value: li + ri}, nil
	case lexer.TokMinus:
		return values.IntValue{Type: types.IntType{}, Value: li - ri}, nil
	case lexer.TokStar:
		return values.IntValue{Type: types.IntType{}, Value: li * ri}, nil
	case lexer.TokSlash:
		return values.IntValue{Type: types.IntType{}, Value: li / ri}, nil
	case lexer.TokPercent:
		return values.IntValue{Type: types.IntType{}, Value: li % ri}, nil
	}
	return nil, &EvalError{Pos: pos, Message: fmt.Sprintf("unknown arithmetic op %d", op)}
}

// evalComparison evaluates comparison operators.
func evalComparison(op int, left, right values.Value, pos ast.Pos) (values.Value, error) {
	// Compare numeric values with promotion
	lf, leftIsFloat := asFloat(left)
	rf, rightIsFloat := asFloat(right)
	li, leftIsInt := asInt(left)
	ri, rightIsInt := asInt(right)

	var result bool
	if (leftIsFloat || leftIsInt) && (rightIsFloat || rightIsInt) {
		// Numeric comparison — promote to float if either is float
		var lv, rv float64
		if leftIsFloat {
			lv = lf
		} else {
			lv = float64(li)
		}
		if rightIsFloat {
			rv = rf
		} else {
			rv = float64(ri)
		}
		switch op {
		case lexer.TokEqEq:
			result = lv == rv
		case lexer.TokNotEq:
			result = lv != rv
		case lexer.TokLt:
			result = lv < rv
		case lexer.TokLtEq:
			result = lv <= rv
		case lexer.TokGt:
			result = lv > rv
		case lexer.TokGtEq:
			result = lv >= rv
		}
		return values.BooleanValue{Type: types.BooleanType{}, Value: result}, nil
	}

	// Boolean equality
	if lb, ok1 := left.(values.BooleanValue); ok1 {
		if rb, ok2 := right.(values.BooleanValue); ok2 {
			switch op {
			case lexer.TokEqEq:
				result = lb.Value == rb.Value
			case lexer.TokNotEq:
				result = lb.Value != rb.Value
			default:
				return nil, &EvalError{Pos: pos, Message: "Boolean only supports == and != comparisons"}
			}
			return values.BooleanValue{Type: types.BooleanType{}, Value: result}, nil
		}
	}

	// String equality
	if ls, ok1 := left.(values.StringValue); ok1 {
		if rs, ok2 := right.(values.StringValue); ok2 {
			switch op {
			case lexer.TokEqEq:
				result = ls.Value == rs.Value
			case lexer.TokNotEq:
				result = ls.Value != rs.Value
			case lexer.TokLt:
				result = ls.Value < rs.Value
			case lexer.TokLtEq:
				result = ls.Value <= rs.Value
			case lexer.TokGt:
				result = ls.Value > rs.Value
			case lexer.TokGtEq:
				result = ls.Value >= rs.Value
			}
			return values.BooleanValue{Type: types.BooleanType{}, Value: result}, nil
		}
	}

	// File equality
	if lf, ok1 := left.(values.FileValue); ok1 {
		if rf, ok2 := right.(values.FileValue); ok2 {
			switch op {
			case lexer.TokEqEq:
				result = lf.Value == rf.Value
			case lexer.TokNotEq:
				result = lf.Value != rf.Value
			default:
				return nil, &EvalError{Pos: pos, Message: "File only supports == and != comparisons"}
			}
			return values.BooleanValue{Type: types.BooleanType{}, Value: result}, nil
		}
	}

	// Directory equality
	if ld, ok1 := left.(values.DirectoryValue); ok1 {
		if rd, ok2 := right.(values.DirectoryValue); ok2 {
			switch op {
			case lexer.TokEqEq:
				result = ld.Value == rd.Value
			case lexer.TokNotEq:
				result = ld.Value != rd.Value
			default:
				return nil, &EvalError{Pos: pos, Message: "Directory only supports == and != comparisons"}
			}
			return values.BooleanValue{Type: types.BooleanType{}, Value: result}, nil
		}
	}

	// Deep equality for compound types (Array, Map, Pair)
	if op == lexer.TokEqEq || op == lexer.TokNotEq {
		eq, err := valuesEqual(left, right)
		if err == nil {
			if op == lexer.TokNotEq {
				eq = !eq
			}
			return values.BooleanValue{Type: types.BooleanType{}, Value: eq}, nil
		}
	}

	return nil, &EvalError{Pos: pos, Message: fmt.Sprintf("cannot compare %T and %T", left, right)}
}

// valuesEqual performs deep equality comparison for compound WDL values.
func valuesEqual(left, right values.Value) (bool, error) {
	switch l := left.(type) {
	case values.ArrayValue:
		r, ok := right.(values.ArrayValue)
		if !ok {
			return false, fmt.Errorf("type mismatch")
		}
		if len(l.Items) != len(r.Items) {
			return false, nil
		}
		for i := range l.Items {
			eq, err := valuesEqual(l.Items[i], r.Items[i])
			if err != nil {
				return false, err
			}
			if !eq {
				return false, nil
			}
		}
		return true, nil
	case values.MapValue:
		r, ok := right.(values.MapValue)
		if !ok {
			return false, fmt.Errorf("type mismatch")
		}
		if len(l.Entries) != len(r.Entries) {
			return false, nil
		}
		for i := range l.Entries {
			keq, err := valuesEqual(l.Entries[i].Key, r.Entries[i].Key)
			if err != nil {
				return false, err
			}
			veq, err := valuesEqual(l.Entries[i].Value, r.Entries[i].Value)
			if err != nil {
				return false, err
			}
			if !keq || !veq {
				return false, nil
			}
		}
		return true, nil
	case values.PairValue:
		r, ok := right.(values.PairValue)
		if !ok {
			return false, fmt.Errorf("type mismatch")
		}
		leq, err := valuesEqual(l.Left, r.Left)
		if err != nil {
			return false, err
		}
		req, err := valuesEqual(l.Right, r.Right)
		if err != nil {
			return false, err
		}
		return leq && req, nil
	case values.IntValue:
		if r, ok := right.(values.IntValue); ok {
			return l.Value == r.Value, nil
		}
		if r, ok := right.(values.FloatValue); ok {
			return float64(l.Value) == r.Value, nil
		}
		return false, fmt.Errorf("type mismatch")
	case values.FloatValue:
		if r, ok := right.(values.FloatValue); ok {
			return l.Value == r.Value, nil
		}
		if r, ok := right.(values.IntValue); ok {
			return l.Value == float64(r.Value), nil
		}
		return false, fmt.Errorf("type mismatch")
	case values.StringValue:
		if r, ok := right.(values.StringValue); ok {
			return l.Value == r.Value, nil
		}
		return false, fmt.Errorf("type mismatch")
	case values.BooleanValue:
		if r, ok := right.(values.BooleanValue); ok {
			return l.Value == r.Value, nil
		}
		return false, fmt.Errorf("type mismatch")
	case values.FileValue:
		if r, ok := right.(values.FileValue); ok {
			return l.Value == r.Value, nil
		}
		return false, fmt.Errorf("type mismatch")
	case values.DirectoryValue:
		if r, ok := right.(values.DirectoryValue); ok {
			return l.Value == r.Value, nil
		}
		return false, fmt.Errorf("type mismatch")
	case values.NullValue:
		if _, ok := right.(values.NullValue); ok {
			return true, nil
		}
		return false, nil
	default:
		return false, fmt.Errorf("unsupported type %T", left)
	}
}

// evalUnary evaluates a unary operator expression.
func evalUnary(e *ast.UnaryExpr, env *Env, ctx ExecContext) (values.Value, error) {
	operand, err := Eval(e.Operand, env, ctx)
	if err != nil {
		return nil, err
	}
	switch e.Op {
	case lexer.TokMinus:
		switch v := operand.(type) {
		case values.IntValue:
			return values.IntValue{Type: types.IntType{}, Value: -v.Value}, nil
		case values.FloatValue:
			return values.FloatValue{Type: types.FloatType{}, Value: -v.Value}, nil
		default:
			return nil, &EvalError{Pos: e.Pos, Message: fmt.Sprintf("unary minus requires Int or Float, got %T", operand)}
		}
	case lexer.TokNot:
		bv, ok := operand.(values.BooleanValue)
		if !ok {
			return nil, &EvalError{Pos: e.Pos, Message: fmt.Sprintf("unary ! requires Boolean, got %T", operand)}
		}
		return values.BooleanValue{Type: types.BooleanType{}, Value: !bv.Value}, nil
	default:
		return nil, &EvalError{Pos: e.Pos, Message: fmt.Sprintf("unknown unary operator %d", e.Op)}
	}
}

// evalGet evaluates a member access expression (struct.field or pair.left/right).
func evalGet(e *ast.GetExpr, env *Env, ctx ExecContext) (values.Value, error) {
	obj, err := Eval(e.Object, env, ctx)
	if err != nil {
		return nil, err
	}
	switch v := obj.(type) {
	case values.NullValue:
		// Null propagation: field access on None returns None.
		// This supports conditional call outputs (e.g. select_first([call.output])).
		return values.NullValue{}, nil
	case values.StructValue:
		val, ok := v.Fields[e.Member]
		if !ok {
			return nil, &EvalError{Pos: e.Pos, Message: fmt.Sprintf("struct has no field %q", e.Member)}
		}
		return val, nil
	case values.PairValue:
		switch e.Member {
		case "left":
			return v.Left, nil
		case "right":
			return v.Right, nil
		default:
			return nil, &EvalError{Pos: e.Pos, Message: fmt.Sprintf("pair has no field %q (use 'left' or 'right')", e.Member)}
		}
	default:
		return nil, &EvalError{Pos: e.Pos, Message: fmt.Sprintf("cannot access field %q on %T", e.Member, obj)}
	}
}

// evalIndex evaluates an index access expression (array[i] or map[k]).
func evalIndex(e *ast.IndexExpr, env *Env, ctx ExecContext) (values.Value, error) {
	obj, err := Eval(e.Collection, env, ctx)
	if err != nil {
		return nil, err
	}
	idx, err := Eval(e.Index, env, ctx)
	if err != nil {
		return nil, err
	}
	switch v := obj.(type) {
	case values.ArrayValue:
		i, ok := idx.(values.IntValue)
		if !ok {
			return nil, &EvalError{Pos: e.Pos, Message: "array index must be Int"}
		}
		if i.Value < 0 || int(i.Value) >= len(v.Items) {
			return nil, &EvalError{Pos: e.Pos, Message: fmt.Sprintf("array index %d out of bounds (len %d)", i.Value, len(v.Items))}
		}
		return v.Items[i.Value], nil
	case values.MapValue:
		for _, entry := range v.Entries {
			if primitiveEqual(entry.Key, idx) {
				return entry.Value, nil
			}
		}
		return nil, &EvalError{Pos: e.Pos, Message: "map key not found"}
	default:
		return nil, &EvalError{Pos: e.Pos, Message: fmt.Sprintf("cannot index into %T", obj)}
	}
}

// primitiveEqual compares two primitive WDL values by their concrete Value fields.
// Returns false for compound types (not valid map keys per WDL spec).
func primitiveEqual(a, b values.Value) bool {
	switch av := a.(type) {
	case values.BooleanValue:
		if bv, ok := b.(values.BooleanValue); ok {
			return av.Value == bv.Value
		}
	case values.IntValue:
		if bv, ok := b.(values.IntValue); ok {
			return av.Value == bv.Value
		}
	case values.FloatValue:
		if bv, ok := b.(values.FloatValue); ok {
			return av.Value == bv.Value
		}
	case values.StringValue:
		if bv, ok := b.(values.StringValue); ok {
			return av.Value == bv.Value
		}
	case values.FileValue:
		if bv, ok := b.(values.FileValue); ok {
			return av.Value == bv.Value
		}
	}
	return false
}

// asFloat returns the float64 value of a FloatValue, and whether it was a FloatValue.
func asFloat(v values.Value) (float64, bool) {
	fv, ok := v.(values.FloatValue)
	if ok {
		return fv.Value, true
	}
	return 0, false
}

// asInt returns the int64 value of an IntValue, and whether it was an IntValue.
func asInt(v values.Value) (int64, bool) {
	iv, ok := v.(values.IntValue)
	if ok {
		return iv.Value, true
	}
	return 0, false
}

// toFloat converts an IntValue or FloatValue to float64, returning an error for other types.
func toFloat(v values.Value, pos ast.Pos) (float64, error) {
	switch tv := v.(type) {
	case values.FloatValue:
		return tv.Value, nil
	case values.IntValue:
		return float64(tv.Value), nil
	default:
		return 0, &EvalError{Pos: pos, Message: fmt.Sprintf("** operand must be Int or Float, got %T", v)}
	}
}

// exprPos extracts the source position from any ast.Expr concrete type.
// Used by helpers that receive an ast.Expr interface and need position for error reporting.
func exprPos(expr ast.Expr) ast.Pos {
	switch e := expr.(type) {
	case *ast.BoolLit:
		return e.Pos
	case *ast.IntLit:
		return e.Pos
	case *ast.FloatLit:
		return e.Pos
	case *ast.NullLit:
		return e.Pos
	case *ast.StringExpr:
		return e.Pos
	case *ast.CommandExpr:
		return e.Pos
	case *ast.ArrayExpr:
		return e.Pos
	case *ast.MapExpr:
		return e.Pos
	case *ast.PairExpr:
		return e.Pos
	case *ast.StructExpr:
		return e.Pos
	case *ast.IfThenElse:
		return e.Pos
	case *ast.BinaryExpr:
		return e.Pos
	case *ast.UnaryExpr:
		return e.Pos
	case *ast.IndexExpr:
		return e.Pos
	case *ast.GetExpr:
		return e.Pos
	case *ast.ApplyExpr:
		return e.Pos
	case *ast.IdentExpr:
		return e.Pos
	default:
		return ast.Pos{}
	}
}
