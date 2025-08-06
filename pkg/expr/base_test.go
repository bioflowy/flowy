package expr

import (
	"testing"

	"github.com/bioflowy/flowy/pkg/env"
	"github.com/bioflowy/flowy/pkg/errors"
	"github.com/bioflowy/flowy/pkg/types"
	"github.com/bioflowy/flowy/pkg/values"
)

// Test helper functions

func TestNewBaseExpr(t *testing.T) {
	pos := errors.SourcePosition{
		URI:    "test.wdl",
		Line:   10,
		Column: 5,
	}

	base := NewBaseExpr(pos)

	if base.pos != pos {
		t.Errorf("Expected position %v, got %v", pos, base.pos)
	}
}

func TestBaseExprPos(t *testing.T) {
	pos := errors.SourcePosition{
		URI:    "test.wdl",
		Line:   20,
		Column: 15,
	}

	base := baseExpr{pos: pos}

	if base.Pos() != pos {
		t.Errorf("Expected position %v, got %v", pos, base.Pos())
	}
}

func TestBaseExprLiteral(t *testing.T) {
	base := baseExpr{}

	literal, isLiteral := base.Literal()

	if literal != nil {
		t.Errorf("Expected nil literal, got %v", literal)
	}

	if isLiteral {
		t.Errorf("Expected false for isLiteral, got true")
	}
}

func TestFunction(t *testing.T) {
	// Test Function struct fields
	fn := Function{
		Name:       "test_function",
		ParamTypes: []types.Base{types.NewString(false), types.NewInt(false)},
		ReturnType: types.NewBoolean(false),
		Variadic:   false,
	}

	if fn.Name != "test_function" {
		t.Errorf("Expected name 'test_function', got %s", fn.Name)
	}

	if len(fn.ParamTypes) != 2 {
		t.Errorf("Expected 2 parameter types, got %d", len(fn.ParamTypes))
	}

	if fn.Variadic {
		t.Errorf("Expected Variadic to be false, got true")
	}
}

// Test TypeCheckHelper

func TestTypeCheckHelperCheckCoercion(t *testing.T) {
	helper := TypeCheckHelper{}
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	tests := []struct {
		name        string
		sourceType  types.Base
		targetType  types.Base
		expectError bool
	}{
		{
			name:        "Same types",
			sourceType:  types.NewString(false),
			targetType:  types.NewString(false),
			expectError: false,
		},
		{
			name:        "Optional to non-optional",
			sourceType:  types.NewString(true),
			targetType:  types.NewString(false),
			expectError: true,
		},
		{
			name:        "Non-optional to optional",
			sourceType:  types.NewString(false),
			targetType:  types.NewString(true),
			expectError: false,
		},
		{
			name:        "Different types",
			sourceType:  types.NewString(false),
			targetType:  types.NewBoolean(false),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := helper.CheckCoercion(tt.sourceType, tt.targetType, pos)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestTypeCheckHelperCheckArity(t *testing.T) {
	helper := TypeCheckHelper{}
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	tests := []struct {
		name        string
		function    string
		expected    int
		actual      int
		variadic    bool
		expectError bool
	}{
		{
			name:        "Exact match",
			function:    "test_func",
			expected:    2,
			actual:      2,
			variadic:    false,
			expectError: false,
		},
		{
			name:        "Too few arguments",
			function:    "test_func",
			expected:    3,
			actual:      2,
			variadic:    false,
			expectError: true,
		},
		{
			name:        "Too many arguments",
			function:    "test_func",
			expected:    2,
			actual:      3,
			variadic:    false,
			expectError: true,
		},
		{
			name:        "Variadic with minimum args",
			function:    "variadic_func",
			expected:    2,
			actual:      2,
			variadic:    true,
			expectError: false,
		},
		{
			name:        "Variadic with extra args",
			function:    "variadic_func",
			expected:    2,
			actual:      5,
			variadic:    true,
			expectError: false,
		},
		{
			name:        "Variadic with too few args",
			function:    "variadic_func",
			expected:    3,
			actual:      2,
			variadic:    true,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := helper.CheckArity(tt.function, tt.expected, tt.actual, tt.variadic, pos)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

// Test InferTypeHelper

func TestInferTypeHelperUnifyTypes(t *testing.T) {
	helper := InferTypeHelper{}
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}

	tests := []struct {
		name        string
		types       []types.Base
		expectError bool
		expectedStr string
	}{
		{
			name:        "Empty types",
			types:       []types.Base{},
			expectError: true,
		},
		{
			name:        "Single type",
			types:       []types.Base{types.NewString(false)},
			expectError: false,
			expectedStr: "String",
		},
		{
			name:        "Same types",
			types:       []types.Base{types.NewString(false), types.NewString(false)},
			expectError: false,
			expectedStr: "String",
		},
		{
			name:        "Incompatible optionality",
			types:       []types.Base{types.NewString(false), types.NewString(true)},
			expectError: true,
		},
		{
			name:        "Incompatible types",
			types:       []types.Base{types.NewString(false), types.NewInt(false)},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := helper.UnifyTypes(tt.types, pos)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if !tt.expectError && result != nil && tt.expectedStr != "" {
				if result.String() != tt.expectedStr {
					t.Errorf("Expected unified type %s, got %s", tt.expectedStr, result.String())
				}
			}
		})
	}
}

// Integration test with a mock stdlib

type mockStdLib struct {
	functions map[string]*Function
}

func (m *mockStdLib) HasFunction(name string) bool {
	_, exists := m.functions[name]
	return exists
}

func (m *mockStdLib) GetFunction(name string) (*Function, error) {
	if fn, exists := m.functions[name]; exists {
		return fn, nil
	}
	return nil, errors.NewUnknownIdentifier(nil, name)
}

func (m *mockStdLib) CallFunction(name string, args []values.Base, pos errors.SourcePosition) (values.Base, error) {
	return nil, errors.NewEvalError(nil, "mock function call not implemented")
}

func (m *mockStdLib) HasOperator(op string) bool {
	return op == "+" || op == "-" || op == "==" || op == "!="
}

func (m *mockStdLib) CallOperator(op string, args []values.Base, pos errors.SourcePosition) (values.Base, error) {
	return nil, errors.NewEvalError(nil, "mock operator call not implemented")
}

func TestExpressionIntegration(t *testing.T) {
	// Create mock environments
	typeEnv := env.NewBindings[types.Base]()
	typeEnv = typeEnv.Bind("test_var", types.NewString(false), nil)

	valueEnv := env.NewBindings[values.Base]()
	valueEnv = valueEnv.Bind("test_var", values.NewString("hello", false), nil)

	// Create mock stdlib
	mockStdlib := &mockStdLib{
		functions: map[string]*Function{
			"length": {
				Name:       "length",
				ParamTypes: []types.Base{types.NewString(false)},
				ReturnType: types.NewInt(false),
				Variadic:   false,
			},
		},
	}

	// Test with a simple identifier
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	identifier := NewIdentifier("test_var", pos)

	// Test type inference
	inferredType, err := identifier.InferType(typeEnv, mockStdlib)
	if err != nil {
		t.Errorf("Type inference failed: %v", err)
	}

	if inferredType.String() != "String" {
		t.Errorf("Expected String type, got %s", inferredType.String())
	}

	// Test evaluation
	value, err := identifier.Eval(valueEnv, mockStdlib)
	if err != nil {
		t.Errorf("Evaluation failed: %v", err)
	}

	stringVal, ok := value.(*values.StringValue)
	if !ok {
		t.Errorf("Expected StringValue, got %T", value)
	}

	if stringVal.Value().(string) != "hello" {
		t.Errorf("Expected 'hello', got %s", stringVal.Value().(string))
	}

	// Test type checking
	err = identifier.TypeCheck(types.NewString(false), typeEnv, mockStdlib)
	if err != nil {
		t.Errorf("Type check failed: %v", err)
	}

	// Test type check failure
	err = identifier.TypeCheck(types.NewBoolean(false), typeEnv, mockStdlib)
	if err == nil {
		t.Errorf("Expected type check to fail for incompatible type")
	}
}

func TestExpressionLiteralDefault(t *testing.T) {
	// Test that expressions return (nil, false) for Literal() by default
	pos := errors.SourcePosition{URI: "test.wdl", Line: 1, Column: 1}
	identifier := NewIdentifier("test", pos)

	literal, isLiteral := identifier.Literal()

	if literal != nil {
		t.Errorf("Expected nil literal, got %v", literal)
	}

	if isLiteral {
		t.Errorf("Expected false for isLiteral, got true")
	}
}

func TestFunctionStringAndPos(t *testing.T) {
	// Test that expressions implement String() and Pos() correctly
	pos := errors.SourcePosition{
		URI:    "test.wdl",
		Line:   42,
		Column: 10,
	}

	identifier := NewIdentifier("my_var", pos)

	// Test Pos()
	if identifier.Pos() != pos {
		t.Errorf("Expected position %v, got %v", pos, identifier.Pos())
	}

	// Test String()
	expected := "my_var"
	if identifier.String() != expected {
		t.Errorf("Expected string %s, got %s", expected, identifier.String())
	}
}
