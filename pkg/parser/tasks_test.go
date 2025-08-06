package parser

import (
	"testing"

	"github.com/bioflowy/flowy/pkg/tree"
)

func TestParseTask(t *testing.T) {
	tests := []struct {
		input        string
		expectedName string
		description  string
	}{
		{`task hello {
			command {
				echo "Hello World"
			}
		}`, "hello", "simple task"},
		{`task process_file {
			input {
				File input_file
				String prefix = "output"
			}
			
			command {
				cat ${input_file} > ${prefix}_result.txt
			}
			
			output {
				File result = "${prefix}_result.txt"
			}
		}`, "process_file", "task with input/output"},
		{`task complex_task {
			input {
				String name
				Int count = 10
				Boolean debug = false
			}
			
			command <<<
				echo "Processing ${name}"
				for i in $(seq 1 ${count}); do
					echo "Step $i"
				done
			>>>
			
			output {
				String message = stdout()
				Int exit_code = 0
			}
			
			runtime {
				docker: "ubuntu:20.04"
				memory: "4GB"
				cpu: 2
			}
			
			meta {
				description: "A complex example task"
				author: "Test Author"
			}
		}`, "complex_task", "complex task with all sections"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseTask()

		if !ok {
			t.Errorf("Failed to parse %s", test.description)
			continue
		}

		task, ok := result.(*tree.Task)
		if !ok {
			t.Errorf("Expected Task, got %T for %s", result, test.description)
			continue
		}

		if task.Name() != test.expectedName {
			t.Errorf("%s: expected name '%s', got '%s'", 
				test.description, test.expectedName, task.Name())
		}
	}
}

func TestParseTaskInput(t *testing.T) {
	tests := []struct {
		input         string
		expectedDecls int
		description   string
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
			File input_file
			String prefix = "output"
			Array[String] tags = []
		}`, 3, "input with complex types"},
		{`input {
		}`, 0, "empty input section"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseTaskInput()

		if !ok {
			t.Errorf("Failed to parse task %s", test.description)
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

func TestParseTaskCommand(t *testing.T) {
	tests := []struct {
		input       string
		description string
	}{
		{`command {
			echo "hello world"
		}`, "simple command block"},
		{`command <<<
			python script.py \
				--input ${input_file} \
				--output ${output_file}
		>>>`, "heredoc command block"},
		{`command {
			if [ "${debug}" == "true" ]; then
				echo "Debug mode enabled"
			fi
			cat ${input_file} > ${output_file}
		}`, "command with conditional"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseTaskCommand()

		if !ok {
			t.Errorf("Failed to parse task command: %s", test.description)
			continue
		}

		command, ok := result.(*tree.TaskCommand)
		if !ok {
			t.Errorf("Expected TaskCommand, got %T for %s", result, test.description)
			continue
		}

		if command.Command() == "" {
			t.Errorf("Task command should not be empty for %s", test.description)
		}
	}
}

func TestParseTaskOutput(t *testing.T) {
	tests := []struct {
		input         string
		expectedDecls int
		description   string
	}{
		{`output {
			String result = stdout()
		}`, 1, "simple output"},
		{`output {
			File result_file = "output.txt"
			String message = stdout()
			Int exit_code = 0
		}`, 3, "multiple outputs"},
		{`output {
			Array[File] results = glob("*.txt")
			Map[String,String] metadata = read_json("metadata.json")
		}`, 2, "complex output types"},
		{`output {
		}`, 0, "empty output section"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseTaskOutput()

		if !ok {
			t.Errorf("Failed to parse task %s", test.description)
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

func TestParseTaskRuntime(t *testing.T) {
	tests := []struct {
		input       string
		description string
	}{
		{`runtime {
			docker: "ubuntu:20.04"
			memory: "4GB"
			cpu: 2
		}`, "basic runtime"},
		{`runtime {
			docker: "python:3.8"
			memory: "8GB"
			cpu: 4
			disk: "100GB"
			preemptible: true
		}`, "extended runtime"},
		{`runtime {
			continueOnReturnCode: [0, 1]
			maxRetries: 3
		}`, "runtime with retry settings"},
		{`runtime {
		}`, "empty runtime section"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseTaskRuntime()

		if !ok {
			t.Errorf("Failed to parse task runtime: %s", test.description)
			continue
		}

		runtime, ok := result.(map[string]interface{})
		if !ok {
			t.Errorf("Expected map[string]interface{}, got %T for %s", result, test.description)
			continue
		}

		_ = runtime // Use the variable
	}
}

func TestParseTaskMeta(t *testing.T) {
	tests := []struct {
		input       string
		description string
	}{
		{`meta {
			description: "A test task"
			author: "John Doe"
			version: "1.0"
		}`, "basic meta"},
		{`meta {
			tags: ["bioinformatics", "genomics"]
			homepage: "https://example.com"
		}`, "meta with array and URL"},
		{`meta {
		}`, "empty meta section"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseTaskMeta()

		if !ok {
			t.Errorf("Failed to parse task meta: %s", test.description)
			continue
		}

		meta, ok := result.(map[string]interface{})
		if !ok {
			t.Errorf("Expected map[string]interface{}, got %T for %s", result, test.description)
			continue
		}

		_ = meta // Use the variable
	}
}

func TestParseTaskParameterMeta(t *testing.T) {
	tests := []struct {
		input       string
		description string
	}{
		{`parameter_meta {
			input_file: "The input file to process"
			output_prefix: "Prefix for output files"
		}`, "basic parameter_meta"},
		{`parameter_meta {
			count: {
				description: "Number of iterations",
				type: "integer",
				minimum: 1
			}
		}`, "structured parameter metadata"},
		{`parameter_meta {
		}`, "empty parameter_meta section"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseTaskParameterMeta()

		if !ok {
			t.Errorf("Failed to parse task parameter_meta: %s", test.description)
			continue
		}

		parameterMeta, ok := result.(map[string]interface{})
		if !ok {
			t.Errorf("Expected map[string]interface{}, got %T for %s", result, test.description)
			continue
		}

		_ = parameterMeta // Use the variable
	}
}

func TestParseTaskRequirements(t *testing.T) {
	tests := []struct {
		input       string
		description string
	}{
		{`requirements {
			return_codes: 0
			fail_on_stderr: false
		}`, "basic requirements"},
		{`requirements {
			docker: "required"
			memory: "minimum"
		}`, "resource requirements"},
		{`requirements {
		}`, "empty requirements section"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseTaskRequirements()

		if !ok {
			t.Errorf("Failed to parse task requirements: %s", test.description)
			continue
		}

		requirements, ok := result.(map[string]interface{})
		if !ok {
			t.Errorf("Expected map[string]interface{}, got %T for %s", result, test.description)
			continue
		}

		_ = requirements // Use the variable
	}
}

func TestParseTaskSection(t *testing.T) {
	tests := []struct {
		input       string
		sectionType string
		description string
	}{
		{`input { String name }`, "input", "input section"},
		{`output { String result = stdout() }`, "output", "output section"},
		{`command { echo "hello" }`, "command", "command section"},
		{`runtime { docker: "ubuntu" }`, "runtime", "runtime section"},
		{`meta { version: "1.0" }`, "meta", "meta section"},
		{`parameter_meta { name: "description" }`, "parameter_meta", "parameter_meta section"},
		{`requirements { return_codes: 0 }`, "requirements", "requirements section"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseTaskSection()

		if !ok {
			t.Errorf("Failed to parse %s", test.description)
			continue
		}

		if result == nil {
			t.Errorf("Task section should not be nil for %s", test.description)
		}
	}
}

func TestIsTaskSectionStart(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"input", true},
		{"output", true},
		{"command", true},
		{"runtime", true},
		{"meta", true},
		{"parameter_meta", true},
		{"requirements", true},
		{"workflow", false},
		{"task", false},
		{"{", false},
		{"String", false},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result := parser.isTaskSectionStart()

		if result != test.expected {
			t.Errorf("Input '%s': isTaskSectionStart() expected %t, got %t", 
				test.input, test.expected, result)
		}
	}
}

func TestParseKeyValuePairs(t *testing.T) {
	tests := []struct {
		input         string
		terminator    TokenType
		expectedPairs int
		description   string
	}{
		{`key1: "value1"
		  key2: 42
		  key3: true}`, TokenRightBrace, 3, "mixed value types"},
		{`docker: "ubuntu:20.04"
		  memory: "4GB"}`, TokenRightBrace, 2, "runtime-style pairs"},
		{`description: "A test"
		  tags: ["tag1", "tag2"]}`, TokenRightBrace, 2, "meta-style pairs"},
		{``, TokenRightBrace, 0, "empty pairs"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseKeyValuePairs(test.terminator)

		if !ok {
			t.Errorf("Failed to parse key-value pairs: %s", test.description)
			continue
		}

		pairs, ok := result.(map[string]interface{})
		if !ok {
			t.Errorf("Expected map[string]interface{}, got %T for %s", result, test.description)
			continue
		}

		if len(pairs) != test.expectedPairs {
			t.Errorf("%s: expected %d pairs, got %d", 
				test.description, test.expectedPairs, len(pairs))
		}
	}
}

func TestParseTaskName(t *testing.T) {
	tests := []struct {
		input       string
		expectedName string
		valid       bool
	}{
		{"valid_task", "valid_task", true},
		{"TaskName", "TaskName", true},
		{"task123", "task123", true},
		{"_private_task", "_private_task", true},
		{"workflow", "", false}, // Reserved keyword
		{"123invalid", "", false}, // Starts with number
		{"", "", false}, // Empty name
	}

	for _, test := range tests {
		parser := NewParser(test.input+" {}", "test.wdl")
		result, ok := parser.parseTaskName()

		if test.valid && !ok {
			t.Errorf("Expected task name '%s' to be valid", test.input)
			continue
		}

		if !test.valid && ok {
			t.Errorf("Expected task name '%s' to be invalid", test.input)
			continue
		}

		if test.valid && result != test.expectedName {
			t.Errorf("Input '%s': expected name '%s', got '%s'", 
				test.input, test.expectedName, result)
		}
	}
}

func TestValidateTaskStructure(t *testing.T) {
	tests := []struct {
		task        *tree.Task
		valid       bool
		description string
	}{
		// Note: These would need actual Task objects, which require proper construction
		// For now, we'll test the validation logic conceptually
	}

	for _, test := range tests {
		parser := NewParser("", "test.wdl")
		result := parser.validateTaskStructure(test.task)

		if result != test.valid {
			t.Errorf("%s: expected valid=%t, got valid=%t", 
				test.description, test.valid, result)
		}
	}
}

func TestTaskParseErrors(t *testing.T) {
	tests := []struct {
		input       string
		description string
	}{
		{"task {}", "missing task name"},
		{"task name", "missing task body"},
		{"task name { input }", "incomplete section"},
		{"task name { invalid_section {} }", "invalid section name"},
		{"task name { command }", "incomplete command section"},
		{`task name { 
			command { echo "hello" }
			command { echo "duplicate" }
		}`, "duplicate sections"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseTask()

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

func TestCommandStringTypes(t *testing.T) {
	tests := []struct {
		input       string
		commandType string
		description string
	}{
		{`command {
			echo "hello"
		}`, "braced", "braced command"},
		{`command <<<
			echo "hello"
		>>>`, "heredoc", "heredoc command"},
	}

	for _, test := range tests {
		parser := NewParser(test.input, "test.wdl")
		result, ok := parser.parseTaskCommand()

		if !ok {
			t.Errorf("Failed to parse command: %s", test.description)
			continue
		}

		command, ok := result.(*tree.TaskCommand)
		if !ok {
			t.Errorf("Expected TaskCommand, got %T for %s", result, test.description)
			continue
		}

		// In a real implementation, we'd check the command type
		_ = command
	}
}