package parser

import (
	"testing"

	"github.com/bioflowy/flowy/pkg/tree"
)

func TestParseWorkflow(t *testing.T) {
	tests := []struct {
		input        string
		expectedName string
		description  string
	}{
		{`workflow hello {
			call echo_task
		}`, "hello", "simple workflow"},
		{`workflow process_data {
			input {
				File input_file
				String prefix
			}
			
			call process_file { 
				input: file=input_file, prefix=prefix 
			}
			
			output {
				File result = process_file.output
			}
		}`, "process_data", "workflow with input/output"},
		{`workflow complex_workflow {
			input {
				Array[File] input_files
				String output_dir
			}
			
			scatter (file in input_files) {
				call process_file { input: file=file }
			}
			
			if (length(process_file.results) > 0) {
				call combine_results { 
					input: files=process_file.results, output_dir=output_dir 
				}
			}
			
			output {
				Array[File] processed = process_file.results
				File? combined = combine_results.result
			}
		}`, "complex_workflow", "workflow with scatter and conditional"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseWorkflow()

		if !ok {
			t.Errorf("Failed to parse %s", test.description)
			continue
		}

		workflow, ok := result.(*tree.Workflow)
		if !ok {
			t.Errorf("Expected Workflow, got %T for %s", result, test.description)
			continue
		}

		if workflow.Name() != test.expectedName {
			t.Errorf("%s: expected name '%s', got '%s'", 
				test.description, test.expectedName, workflow.Name())
		}
	}
}

func TestParseWorkflowInput(t *testing.T) {
	tests := []struct {
		input         string
		expectedDecls int
		description   string
	}{
		{`input {
			File input_file
			String prefix
		}`, 2, "simple workflow input"},
		{`input {
			Array[File] files
			String output_dir = "results"
			Boolean debug = false
		}`, 3, "input with defaults"},
		{`input {
			Map[String,String] config
			Int? max_threads
		}`, 2, "input with complex and optional types"},
		{`input {
		}`, 0, "empty input section"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseWorkflowInput()

		if !ok {
			t.Errorf("Failed to parse workflow %s", test.description)
			continue
		}

		declarations, ok := result.([]*tree.Declaration)
		if !ok {
			t.Errorf("Expected []*tree.Declaration, got %T", result)
			continue
		}

		if len(declarations) != test.expectedDecls {
			t.Errorf("%s: expected %d declarations, got %d", 
				test.description, test.expectedDecls, len(declarations))
		}
	}
}

func TestParseWorkflowOutput(t *testing.T) {
	tests := []struct {
		input         string
		expectedDecls int
		description   string
	}{
		{`output {
			File result = task.output
		}`, 1, "simple workflow output"},
		{`output {
			Array[File] results = scatter_task.outputs
			File summary = combine_task.summary
			String status = "completed"
		}`, 3, "multiple outputs"},
		{`output {
			File? optional_result = if defined(task.output) then task.output else None
		}`, 1, "conditional output"},
		{`output {
		}`, 0, "empty output section"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseWorkflowOutput()

		if !ok {
			t.Errorf("Failed to parse workflow %s", test.description)
			continue
		}

		declarations, ok := result.([]*tree.Declaration)
		if !ok {
			t.Errorf("Expected []*tree.Declaration, got %T", result)
			continue
		}

		if len(declarations) != test.expectedDecls {
			t.Errorf("%s: expected %d declarations, got %d", 
				test.description, test.expectedDecls, len(declarations))
		}
	}
}

func TestParseCall(t *testing.T) {
	tests := []struct {
		input           string
		expectedTask    string
		expectedAlias   string
		description     string
	}{
		{`call echo_task`, "echo_task", "", "simple call"},
		{`call my_namespace.process_file`, "process_file", "", "namespaced call"},
		{`call process_file as processor`, "process_file", "processor", "call with alias"},
		{`call process_file { 
			input: file=input_file, prefix="output" 
		}`, "process_file", "", "call with input mapping"},
		{`call my_namespace.complex_task as complex { 
			input: 
				input_file=file,
				config=workflow_config,
				debug=true 
		}`, "complex_task", "complex", "complex call with namespace and alias"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseCall()

		if !ok {
			t.Errorf("Failed to parse call: %s", test.description)
			continue
		}

		call, ok := result.(*tree.Call)
		if !ok {
			t.Errorf("Expected Call, got %T for %s", result, test.description)
			continue
		}

		if call.Task() != test.expectedTask {
			t.Errorf("%s: expected task '%s', got '%s'", 
				test.description, test.expectedTask, call.Task())
		}

		if call.Alias() != test.expectedAlias {
			t.Errorf("%s: expected alias '%s', got '%s'", 
				test.description, test.expectedAlias, call.Alias())
		}
	}
}

func TestParseCallInputs(t *testing.T) {
	tests := []struct {
		input         string
		expectedPairs int
		description   string
	}{
		{`{
			input: file=input_file, prefix="output"
		}`, 2, "simple input mapping"},
		{`{
			input:
				input_file=file,
				config=workflow_config,
				debug=true
		}`, 3, "multi-line input mapping"},
		{`{
			input: single_param=value
		}`, 1, "single input mapping"},
		{`{}`, 0, "empty call inputs"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseCallInputs()

		if !ok {
			t.Errorf("Failed to parse call inputs: %s", test.description)
			continue
		}

		inputs, ok := result.(map[string]interface{})
		if !ok {
			t.Errorf("Expected map[string]interface{}, got %T for %s", result, test.description)
			continue
		}

		if len(inputs) != test.expectedPairs {
			t.Errorf("%s: expected %d input pairs, got %d", 
				test.description, test.expectedPairs, len(inputs))
		}
	}
}

func TestParseScatter(t *testing.T) {
	tests := []struct {
		input       string
		description string
	}{
		{`scatter (item in array) {
			call process { input: item=item }
		}`, "simple scatter"},
		{`scatter (file in input_files) {
			call process_file { input: file=file }
			call validate { input: result=process_file.output }
		}`, "scatter with multiple calls"},
		{`scatter (pair in zip(files, names)) {
			call process { input: file=pair.left, name=pair.right }
		}`, "scatter with zip"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseScatter()

		if !ok {
			t.Errorf("Failed to parse scatter: %s", test.description)
			continue
		}

		scatter, ok := result.(*tree.Scatter)
		if !ok {
			t.Errorf("Expected Scatter, got %T for %s", result, test.description)
			continue
		}

		if scatter.Variable() == "" {
			t.Errorf("Scatter variable should not be empty for %s", test.description)
		}
	}
}

func TestParseConditional(t *testing.T) {
	tests := []struct {
		input       string
		description string
	}{
		{`if (condition) {
			call optional_task
		}`, "simple conditional"},
		{`if (length(files) > 0) {
			call process_files { input: files=files }
		}`, "conditional with expression"},
		{`if (defined(optional_input)) {
			call process { input: data=optional_input }
			call validate { input: result=process.output }
		}`, "conditional with multiple calls"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseConditional()

		if !ok {
			t.Errorf("Failed to parse conditional: %s", test.description)
			continue
		}

		conditional, ok := result.(*tree.Conditional)
		if !ok {
			t.Errorf("Expected Conditional, got %T for %s", result, test.description)
			continue
		}

		if conditional.Condition() == nil {
			t.Errorf("Conditional condition should not be nil for %s", test.description)
		}
	}
}

func TestParseWorkflowElement(t *testing.T) {
	tests := []struct {
		input       string
		elementType string
		description string
	}{
		{`call task_name`, "call", "call element"},
		{`scatter (item in array) { call task }`, "scatter", "scatter element"},
		{`if (condition) { call task }`, "conditional", "conditional element"},
		{`String variable = "value"`, "declaration", "declaration element"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseWorkflowElement()

		if !ok {
			t.Errorf("Failed to parse workflow element: %s", test.description)
			continue
		}

		if result == nil {
			t.Errorf("Workflow element should not be nil for %s", test.description)
		}
	}
}

func TestParseAfterClause(t *testing.T) {
	tests := []struct {
		input       string
		expectedDeps int
		description string
	}{
		{`after task1`, 1, "single dependency"},
		{`after task1 task2 task3`, 3, "multiple dependencies"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseAfterClause()

		if !ok {
			t.Errorf("Failed to parse after clause: %s", test.description)
			continue
		}

		dependencies, ok := result.([]string)
		if !ok {
			t.Errorf("Expected []string, got %T for %s", result, test.description)
			continue
		}

		if len(dependencies) != test.expectedDeps {
			t.Errorf("%s: expected %d dependencies, got %d", 
				test.description, test.expectedDeps, len(dependencies))
		}
	}
}

func TestIsWorkflowElementStart(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"call", true},
		{"scatter", true},
		{"if", true},
		{"String", true}, // Declaration
		{"Int", true},    // Declaration
		{"workflow", false},
		{"task", false},
		{"{", false},
		{"42", false},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result := parser.isWorkflowElementStart()

		if result != test.expected {
			t.Errorf("Input '%s': isWorkflowElementStart() expected %t, got %t", 
				test.input, test.expected, result)
		}
	}
}

func TestParseWorkflowName(t *testing.T) {
	tests := []struct {
		input       string
		expectedName string
		valid       bool
	}{
		{"valid_workflow", "valid_workflow", true},
		{"WorkflowName", "WorkflowName", true},
		{"workflow123", "workflow123", true},
		{"_private_workflow", "_private_workflow", true},
		{"task", "", false}, // Reserved keyword
		{"123invalid", "", false}, // Starts with number
		{"", "", false}, // Empty name
	}

	for _, test := range tests {
		parser := NewParser(test.input+" {}", "test.wdl")
		result, ok := parser.parseWorkflowName()

		if test.valid && !ok {
			t.Errorf("Expected workflow name '%s' to be valid", test.input)
			continue
		}

		if !test.valid && ok {
			t.Errorf("Expected workflow name '%s' to be invalid", test.input)
			continue
		}

		if test.valid && result != test.expectedName {
			t.Errorf("Input '%s': expected name '%s', got '%s'", 
				test.input, test.expectedName, result)
		}
	}
}

func TestParseInputMapping(t *testing.T) {
	tests := []struct {
		input         string
		expectedPairs int
		description   string
	}{
		{`file=input_file, prefix="output"`, 2, "simple mapping"},
		{`input_file=file, config=workflow_config, debug=true`, 3, "multiple mappings"},
		{`single_param=value`, 1, "single mapping"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseInputMapping()

		if !ok {
			t.Errorf("Failed to parse input mapping: %s", test.description)
			continue
		}

		mapping, ok := result.(map[string]interface{})
		if !ok {
			t.Errorf("Expected map[string]interface{}, got %T for %s", result, test.description)
			continue
		}

		if len(mapping) != test.expectedPairs {
			t.Errorf("%s: expected %d mappings, got %d", 
				test.description, test.expectedPairs, len(mapping))
		}
	}
}

func TestValidateWorkflowStructure(t *testing.T) {
	tests := []struct {
		workflow    *tree.Workflow
		valid       bool
		description string
	}{
		// Note: These would need actual Workflow objects, which require proper construction
		// For now, we'll test the validation logic conceptually
	}

	for _, test := range tests {
		parser := NewParser("", "test.wdl")
		result := parser.validateWorkflowStructure(test.workflow)

		if result != test.valid {
			t.Errorf("%s: expected valid=%t, got valid=%t", 
				test.description, test.valid, result)
		}
	}
}

func TestWorkflowParseErrors(t *testing.T) {
	tests := []struct {
		input       string
		description string
	}{
		{"workflow {}", "missing workflow name"},
		{"workflow name", "missing workflow body"},
		{"workflow name { invalid_element }", "invalid workflow element"},
		{"workflow name { call }", "incomplete call"},
		{"workflow name { scatter item in array }", "malformed scatter"},
		{"workflow name { if condition }", "malformed conditional"},
		{`workflow name { 
			call task1
			call task1
		}`, "duplicate call names"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseWorkflow()

		if ok {
			t.Errorf("Expected parsing '%s' to fail (%s), but got: %T", 
				test.input, test.description, result)
		}

		// Check that error was recorded
		if !parser.HasErrors() {
			t.Errorf("Expected error to be recorded when parsing '%s'", test.input)
		}
	}
}

func TestCallWithNamespace(t *testing.T) {
	tests := []struct {
		input           string
		expectedNamespace string
		expectedTask    string
		description     string
	}{
		{`call my_namespace.task_name`, "my_namespace", "task_name", "simple namespace"},
		{`call utils.string.split`, "utils.string", "split", "nested namespace"},
		{`call task_name`, "", "task_name", "no namespace"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseCall()

		if !ok {
			t.Errorf("Failed to parse call: %s", test.description)
			continue
		}

		call, ok := result.(*tree.Call)
		if !ok {
			t.Errorf("Expected Call, got %T for %s", result, test.description)
			continue
		}

		if call.Namespace() != test.expectedNamespace {
			t.Errorf("%s: expected namespace '%s', got '%s'", 
				test.description, test.expectedNamespace, call.Namespace())
		}

		if call.Task() != test.expectedTask {
			t.Errorf("%s: expected task '%s', got '%s'", 
				test.description, test.expectedTask, call.Task())
		}
	}
}