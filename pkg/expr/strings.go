package expr

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/bioflowy/flowy/pkg/env"
	"github.com/bioflowy/flowy/pkg/errors"
	"github.com/bioflowy/flowy/pkg/types"
	"github.com/bioflowy/flowy/pkg/utils"
	"github.com/bioflowy/flowy/pkg/values"
)

// TaskCommand represents a task command string with special WDL handling
type TaskCommand struct {
	baseExpr
	Value         string
	Interpolation []Expr // For ${expr} interpolation within commands
}

// NewTaskCommand creates a new task command string
func NewTaskCommand(value string, interpolation []Expr, pos errors.SourcePosition) *TaskCommand {
	return &TaskCommand{
		baseExpr:      NewBaseExpr(pos),
		Value:         value,
		Interpolation: interpolation,
	}
}

func (tc *TaskCommand) InferType(typeEnv *env.Bindings[types.Base], stdlib StdLib) (types.Base, error) {
	// Type check interpolated expressions if present
	if tc.Interpolation != nil {
		for _, expr := range tc.Interpolation {
			exprType, err := expr.InferType(typeEnv, stdlib)
			if err != nil {
				return nil, err
			}
			// All interpolated expressions should be coercible to String
			if err := exprType.Check(types.NewString(false), true); err != nil {
				return nil, &errors.InvalidType{
					ValidationError: errors.NewValidationErrorFromPos(tc.pos, fmt.Sprintf("type mismatch: expected %s, got %s", "String", exprType.String())),
				}
			}
		}
	}
	return types.NewString(false), nil
}

func (tc *TaskCommand) Eval(valueEnv *env.Bindings[values.Base], stdlib StdLib) (values.Base, error) {
	// Remove common leading whitespace (dedentation)
	_, dedented := utils.StripLeadingWhitespace(tc.Value)
	
	if tc.Interpolation == nil {
		// Simple command string - return dedented value
		return values.NewString(dedented, false), nil
	}

	// Command interpolation - evaluate embedded expressions
	result := dedented
	for _, expr := range tc.Interpolation {
		val, err := expr.Eval(valueEnv, stdlib)
		if err != nil {
			return nil, err
		}

		// Coerce to string
		stringVal, err := val.Coerce(types.NewString(false))
		if err != nil {
			return nil, errors.NewEvalErrorFromPos(tc.pos, "cannot convert expression to string: "+err.Error())
		}

		// Replace placeholder with actual value
		strValue := stringVal.(*values.StringValue)
		// In a real implementation, we'd need to track placeholder positions
		// For now, we'll just append - this would need more sophisticated string building
		result += strValue.Value().(string)
	}

	return values.NewString(result, false), nil
}

func (tc *TaskCommand) TypeCheck(expectedType types.Base, typeEnv *env.Bindings[types.Base], stdlib StdLib) error {
	// First ensure this command can coerce to expected type
	stringType := types.NewString(false)
	helper := TypeCheckHelper{}
	if err := helper.CheckCoercion(stringType, expectedType, tc.pos); err != nil {
		return err
	}

	// Type check interpolated expressions
	if tc.Interpolation != nil {
		for _, expr := range tc.Interpolation {
			if err := expr.TypeCheck(types.NewString(false), typeEnv, stdlib); err != nil {
				return err
			}
		}
	}

	return nil
}

func (tc *TaskCommand) Children() []Expr {
	if tc.Interpolation == nil {
		return []Expr{}
	}
	return tc.Interpolation
}

func (tc *TaskCommand) String() string {
	if tc.Interpolation == nil {
		return fmt.Sprintf(`command{%s}`, tc.Value)
	}
	return fmt.Sprintf(`command{%s} (with interpolation)`, tc.Value)
}

// MultilineString represents a WDL 1.2 multiline string literal
type MultilineString struct {
	baseExpr
	Value         string
	Interpolation []Expr
}

// NewMultilineString creates a new multiline string
func NewMultilineString(value string, interpolation []Expr, pos errors.SourcePosition) *MultilineString {
	return &MultilineString{
		baseExpr:      NewBaseExpr(pos),
		Value:         value,
		Interpolation: interpolation,
	}
}

func (ms *MultilineString) InferType(typeEnv *env.Bindings[types.Base], stdlib StdLib) (types.Base, error) {
	// Type check interpolated expressions if present
	if ms.Interpolation != nil {
		for _, expr := range ms.Interpolation {
			exprType, err := expr.InferType(typeEnv, stdlib)
			if err != nil {
				return nil, err
			}
			// All interpolated expressions should be coercible to String
			if err := exprType.Check(types.NewString(false), true); err != nil {
				return nil, &errors.InvalidType{
					ValidationError: errors.NewValidationErrorFromPos(ms.pos, fmt.Sprintf("type mismatch: expected %s, got %s", "String", exprType.String())),
				}
			}
		}
	}
	return types.NewString(false), nil
}

func (ms *MultilineString) Eval(valueEnv *env.Bindings[values.Base], stdlib StdLib) (values.Base, error) {
	// Process multiline string: trim whitespace, remove escaped newlines, dedent
	processed := ms.processMultilineString(ms.Value)
	
	if ms.Interpolation == nil {
		return values.NewString(processed, false), nil
	}

	// String interpolation - evaluate embedded expressions
	result := processed
	for _, expr := range ms.Interpolation {
		val, err := expr.Eval(valueEnv, stdlib)
		if err != nil {
			return nil, err
		}

		// Coerce to string
		stringVal, err := val.Coerce(types.NewString(false))
		if err != nil {
			return nil, errors.NewEvalErrorFromPos(ms.pos, "cannot convert expression to string: "+err.Error())
		}

		// Replace placeholder with actual value
		strValue := stringVal.(*values.StringValue)
		result += strValue.Value().(string)
	}

	return values.NewString(result, false), nil
}

func (ms *MultilineString) processMultilineString(value string) string {
	// Remove escaped newlines
	escaped := regexp.MustCompile(`\\n`).ReplaceAllString(value, "\n")
	
	// Dedent non-blank lines while preserving structure
	lines := strings.Split(escaped, "\n")
	if len(lines) <= 1 {
		return escaped
	}
	
	// Find minimum indentation of non-empty lines
	minIndent := -1
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			indent := 0
			for _, char := range line {
				if char == ' ' || char == '\t' {
					indent++
				} else {
					break
				}
			}
			if minIndent == -1 || indent < minIndent {
				minIndent = indent
			}
		}
	}
	
	if minIndent <= 0 {
		return escaped
	}
	
	// Remove common indentation
	var dedented []string
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			dedented = append(dedented, "")
		} else if len(line) > minIndent {
			dedented = append(dedented, line[minIndent:])
		} else {
			dedented = append(dedented, line)
		}
	}
	
	return strings.Join(dedented, "\n")
}

func (ms *MultilineString) TypeCheck(expectedType types.Base, typeEnv *env.Bindings[types.Base], stdlib StdLib) error {
	stringType := types.NewString(false)
	helper := TypeCheckHelper{}
	if err := helper.CheckCoercion(stringType, expectedType, ms.pos); err != nil {
		return err
	}

	// Type check interpolated expressions
	if ms.Interpolation != nil {
		for _, expr := range ms.Interpolation {
			if err := expr.TypeCheck(types.NewString(false), typeEnv, stdlib); err != nil {
				return err
			}
		}
	}

	return nil
}

func (ms *MultilineString) Children() []Expr {
	if ms.Interpolation == nil {
		return []Expr{}
	}
	return ms.Interpolation
}

func (ms *MultilineString) String() string {
	if ms.Interpolation == nil {
		return fmt.Sprintf(`""""%s""""`, ms.Value)
	}
	return fmt.Sprintf(`""""%s"""" (with interpolation)`, ms.Value)
}

// LeftName is a placeholder node used by the parser for disambiguation
type LeftName struct {
	baseExpr
	Name string
}

// NewLeftName creates a new left name placeholder
func NewLeftName(name string, pos errors.SourcePosition) *LeftName {
	return &LeftName{
		baseExpr: NewBaseExpr(pos),
		Name:     name,
	}
}

func (ln *LeftName) InferType(typeEnv *env.Bindings[types.Base], stdlib StdLib) (types.Base, error) {
	// LeftName should be transformed to proper Identifier during typechecking
	return nil, errors.NewValidationErrorFromPos(ln.pos, "LeftName node should be transformed during parsing")
}

func (ln *LeftName) Eval(valueEnv *env.Bindings[values.Base], stdlib StdLib) (values.Base, error) {
	return nil, errors.NewEvalErrorFromPos(ln.pos, "LeftName node should not be evaluated")
}

func (ln *LeftName) TypeCheck(expectedType types.Base, typeEnv *env.Bindings[types.Base], stdlib StdLib) error {
	return errors.NewValidationErrorFromPos(ln.pos, "LeftName node should be transformed during parsing")
}

func (ln *LeftName) String() string {
	return fmt.Sprintf("_LeftName(%s)", ln.Name)
}

// Placeholder represents an interpolated expression within strings/commands
type Placeholder struct {
	baseExpr
	Expression Expr
	Options    *PlaceholderOptions
}

// PlaceholderOptions contains options for placeholder interpolation
type PlaceholderOptions struct {
	Separator string    // sep option
	Default   Expr      // default option  
	TrueValue *string   // true option
	FalseValue *string  // false option
}

// NewPlaceholder creates a new placeholder
func NewPlaceholder(expression Expr, options *PlaceholderOptions, pos errors.SourcePosition) *Placeholder {
	return &Placeholder{
		baseExpr:   NewBaseExpr(pos),
		Expression: expression,
		Options:    options,
	}
}

func (p *Placeholder) InferType(typeEnv *env.Bindings[types.Base], stdlib StdLib) (types.Base, error) {
	// Placeholder always produces a string
	_, err := p.Expression.InferType(typeEnv, stdlib)
	if err != nil {
		return nil, err
	}
	
	// Type check default expression if present
	if p.Options != nil && p.Options.Default != nil {
		_, err := p.Options.Default.InferType(typeEnv, stdlib)
		if err != nil {
			return nil, err
		}
	}
	
	return types.NewString(false), nil
}

func (p *Placeholder) Eval(valueEnv *env.Bindings[values.Base], stdlib StdLib) (values.Base, error) {
	val, err := p.Expression.Eval(valueEnv, stdlib)
	if err != nil {
		// If evaluation fails and we have a default, use it
		if p.Options != nil && p.Options.Default != nil {
			defaultVal, defaultErr := p.Options.Default.Eval(valueEnv, stdlib)
			if defaultErr != nil {
				return nil, err // Return original error
			}
			val = defaultVal
		} else {
			return nil, err
		}
	}

	// Convert value to string based on type and options
	stringResult, err := p.valueToString(val)
	if err != nil {
		return nil, errors.NewEvalErrorFromPos(p.pos, "failed to convert placeholder value to string: "+err.Error())
	}

	return values.NewString(stringResult, false), nil
}

func (p *Placeholder) valueToString(val values.Base) (string, error) {
	switch v := val.(type) {
	case *values.BooleanValue:
		boolVal := v.Value().(bool)
		if p.Options != nil {
			if boolVal && p.Options.TrueValue != nil {
				return *p.Options.TrueValue, nil
			}
			if !boolVal && p.Options.FalseValue != nil {
				return *p.Options.FalseValue, nil
			}
		}
		if boolVal {
			return "true", nil
		}
		return "false", nil
		
	case *values.ArrayValue:
		items := v.Items()
		var strItems []string
		for _, item := range items {
			itemStr, err := p.valueToString(item)
			if err != nil {
				return "", err
			}
			strItems = append(strItems, itemStr)
		}
		separator := " "
		if p.Options != nil && p.Options.Separator != "" {
			separator = p.Options.Separator
		}
		return strings.Join(strItems, separator), nil
		
	default:
		// Use standard string coercion
		stringVal, err := val.Coerce(types.NewString(false))
		if err != nil {
			return "", err
		}
		return stringVal.(*values.StringValue).Value().(string), nil
	}
}

func (p *Placeholder) TypeCheck(expectedType types.Base, typeEnv *env.Bindings[types.Base], stdlib StdLib) error {
	// Placeholder always produces string, check if it can coerce to expected type
	stringType := types.NewString(false)
	helper := TypeCheckHelper{}
	if err := helper.CheckCoercion(stringType, expectedType, p.pos); err != nil {
		return err
	}

	// Type check the embedded expression
	if err := p.Expression.TypeCheck(types.NewAny(false, false), typeEnv, stdlib); err != nil {
		return err
	}
	
	// Type check default if present
	if p.Options != nil && p.Options.Default != nil {
		if err := p.Options.Default.TypeCheck(types.NewAny(false, false), typeEnv, stdlib); err != nil {
			return err
		}
	}

	return nil
}

func (p *Placeholder) Children() []Expr {
	children := []Expr{p.Expression}
	if p.Options != nil && p.Options.Default != nil {
		children = append(children, p.Options.Default)
	}
	return children
}

func (p *Placeholder) String() string {
	result := fmt.Sprintf("${%s", p.Expression.String())
	if p.Options != nil {
		if p.Options.Separator != "" {
			result += fmt.Sprintf(" sep='%s'", p.Options.Separator)
		}
		if p.Options.Default != nil {
			result += fmt.Sprintf(" default=%s", p.Options.Default.String())
		}
		if p.Options.TrueValue != nil {
			result += fmt.Sprintf(" true='%s'", *p.Options.TrueValue)
		}
		if p.Options.FalseValue != nil {
			result += fmt.Sprintf(" false='%s'", *p.Options.FalseValue)
		}
	}
	result += "}"
	return result
}