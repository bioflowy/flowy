package task

import (
	"testing"

	"github.com/bioflowy/flowy/wdl/types"
	"github.com/bioflowy/flowy/wdl/values"
)

func TestCoerceOutputValue(t *testing.T) {
	tests := []struct {
		name     string
		input    values.Value
		declType types.Type
		workDir  string
		wantVal  values.Value
	}{
		{
			name:     "relative path + File type resolves against workDir",
			input:    values.StringValue{Value: "output.txt"},
			declType: types.FileType{},
			workDir:  "/work",
			wantVal:  values.FileValue{Type: types.FileType{}, Value: "/work/output.txt"},
		},
		{
			name:     "relative path + Directory type resolves against workDir",
			input:    values.StringValue{Value: "results"},
			declType: types.DirectoryType{},
			workDir:  "/work",
			wantVal:  values.DirectoryValue{Type: types.DirectoryType{}, Value: "/work/results"},
		},
		{
			name:     "absolute path + File type is unchanged",
			input:    values.StringValue{Value: "/abs/file.txt"},
			declType: types.FileType{},
			workDir:  "/work",
			wantVal:  values.FileValue{Type: types.FileType{}, Value: "/abs/file.txt"},
		},
		{
			name:     "absolute path + Directory type is unchanged",
			input:    values.StringValue{Value: "/abs/dir"},
			declType: types.DirectoryType{},
			workDir:  "/work",
			wantVal:  values.DirectoryValue{Type: types.DirectoryType{}, Value: "/abs/dir"},
		},
		{
			name:     "String type returns original StringValue without coercion",
			input:    values.StringValue{Value: "hello"},
			declType: types.StringType{},
			workDir:  "/work",
			wantVal:  values.StringValue{Value: "hello"},
		},
		{
			name:     "non-string value is returned unchanged",
			input:    values.IntValue{Value: 42},
			declType: types.FileType{},
			workDir:  "/work",
			wantVal:  values.IntValue{Value: 42},
		},
		{
			name:     "empty workDir leaves relative path unresolved",
			input:    values.StringValue{Value: "output.txt"},
			declType: types.FileType{},
			workDir:  "",
			wantVal:  values.FileValue{Type: types.FileType{}, Value: "output.txt"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := coerceOutputValue(tc.input, tc.declType, tc.workDir)
			if got != tc.wantVal {
				t.Errorf("coerceOutputValue(%v, %T, %q)\n  got  %#v\n  want %#v",
					tc.input, tc.declType, tc.workDir, got, tc.wantVal)
			}
		})
	}
}
