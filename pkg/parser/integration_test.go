package parser

import (
	"testing"
)

// TestParserIntegrationBasic tests basic parsing capabilities
func TestParserIntegrationBasic(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		desc    string
	}{
		{
			name: "simple_workflow",
			input: `version 1.0

workflow simple {
	input {
		String name
		Int count = 5
	}
	
	output {
		String result = "Hello ${name}"
	}
}`,
			wantErr: false,
			desc:    "Simple workflow with input and output",
		},
		{
			name: "simple_task",
			input: `version 1.0

task hello {
	input {
		String name
	}
	
	command {
		echo "Hello ${name}"
	}
	
	output {
		String greeting = stdout()
	}
}`,
			wantErr: false,
			desc:    "Simple task with command",
		},
		{
			name: "invalid_syntax",
			input: `version 1.0

workflow invalid {
	input {
		String name =
	}
}`,
			wantErr: true,
			desc:    "Invalid syntax should produce errors",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser(tt.input, "test.wdl")
			doc, ok := parser.ParseDocumentInternal()

			if tt.wantErr {
				if ok && !parser.HasErrors() {
					t.Errorf("Expected parsing error for %s, but got none", tt.desc)
				}
				return
			}

			if !ok {
				t.Errorf("Failed to parse %s: %v", tt.desc, parser.Errors())
				return
			}

			if doc == nil {
				t.Errorf("Expected document for %s, got nil", tt.desc)
				return
			}

			if parser.HasErrors() {
				t.Errorf("Unexpected errors for %s: %v", tt.desc, parser.Errors())
			}
		})
	}
}

// TestParserIntegrationTypes tests type parsing
func TestParserIntegrationTypes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		desc  string
	}{
		{
			name: "primitive_types",
			input: `version 1.0

task test {
	input {
		String str
		Int num
		Boolean flag
		Float value
		File input_file
	}
}`,
			desc: "Basic primitive types",
		},
		{
			name: "compound_types",
			input: `version 1.0

task test {
	input {
		Array[String] strings
		Map[String,Int] counts
		Pair[String,Int] item
	}
}`,
			desc: "Compound types",
		},
		{
			name: "optional_types",
			input: `version 1.0

task test {
	input {
		String? optional_str
		Array[Int]+ nonempty_ints
	}
}`,
			desc: "Optional and nonempty types",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser(tt.input, "test.wdl")
			doc, ok := parser.ParseDocumentInternal()

			if !ok {
				t.Errorf("Failed to parse %s: %v", tt.desc, parser.Errors())
				return
			}

			if doc == nil {
				t.Errorf("Expected document for %s, got nil", tt.desc)
				return
			}

			if parser.HasErrors() {
				t.Errorf("Unexpected errors for %s: %v", tt.desc, parser.Errors())
			}

			// Verify we have at least one task
			if len(doc.Tasks) == 0 {
				t.Errorf("Expected at least one task for %s", tt.desc)
			}
		})
	}
}

// TestParserIntegrationExpressions tests expression parsing
func TestParserIntegrationExpressions(t *testing.T) {
	tests := []struct {
		name  string
		input string
		desc  string
	}{
		{
			name: "simple_expressions",
			input: `version 1.0

workflow test {
	input {
		Int a = 1 + 2
		String b = "hello" + " world"
		Boolean c = true && false
	}
}`,
			desc: "Simple binary expressions",
		},
		{
			name: "function_calls",
			input: `version 1.0

workflow test {
	input {
		String result = length("hello")
		Array[Int] numbers = range(10)
	}
}`,
			desc: "Function call expressions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser(tt.input, "test.wdl")
			doc, ok := parser.ParseDocumentInternal()

			if !ok {
				t.Errorf("Failed to parse %s: %v", tt.desc, parser.Errors())
				return
			}

			if doc == nil {
				t.Errorf("Expected document for %s, got nil", tt.desc)
				return
			}

			if parser.HasErrors() {
				t.Errorf("Unexpected errors for %s: %v", tt.desc, parser.Errors())
			}
		})
	}
}

// TestParserIntegrationErrorRecovery tests error recovery capabilities
func TestParserIntegrationErrorRecovery(t *testing.T) {
	input := `version 1.0

workflow test {
	input {
		String name =  # Missing expression
		Int count
	}
	
	output {
		String result = "Hello"
	}
}`

	parser := NewParser(input, "test.wdl")
	doc, ok := parser.ParseDocumentInternal()

	// Should have errors but still produce a partial document
	if !parser.HasErrors() {
		t.Error("Expected parsing errors for invalid syntax")
	}

	// Parser should still attempt to parse and may produce partial results
	_ = doc
	_ = ok

	// Verify error count is reasonable
	errors := parser.Errors()
	if len(errors) == 0 {
		t.Error("Expected at least one error")
	}

	// Verify errors have proper position information
	for i, err := range errors {
		if err.Pos.Line == 0 || err.Pos.Column == 0 {
			t.Errorf("Error %d missing position information: %v", i, err)
		}
	}
}

// TestParserRequirementsSection tests parsing of task requirements sections
func TestParserRequirementsSection(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantErr        bool
		expectedKeys   []string
		desc           string
	}{
		{
			name: "requirements_with_commas",
			input: `version 1.0

task test {
	input {
		String name
	}
	
	requirements {
		memory: "4G",
		cpu: 2,
		disk: "100G"
	}
	
	command {
		echo "Hello ${name}"
	}
	
	output {
		String result = stdout()
	}
}`,
			wantErr:      false,
			expectedKeys: []string{"memory", "cpu", "disk"},
			desc:         "Requirements section with commas between entries",
		},
		{
			name: "requirements_without_commas",
			input: `version 1.0

task test {
	input {
		String name
	}
	
	requirements {
		memory: "4G"
		cpu: 2
		disk: "100G"
	}
	
	command {
		echo "Hello ${name}"
	}
	
	output {
		String result = stdout()
	}
}`,
			wantErr:      false,
			expectedKeys: []string{"memory", "cpu", "disk"},
			desc:         "Requirements section without commas between entries",
		},
		{
			name: "requirements_mixed_commas",
			input: `version 1.0

task test {
	input {
		String name
	}
	
	requirements {
		memory: "4G",
		cpu: 2
		disk: "100G",
		preemptible: true
	}
	
	command {
		echo "Hello ${name}"
	}
	
	output {
		String result = stdout()
	}
}`,
			wantErr:      false,
			expectedKeys: []string{"memory", "cpu", "disk", "preemptible"},
			desc:         "Requirements section with mixed comma usage",
		},
		{
			name: "requirements_trailing_comma",
			input: `version 1.0

task test {
	input {
		String name
	}
	
	requirements {
		memory: "4G",
		cpu: 2,
	}
	
	command {
		echo "Hello ${name}"
	}
	
	output {
		String result = stdout()
	}
}`,
			wantErr:      false,
			expectedKeys: []string{"memory", "cpu"},
			desc:         "Requirements section with trailing comma",
		},
		{
			name: "runtime_section",
			input: `version 1.0

task test {
	input {
		String name
	}
	
	runtime {
		memory: "4G"
		cpu: 2
	}
	
	command {
		echo "Hello ${name}"
	}
	
	output {
		String result = stdout()
	}
}`,
			wantErr:      false,
			expectedKeys: []string{"memory", "cpu"},
			desc:         "Runtime section (legacy syntax) without commas",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser(tt.input, "test.wdl")
			doc, ok := parser.ParseDocumentInternal()

			if tt.wantErr {
				if ok {
					t.Errorf("Expected error for %s, but parsing succeeded", tt.desc)
					return
				}
			} else {
				if !ok {
					t.Errorf("Failed to parse %s: %v", tt.desc, parser.Errors())
					return
				}

				if doc == nil {
					t.Errorf("Expected document for %s, got nil", tt.desc)
					return
				}

				if parser.HasErrors() {
					t.Errorf("Unexpected errors for %s: %v", tt.desc, parser.Errors())
					return
				}

				// Verify that the task was parsed correctly
				if len(doc.Tasks) != 1 {
					t.Errorf("Expected 1 task for %s, got %d", tt.desc, len(doc.Tasks))
					return
				}

				task := doc.Tasks[0]
				if task.Runtime == nil {
					t.Errorf("Expected requirements/runtime section for %s, got nil", tt.desc)
					return
				}

				// Check that all expected keys are present
				for _, key := range tt.expectedKeys {
					if _, exists := task.Runtime[key]; !exists {
						t.Errorf("Missing expected key '%s' in requirements for %s", key, tt.desc)
					}
				}

				// Verify we have the expected number of requirements
				if len(task.Runtime) != len(tt.expectedKeys) {
					t.Errorf("Expected %d requirements for %s, got %d", len(tt.expectedKeys), tt.desc, len(task.Runtime))
				}
			}
		})
	}
}