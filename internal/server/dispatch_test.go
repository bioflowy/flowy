package server

import (
	"testing"

	"github.com/bioflowy/flowy/wdl/ast"
	"github.com/bioflowy/flowy/wdl/types"
	"github.com/bioflowy/flowy/wdl/values"
)

func TestBuildWorkflowCompletionRecord_CollectsManagedFiles(t *testing.T) {
	t.Parallel()

	wf := &ast.Workflow{
		Name: "align",
		Outputs: []*ast.Decl{
			{Name: "reads", Type: types.FileType{}},
			{Name: "workdir", Type: types.DirectoryType{}},
			{Name: "lanes", Type: types.ArrayType{ItemType: types.FileType{}}},
			{Name: "dirs", Type: types.ArrayType{ItemType: types.DirectoryType{}}},
			{Name: "message", Type: types.StringType{}},
		},
		ParamMeta: map[string]any{
			"reads": map[string]any{"type": "fastq"},
			"lanes": map[string]any{"type": "fastq"},
			"dirs":  map[string]any{"type": "analysis-dir"},
		},
	}

	outputs := map[string]values.Value{
		"reads":   values.FileValue{Type: types.FileType{}, Value: "/data/reads.fastq.gz"},
		"workdir": values.DirectoryValue{Type: types.DirectoryType{}, Value: "/data/work"},
		"lanes": values.ArrayValue{
			Type: types.ArrayType{ItemType: types.FileType{}},
			Items: []values.Value{
				values.FileValue{Type: types.FileType{}, Value: "/data/lane-1.fastq.gz"},
				values.FileValue{Type: types.FileType{}, Value: "/data/lane-2.fastq.gz"},
				values.FileValue{Type: types.FileType{}, Value: "/data/reads.fastq.gz"},
			},
		},
		"dirs": values.ArrayValue{
			Type: types.ArrayType{ItemType: types.DirectoryType{}},
			Items: []values.Value{
				values.DirectoryValue{Type: types.DirectoryType{}, Value: "/data/work"},
				values.DirectoryValue{Type: types.DirectoryType{}, Value: "/data/archive"},
			},
		},
		"message": values.StringValue{Type: types.StringType{}, Value: "done"},
	}

	inputs := map[string]interface{}{
		"sample_id": "sample-42",
	}

	outputsJSON, files, err := buildWorkflowCompletionRecord("project-1", "run-1", wf.Name, wf, inputs, outputs)
	if err != nil {
		t.Fatalf("buildWorkflowCompletionRecord: %v", err)
	}
	if outputsJSON == "" {
		t.Fatal("expected outputs JSON")
	}
	if len(files) != 5 {
		t.Fatalf("expected 5 unique managed files, got %d", len(files))
	}

	byPath := make(map[string]string, len(files))
	byPathLabel := make(map[string]string, len(files))
	for _, file := range files {
		if file.ProjectID != "project-1" {
			t.Fatalf("file.ProjectID: got %q", file.ProjectID)
		}
		if file.RunID == nil || *file.RunID != "run-1" {
			t.Fatalf("file.RunID: got %+v", file.RunID)
		}
		byPath[file.Path] = file.FileType
		byPathLabel[file.Path] = file.Label
		if file.SourceType != "workflow" {
			t.Fatalf("file.SourceType: got %q", file.SourceType)
		}
	}

	if byPath["/data/reads.fastq.gz"] != "fastq" {
		t.Fatalf("reads file_type: got %q", byPath["/data/reads.fastq.gz"])
	}
	if byPath["/data/work"] != "" {
		t.Fatalf("directory file_type: got %q, want empty", byPath["/data/work"])
	}
	if byPath["/data/lane-1.fastq.gz"] != "fastq" {
		t.Fatalf("lane file_type: got %q", byPath["/data/lane-1.fastq.gz"])
	}
	if byPath["/data/archive"] != "analysis-dir" {
		t.Fatalf("dir array file_type: got %q", byPath["/data/archive"])
	}
	if byPathLabel["/data/reads.fastq.gz"] != "reads.fastq.gz" {
		t.Fatalf("reads label: got %q", byPathLabel["/data/reads.fastq.gz"])
	}
}

func TestCollectManagedFilesForWorkflowOutputs_IgnoresMalformedParamMeta(t *testing.T) {
	t.Parallel()

	wf := &ast.Workflow{
		Outputs: []*ast.Decl{
			{Name: "reads", Type: types.FileType{}},
			{Name: "optional_dir", Type: types.DirectoryType{Optional: true}},
		},
		ParamMeta: map[string]any{
			"reads":        "fastq",
			"optional_dir": map[string]any{"help": "not a type annotation"},
		},
	}

	files, err := collectManagedFilesForWorkflowOutputs("project-1", "run-1", wf, nil, map[string]values.Value{
		"reads":        values.FileValue{Type: types.FileType{}, Value: "/tmp/input.fastq"},
		"optional_dir": values.NullValue{},
	})
	if err != nil {
		t.Fatalf("collectManagedFilesForWorkflowOutputs: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 managed file, got %d", len(files))
	}
	if files[0].FileType != "" {
		t.Fatalf("expected empty file_type for malformed metadata, got %q", files[0].FileType)
	}
}

func TestCollectManagedFilesForWorkflowOutputs_RejectsMismatchedRuntimeValue(t *testing.T) {
	t.Parallel()

	wf := &ast.Workflow{
		Outputs: []*ast.Decl{
			{Name: "reads", Type: types.FileType{}},
		},
	}

	_, err := collectManagedFilesForWorkflowOutputs("project-1", "run-1", wf, nil, map[string]values.Value{
		"reads": values.StringValue{Type: types.StringType{}, Value: "/tmp/input.fastq"},
	})
	if err == nil {
		t.Fatal("expected mismatch error, got nil")
	}
}

func TestCollectManagedFilesForWorkflowOutputs_UsesLabelTemplates(t *testing.T) {
	t.Parallel()

	wf := &ast.Workflow{
		Inputs: []*ast.Decl{
			{Name: "sample_id", Type: types.StringType{}},
			{Name: "reads", Type: types.FileType{}},
		},
		Outputs: []*ast.Decl{
			{Name: "report", Type: types.FileType{}},
		},
		ParamMeta: map[string]any{
			"report": map[string]any{
				"label": "${sample_id}_${reads##/data/}.qc.txt",
			},
		},
	}

	files, err := collectManagedFilesForWorkflowOutputs("project-1", "run-1", wf, map[string]interface{}{
		"sample_id": "sample-42",
		"reads":     "/data/reads.fastq.gz",
	}, map[string]values.Value{
		"report": values.FileValue{Type: types.FileType{}, Value: "/tmp/report.txt"},
	})
	if err != nil {
		t.Fatalf("collectManagedFilesForWorkflowOutputs: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 managed file, got %d", len(files))
	}
	if files[0].Label != "sample-42_reads.fastq.gz.qc.txt" {
		t.Fatalf("label: got %q", files[0].Label)
	}
}

func TestCollectManagedFilesForWorkflowOutputs_FallsBackOnInvalidLabelTemplate(t *testing.T) {
	t.Parallel()

	wf := &ast.Workflow{
		Inputs: []*ast.Decl{
			{Name: "sample_id", Type: types.StringType{}},
		},
		Outputs: []*ast.Decl{
			{Name: "report", Type: types.FileType{}},
		},
		ParamMeta: map[string]any{
			"report": map[string]any{
				"label": "${missing}.txt",
			},
		},
	}

	files, err := collectManagedFilesForWorkflowOutputs("project-1", "run-1", wf, map[string]interface{}{
		"sample_id": "sample-42",
	}, map[string]values.Value{
		"report": values.FileValue{Type: types.FileType{}, Value: "/tmp/report.txt"},
	})
	if err != nil {
		t.Fatalf("collectManagedFilesForWorkflowOutputs: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 managed file, got %d", len(files))
	}
	if files[0].Label != "report.txt" {
		t.Fatalf("label fallback: got %q", files[0].Label)
	}
}
