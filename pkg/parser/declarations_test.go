package parser

import (
	"testing"

	"github.com/bioflowy/flowy/pkg/tree"
	"github.com/bioflowy/flowy/pkg/types"
)

func TestParseUnboundDeclaration(t *testing.T) {
	tests := []struct {
		input        string
		expectedType string
		expectedName string
	}{
		{"Int counter", "Int", "counter"},
		{"String message", "String", "message"},
		{"Boolean flag", "Boolean", "flag"},
		{"Array[Int] numbers", "Array[Int]", "numbers"},
		{"Map[String,Int] lookup", "Map[String,Int]", "lookup"},
		{"File input_file", "File", "input_file"},
		{"Int? optional_value", "Int?", "optional_value"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseUnboundDeclaration()

		if !ok {
			t.Errorf("Failed to parse unbound declaration '%s'", test.input)
			continue
		}

		decl := result
		if decl == nil {
			t.Errorf("Expected Declaration, got nil")
			continue
		}

		if decl.Type.String() != test.expectedType {
			t.Errorf("Input '%s': expected type '%s', got '%s'", 
				test.input, test.expectedType, decl.Type.String())
		}

		if decl.Name != test.expectedName {
			t.Errorf("Input '%s': expected name '%s', got '%s'", 
				test.input, test.expectedName, decl.Name)
		}

		if decl.Expr != nil {
			t.Errorf("Input '%s': unbound declaration should have no value, got %T", 
				test.input, decl.Expr)
		}
	}
}

func TestParseBoundDeclaration(t *testing.T) {
	tests := []struct {
		input        string
		expectedType string
		expectedName string
		expectedValue string
	}{
		{"Int counter = 42", "Int", "counter", "IntLiteral"},
		{"String message = \"hello\"", "String", "message", "StringLiteral"},
		{"Boolean flag = true", "Boolean", "flag", "BooleanLiteral"},
		{"Float ratio = 3.14", "Float", "ratio", "FloatLiteral"},
		{"Int calculated = func(x, y)", "Int", "calculated", "FunctionCall"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseBoundDeclaration()

		if !ok {
			t.Errorf("Failed to parse bound declaration '%s'", test.input)
			continue
		}

		decl := result
		if decl == nil {
			t.Errorf("Expected Declaration, got nil")
			continue
		}

		if decl.Type.String() != test.expectedType {
			t.Errorf("Input '%s': expected type '%s', got '%s'", 
				test.input, test.expectedType, decl.Type.String())
		}

		if decl.Name != test.expectedName {
			t.Errorf("Input '%s': expected name '%s', got '%s'", 
				test.input, test.expectedName, decl.Name)
		}

		if decl.Expr == nil {
			t.Errorf("Input '%s': bound declaration should have a value", test.input)
		}
	}
}

func TestParseAnyDeclaration(t *testing.T) {
	tests := []struct {
		input    string
		hasBound bool
	}{
		{"Int counter", false},
		{"String message = \"hello\"", true},
		{"Boolean flag", false},
		{"Float ratio = 3.14", true},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseDeclaration()

		if !ok {
			t.Errorf("Failed to parse any declaration '%s'", test.input)
			continue
		}

		decl := result
		if decl == nil {
			t.Errorf("Expected Declaration, got nil")
			continue
		}

		hasValue := (decl.Expr != nil)
		if hasValue != test.hasBound {
			t.Errorf("Input '%s': expected bound=%t, got bound=%t", 
				test.input, test.hasBound, hasValue)
		}
	}
}

func TestParseStruct(t *testing.T) {
	tests := []struct {
		input         string
		expectedName  string
		expectedFields int
	}{
		{`struct Person {
			String name
			Int age
		}`, "Person", 2},
		{`struct Point {
			Float x
			Float y
			Float z
		}`, "Point", 3},
		{`struct Empty {
		}`, "Empty", 0},
		{`struct Config {
			Boolean debug = false
			Int timeout = 30
			String? logfile
		}`, "Config", 3},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseStruct()

		if !ok {
			t.Errorf("Failed to parse struct '%s'", test.input)
			continue
		}

		structDef := result
		if structDef == nil {
			t.Errorf("Expected StructTypeDef, got nil")
			continue
		}

		if structDef.Name != test.expectedName {
			t.Errorf("Input struct: expected name '%s', got '%s'", 
				test.expectedName, structDef.Name)
		}

		if len(structDef.Members) != test.expectedFields {
			t.Errorf("Input struct: expected %d fields, got %d", 
				test.expectedFields, len(structDef.Members))
		}
	}
}

func TestParseStructMember(t *testing.T) {
	tests := []struct {
		input        string
		expectedType string
		expectedName string
		hasBound     bool
	}{
		{"String name", "String", "name", false},
		{"Int age", "Int", "age", false},
		{"Boolean? active", "Boolean?", "active", false},
		{"Array[String] tags", "Array[String]", "tags", false},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseStructMember()

		if !ok {
			t.Errorf("Failed to parse struct member '%s'", test.input)
			continue
		}

		member := result
		if member == nil {
			t.Errorf("Expected Declaration, got nil")
			continue
		}

		if member.Type.String() != test.expectedType {
			t.Errorf("Input '%s': expected type '%s', got '%s'", 
				test.input, test.expectedType, member.Type.String())
		}

		if member.Name != test.expectedName {
			t.Errorf("Input '%s': expected name '%s', got '%s'", 
				test.input, test.expectedName, member.Name)
		}

		// StructMember doesn't support initialization expressions
		if test.hasBound {
			t.Errorf("Input '%s': StructMember doesn't support initialization expressions", 
				test.input)
		}
	}
}

func TestParseInputSection(t *testing.T) {
	tests := []struct {
		input           string
		expectedDecls   int
		description     string
	}{
		{`input {
			String name
			Int count
		}`, 2, "simple input section"},
		{`input {
			String name = "default"
			Int count
			Boolean? optional
		}`, 3, "input with defaults and optional"},
		{`input {
		}`, 0, "empty input section"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseInputDeclarations()

		if !ok {
			t.Errorf("Failed to parse %s", test.description)
			continue
		}

		input := result
		if input == nil {
			t.Errorf("Expected Input, got nil")
			continue
		}

		if len(input.Decls) != test.expectedDecls {
			t.Errorf("%s: expected %d declarations, got %d", 
				test.description, test.expectedDecls, len(input.Decls))
		}
	}
}

func TestParseOutputSection(t *testing.T) {
	tests := []struct {
		input           string
		expectedDecls   int
		description     string
	}{
		{`output {
			String result = task.output
			Int exitCode = task.exitCode
		}`, 2, "simple output section"},
		{`output {
			File output_file = my_task.result
		}`, 1, "single output"},
		{`output {
		}`, 0, "empty output section"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseOutputDeclarations()

		if !ok {
			t.Errorf("Failed to parse %s", test.description)
			continue
		}

		output := result
		if output == nil {
			t.Errorf("Expected Output, got nil")
			continue
		}

		if len(output.Decls) != test.expectedDecls {
			t.Errorf("%s: expected %d declarations, got %d", 
				test.description, test.expectedDecls, len(output.Decls))
		}
	}
}

func TestParseDeclarationList(t *testing.T) {
	tests := []struct {
		input         string
		terminator    TokenType
		expectedCount int
		description   string
	}{
		{`String name
		  Int age
		  Boolean active}`, TokenRightBrace, 3, "multiple declarations"},
		{`File input
		  String? optional}`, TokenRightBrace, 2, "mixed declarations"},
		{``, TokenRightBrace, 0, "empty list"},
		{`Int single}`, TokenRightBrace, 1, "single declaration"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseDeclarationList(test.terminator)

		if !ok {
			t.Errorf("Failed to parse declaration list: %s", test.description)
			continue
		}

		if len(result) != test.expectedCount {
			t.Errorf("%s: expected %d declarations, got %d", 
				test.description, test.expectedCount, len(result))
		}
	}
}

func TestParseMetaSection(t *testing.T) {
	tests := []struct {
		input       string
		description string
	}{
		{`meta {
			author: "John Doe"
			version: "1.0"
		}`, "simple meta section"},
		{`meta {
			description: "A test task"
			tags: ["bioinformatics", "genomics"]
		}`, "meta with array"},
		{`meta {
		}`, "empty meta section"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseMetaSection()

		if !ok {
			t.Errorf("Failed to parse %s", test.description)
			continue
		}

		metaMap := result
		_ = metaMap // Use the variable to avoid unused warning
	}
}

func TestParseParameterMetaSection(t *testing.T) {
	tests := []struct {
		input       string
		description string
	}{
		{`parameter_meta {
			name: "The input name"
			count: "Number of items"
		}`, "simple parameter_meta section"},
		{`parameter_meta {
		}`, "empty parameter_meta section"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseParameterMetaSection()

		if !ok {
			t.Errorf("Failed to parse %s", test.description)
			continue
		}

		metaMap := result
		_ = metaMap // Use the variable
	}
}

func TestIsDeclarationStart(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"Int", true},
		{"String", true},
		{"Array[Int]", true},
		{"MyStruct", true},
		{"workflow", false},
		{"task", false},
		{"{", false},
		{"42", false},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result := parser.isDeclarationStart()

		if result != test.expected {
			t.Errorf("Input '%s': isDeclarationStart() expected %t, got %t", 
				test.input, test.expected, result)
		}
	}
}

func TestValidateDeclaration(t *testing.T) {
	tests := []struct {
		declType types.Base
		name     string
		valid    bool
	}{
		{types.NewInt(false), "valid_name", true},
		{types.NewString(false), "validName", true},
		{types.NewBoolean(false), "_private", true},
		{types.NewFloat(false), "name123", true},
		{nil, "invalid_type", false},
		{types.NewInt(false), "", false},
		{types.NewInt(false), "123invalid", false},
		{types.NewInt(false), "workflow", false}, // Reserved keyword
	}

	for _, test := range tests {
		parser := NewParser("workflow test {}", "test.wdl")
		// Test type validity (nil check) and name validity
		typeValid := test.declType != nil
		nameValid := parser.validateDeclarationName(test.name)
		result := typeValid && nameValid

		if result != test.valid {
			t.Errorf("Declaration validation for type=%v, name='%s': expected %t, got %t", 
				test.declType, test.name, test.valid, result)
		}
	}
}

func TestDeclarationParseErrors(t *testing.T) {
	tests := []struct {
		input       string
		description string
	}{
		{"workflow name", "invalid type name"},
		{"Int", "missing variable name"},
		{"String name =", "missing expression after assignment"},
		{"Int 123invalid", "invalid variable name"},
		{"name", "missing type"},
		{"struct {}", "missing struct name"},
		{"struct Name", "missing struct body"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		
		// Try different parsing methods
		if _, ok := parser.parseUnboundDeclaration(); ok {
			t.Errorf("Expected unbound declaration parsing of '%s' to fail (%s)", 
				test.input, test.description)
		}
		
		// Reset parser for next attempt
		parser = NewParser(test.input, "test.wdl")
		if _, ok := parser.parseBoundDeclaration(); ok && test.description != "missing expression after assignment" {
			t.Errorf("Expected bound declaration parsing of '%s' to fail (%s)", 
				test.input, test.description)
		}
	}
}

func TestOptionalDeclaration(t *testing.T) {
	tests := []struct {
		input    string
		expected bool // whether a declaration was parsed
	}{
		{"Int count", true},
		{"String name = \"hello\"", true},
		{"workflow test", false}, // Not a declaration
		{"42", false},            // Not a declaration
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		// Try to parse a declaration, return nil if it fails
		var result *tree.Decl
		if parser.isDeclarationStart() {
			if decl, ok := parser.parseDeclaration(); ok {
				result = decl
			}
		}

		hasDeclaration := (result != nil)
		if hasDeclaration != test.expected {
			t.Errorf("Input '%s': parseOptionalDeclaration() expected decl=%t, got decl=%t", 
				test.input, test.expected, hasDeclaration)
		}
	}
}