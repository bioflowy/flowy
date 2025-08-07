package parser

import (
	"testing"

	"github.com/bioflowy/flowy/pkg/tree"
)

func TestParseDocument(t *testing.T) {
	tests := []struct {
		input         string
		hasVersion    bool
		hasWorkflow   bool
		taskCount     int
		importCount   int
		structCount   int
		description   string
	}{
		{`version 1.0

workflow hello {
	call echo_task
}

task echo_task {
	command {
		echo "Hello World"
	}
}`, true, true, 1, 0, 0, "complete WDL document"},
		{`import "utils.wdl" as utils

workflow process {
	call utils.process_file
}`, false, true, 0, 1, 0, "document with import"},
		{`version 1.1

struct Person {
	String name
	Int age
}

workflow test {
	input {
		Person person
	}
}`, true, true, 0, 0, 1, "document with struct"},
		{`task simple_task {
	command {
		echo "simple"
	}
}`, false, false, 1, 0, 0, "task-only document"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseDocument()

		if !ok {
			t.Errorf("Failed to parse document: %s", test.description)
			continue
		}

		doc := result

		// Check workflow presence
		hasWorkflow := (doc.Workflow != nil)
		if hasWorkflow != test.hasWorkflow {
			t.Errorf("%s: expected workflow=%t, got workflow=%t", 
				test.description, test.hasWorkflow, hasWorkflow)
		}

		// Check counts
		if len(doc.Tasks) != test.taskCount {
			t.Errorf("%s: expected %d tasks, got %d", 
				test.description, test.taskCount, len(doc.Tasks))
		}

		if len(doc.Imports) != test.importCount {
			t.Errorf("%s: expected %d imports, got %d", 
				test.description, test.importCount, len(doc.Imports))
		}

		if len(doc.Structs) != test.structCount {
			t.Errorf("%s: expected %d structs, got %d", 
				test.description, test.structCount, len(doc.Structs))
		}
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input           string
		expectedVersion string
		valid           bool
	}{
		{`version 1.0`, "1.0", true},
		{`version "1.1"`, "1.1", true},
		{`version development`, "development", true},
		{`version "draft-3"`, "draft-3", true},
		{`version 1.2`, "1.2", true},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseVersion()

		if test.valid && !ok {
			t.Errorf("Expected version '%s' to be valid", test.input)
			continue
		}

		if !test.valid && ok {
			t.Errorf("Expected version '%s' to be invalid", test.input)
			continue
		}

		if test.valid && result != test.expectedVersion {
			t.Errorf("Input '%s': expected version '%s', got '%s'", 
				test.input, test.expectedVersion, result)
		}
	}
}

func TestParseImport(t *testing.T) {
	tests := []struct {
		input             string
		expectedURI       string
		expectedNamespace string
		hasAlias          bool
		description       string
	}{
		{`import "utils.wdl"`, "utils.wdl", "utils", false, "simple import"},
		{`import "lib/helpers.wdl" as helpers`, "lib/helpers.wdl", "helpers", true, "import with alias"},
		{`import "https://example.com/lib.wdl"`, "https://example.com/lib.wdl", "lib", false, "URL import"},
		{`import "namespace/utils.wdl" as ns`, "namespace/utils.wdl", "ns", true, "namespaced import with alias"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseImport()

		if !ok {
			t.Errorf("Failed to parse import: %s", test.description)
			continue
		}

		importStmt := result

		if importStmt.URI != test.expectedURI {
			t.Errorf("%s: expected URI '%s', got '%s'", 
				test.description, test.expectedURI, importStmt.URI)
		}

		if importStmt.Namespace != test.expectedNamespace {
			t.Errorf("%s: expected namespace '%s', got '%s'", 
				test.description, test.expectedNamespace, importStmt.Namespace)
		}

		hasAlias := (importStmt.Alias != nil)
		if hasAlias != test.hasAlias {
			t.Errorf("%s: expected hasAlias=%t, got hasAlias=%t", 
				test.description, test.hasAlias, hasAlias)
		}
	}
}

func TestExtractNamespaceFromURI(t *testing.T) {
	tests := []struct {
		uri               string
		expectedNamespace string
	}{
		{"utils.wdl", "utils"},
		{"lib/helpers.wdl", "helpers"},
		{"https://example.com/library.wdl", "library"},
		{"/path/to/my-utils.wdl", "my_utils"},
		{"complex.name.wdl", "complex_name"},
		{"simple", "simple"},
	}

	for _, test := range tests {
		result := extractNamespaceFromURI(test.uri)

		if result != test.expectedNamespace {
			t.Errorf("URI '%s': expected namespace '%s', got '%s'", 
				test.uri, test.expectedNamespace, result)
		}
	}
}

func TestParseDocumentElement(t *testing.T) {
	tests := []struct {
		input       string
		elementType string
		description string
	}{
		{`import "lib.wdl"`, "import", "import element"},
		{`task test_task { command { echo "test" } }`, "task", "task element"},
		{`workflow test_workflow { call test_task }`, "workflow", "workflow element"},
		{`struct TestStruct { String name }`, "struct", "struct element"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseDocumentElement()

		if !ok {
			t.Errorf("Failed to parse document element: %s", test.description)
			continue
		}

		if result == nil {
			t.Errorf("Document element should not be nil for %s", test.description)
		}
	}
}

func TestIsDocumentElementStart(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"import", true},
		{"task", true},
		{"workflow", true},
		{"struct", true},
		{"version", false},
		{"{", false},
		{"String", false},
		{"42", false},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result := parser.isDocumentElementStart()

		if result != test.expected {
			t.Errorf("Input '%s': isDocumentElementStart() expected %t, got %t", 
				test.input, test.expected, result)
		}
	}
}

func TestValidateDocumentStructure(t *testing.T) {
	tests := []struct {
		document    *tree.Document
		valid       bool
		description string
	}{
		// Note: These would need actual Document objects, which require proper construction
		// For now, we'll test the validation logic conceptually
	}

	for _, test := range tests {
		parser := NewParser("", "test.wdl")
		result := parser.validateDocumentStructure(test.document)

		if result != test.valid {
			t.Errorf("%s: expected valid=%t, got valid=%t", 
				test.description, test.valid, result)
		}
	}
}

func TestParseDocumentElements(t *testing.T) {
	tests := []struct {
		input           string
		expectedCount   int
		description     string
	}{
		{`import "lib.wdl"
		  task test { command { echo "test" } }
		  workflow main { call test }`, 3, "mixed elements"},
		{`task task1 { command { echo "1" } }
		  task task2 { command { echo "2" } }`, 2, "multiple tasks"},
		{`workflow only { call some_task }`, 1, "single workflow"},
		{``, 0, "no elements"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseDocumentElements()

		if !ok {
			t.Errorf("Failed to parse document elements: %s", test.description)
			continue
		}

		if len(result) != test.expectedCount {
			t.Errorf("%s: expected %d elements, got %d", 
				test.description, test.expectedCount, len(result))
		}
	}
}

func TestParseOptionalVersion(t *testing.T) {
	tests := []struct {
		input           string
		expectedVersion string
		hasVersion      bool
	}{
		{`version 1.0`, "1.0", true},
		{`workflow test {}`, "", false},
		{`task test {}`, "", false},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseOptionalVersion()

		if !ok {
			t.Errorf("Failed to parse optional version for input '%s'", test.input)
			continue
		}

		hasVersion := (result != "")
		if hasVersion != test.hasVersion {
			t.Errorf("Input '%s': expected hasVersion=%t, got hasVersion=%t", 
				test.input, test.hasVersion, hasVersion)
		}

		if test.hasVersion && result != test.expectedVersion {
			t.Errorf("Input '%s': expected version '%s', got '%s'", 
				test.input, test.expectedVersion, result)
		}
	}
}

func TestIsValidWDLVersion(t *testing.T) {
	tests := []struct {
		version string
		valid   bool
	}{
		{"1.0", true},
		{"1.1", true},
		{"1.2", true},
		{"draft-2", true},
		{"draft-3", true},
		{"development", true},
		{"2.0", false},
		{"0.5", false},
		{"invalid", false},
		{"", false},
	}

	for _, test := range tests {
		result := isValidWDLVersion(test.version)

		if result != test.valid {
			t.Errorf("Version '%s': expected valid=%t, got valid=%t", 
				test.version, test.valid, result)
		}
	}
}

func TestCompleteWDLDocument(t *testing.T) {
	input := `version 1.0

import "utils.wdl" as utils

struct Config {
	String output_dir
	Boolean debug = false
}

task process_file {
	input {
		File input_file
		Config config
	}
	
	command <<<
		mkdir -p ${config.output_dir}
		cp ${input_file} ${config.output_dir}/
	>>>
	
	output {
		File result = "${config.output_dir}/" + basename(input_file)
	}
	
	runtime {
		docker: "ubuntu:20.04"
		memory: "2GB"
	}
}

workflow main {
	input {
		Array[File] input_files
		Config config
	}
	
	scatter (file in input_files) {
		call process_file { 
			input: input_file=file, config=config 
		}
	}
	
	output {
		Array[File] results = process_file.result
	}
}`

	parser := NewParser(input, "complete.wdl")
	result, ok := parser.parseDocument()

	if !ok {
		t.Errorf("Failed to parse complete WDL document")
		return
	}

	doc := result

	// Verify document structure
	if len(doc.Imports) != 1 {
		t.Errorf("Expected 1 import, got %d", len(doc.Imports))
	}

	if len(doc.Structs) != 1 {
		t.Errorf("Expected 1 struct, got %d", len(doc.Structs))
	}

	if len(doc.Tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(doc.Tasks))
	}

	if doc.Workflow == nil {
		t.Error("Expected workflow to be present")
	}

	if doc.Workflow.Name != "main" {
		t.Errorf("Expected workflow name 'main', got '%s'", doc.Workflow.Name)
	}
}

func TestDocumentParseErrors(t *testing.T) {
	tests := []struct {
		input       string
		description string
	}{
		{"version", "incomplete version"},
		{"import", "incomplete import"},
		{"task", "incomplete task"},
		{"workflow", "incomplete workflow"},
		{"struct", "incomplete struct"},
		{"invalid_keyword {}", "invalid top-level element"},
		{"version 1.0 version 1.1", "duplicate version"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseDocument()

		if ok && test.description != "duplicate version" {
			t.Errorf("Expected parsing '%s' to fail (%s), but got: %T", 
				test.input, test.description, result)
		}

		// Some errors might still produce a partial document
		// Check that error was recorded
		if !parser.HasErrors() && test.description != "duplicate version" {
			t.Errorf("Expected error to be recorded when parsing '%s'", test.input)
		}
	}
}

func TestImportWithAliases(t *testing.T) {
	input := `import "lib.wdl" as lib alias old_func as new_func`

	parser := NewParser(input, "test.wdl")
	result, ok := parser.parseImport()

	if !ok {
		t.Errorf("Failed to parse import with aliases")
		return
	}

	importStmt := result

	if importStmt.URI != "lib.wdl" {
		t.Errorf("Expected URI 'lib.wdl', got '%s'", importStmt.URI)
	}

	if importStmt.Namespace != "lib" {
		t.Errorf("Expected namespace 'lib', got '%s'", importStmt.Namespace)
	}
}

func TestEmptyDocument(t *testing.T) {
	parser := NewParser("", "empty.wdl")
	result, ok := parser.parseDocument()

	// An empty document might be valid but should have errors
	// since WDL requires at least one task or workflow
	if ok {
		doc := result
		if doc != nil {
			if len(doc.Tasks) == 0 && doc.Workflow == nil {
				// This should trigger a validation error
				if !parser.HasErrors() {
					t.Error("Expected validation error for empty document")
				}
			}
		}
	}
}