// Package checker - struct_check.go contains checkStructExpr and checkGetExpr,
// which validate struct literal expressions and field access expressions respectively.
package checker

import (
	"fmt"

	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/types"
)

// checkStructExpr validates a struct literal expression against the document's struct definitions.
// It checks that the struct name exists, all required fields are provided, no unknown fields are
// present, and each field value's type is compatible with the declared field type.
// Returns any TypeErrors found and the resolved StructType on success.
func checkStructExpr(expr *ast.StructExpr, pos ast.Pos, scope *Scope, structDefs map[string]*ast.StructDef) ([]error, types.Type) {
	var errs []error

	// Look up the struct definition by name.
	def, ok := structDefs[expr.TypeName]
	if !ok {
		errs = append(errs, &TypeError{
			Pos:     pos,
			Message: fmt.Sprintf("undefined struct %q", expr.TypeName),
		})
		return errs, nil
	}

	// Build a set of provided field names from the literal.
	provided := make(map[string]ast.StructMember, len(expr.Members))
	for _, m := range expr.Members {
		provided[m.Name] = m
	}

	// Build a set of declared field names from the definition.
	declared := make(map[string]types.Type, len(def.Members))
	for _, d := range def.Members {
		declared[d.Name] = d.Type
	}

	// Check for missing required fields.
	for _, d := range def.Members {
		if _, present := provided[d.Name]; !present {
			errs = append(errs, &TypeError{
				Pos:     pos,
				Message: fmt.Sprintf("struct %q missing field %q", expr.TypeName, d.Name),
			})
		}
	}

	// Check for unknown fields and validate field types.
	for _, m := range expr.Members {
		fieldType, knownField := declared[m.Name]
		if !knownField {
			errs = append(errs, &TypeError{
				Pos:     pos,
				Message: fmt.Sprintf("struct %q has no field %q", expr.TypeName, m.Name),
			})
			continue
		}

		// Determine the value's type.
		var valueType types.Type
		switch v := m.Value.(type) {
		case *ast.BoolLit:
			valueType = types.BooleanType{}
		case *ast.IntLit:
			valueType = types.IntType{}
		case *ast.FloatLit:
			valueType = types.FloatType{}
		case *ast.NullLit:
			valueType = types.AnyType{Optional: true}
		case *ast.StringExpr:
			valueType = types.StringType{}
		case *ast.IdentExpr:
			t, found := scope.lookup(v.Name)
			if !found {
				errs = append(errs, &TypeError{
					Pos:     pos,
					Message: fmt.Sprintf("undefined name %q in struct literal field %q", v.Name, m.Name),
				})
				continue
			}
			valueType = t
		default:
			// Complex expressions are deferred to Phase 4; skip type check for this field.
			continue
		}

		// Validate field value type against declared field type.
		if valueType.TypeName() != fieldType.TypeName() && !Coerces(valueType, fieldType) {
			errs = append(errs, &TypeError{
				Pos:     pos,
				Message: fmt.Sprintf("field %q: cannot use %s where %s is expected", m.Name, valueType.TypeName(), fieldType.TypeName()),
			})
		}
	}

	if len(errs) > 0 {
		return errs, nil
	}

	// Build the StructType with a deep copy of Members (per types.StructType warning).
	members := make(map[string]types.Type, len(def.Members))
	for _, d := range def.Members {
		members[d.Name] = d.Type
	}
	return nil, types.StructType{Name: expr.TypeName, Members: members}
}

// checkGetExpr validates a field access expression (e.g. p.x) where the base
// expression must resolve to a StructType in the current scope. Returns a TypeError
// if the base is not a struct or the field does not exist, otherwise the field's type.
// structDefs is used to resolve the full field map when the scope only holds a
// name-only StructType (as is the case after Pass 1 registers declared types).
func checkGetExpr(expr *ast.GetExpr, pos ast.Pos, scope *Scope, structDefs map[string]*ast.StructDef) (error, types.Type) {
	// Resolve the base expression type. Only IdentExpr base is supported;
	// complex base expressions are deferred to Phase 4.
	var baseType types.Type
	switch base := expr.Object.(type) {
	case *ast.IdentExpr:
		t, ok := scope.lookup(base.Name)
		if !ok {
			return &TypeError{
				Pos:     pos,
				Message: fmt.Sprintf("undefined name %q", base.Name),
			}, nil
		}
		baseType = t
	default:
		// Complex base expressions deferred to Phase 4.
		return nil, types.AnyType{}
	}

	// Allow field access on AnyType — this occurs when the checker cannot
	// fully infer the type (e.g., complex scatter expressions). Return AnyType
	// to avoid false type errors; runtime evaluation will catch real errors.
	if _, ok := baseType.(types.AnyType); ok {
		return nil, types.AnyType{}
	}

	// Handle Pair field access (.left / .right).
	if pt, ok := baseType.(types.PairType); ok {
		switch expr.Member {
		case "left":
			return nil, pt.LeftType
		case "right":
			return nil, pt.RightType
		default:
			return &TypeError{
				Pos:     pos,
				Message: fmt.Sprintf("Pair has no field %q (only \"left\" and \"right\")", expr.Member),
			}, nil
		}
	}

	// Type-assert base to StructType.
	st, ok := baseType.(types.StructType)
	if !ok {
		return &TypeError{
			Pos:     pos,
			Message: fmt.Sprintf("field access on non-struct type %s", baseType.TypeName()),
		}, nil
	}

	// If the StructType has no Members (e.g. registered by Pass 1 via the declared type
	// before the struct literal was resolved), fall back to structDefs for field lookup.
	members := st.Members
	if len(members) == 0 && structDefs != nil {
		if def, found := structDefs[st.Name]; found {
			members = make(map[string]types.Type, len(def.Members))
			for _, d := range def.Members {
				members[d.Name] = d.Type
			}
		}
	}

	// Look up the field in the struct's Members map.
	fieldType, exists := members[expr.Member]
	if !exists {
		return &TypeError{
			Pos:     pos,
			Message: fmt.Sprintf("struct %q has no field %q", st.Name, expr.Member),
		}, nil
	}

	return nil, fieldType
}
