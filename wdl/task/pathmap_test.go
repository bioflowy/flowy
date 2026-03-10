package task_test

import (
	"strings"
	"testing"

	"github.com/bioflowy/flowy/wdl/task"
	"github.com/bioflowy/flowy/wdl/types"
	"github.com/bioflowy/flowy/wdl/values"
)

func TestPathMapAddFile(t *testing.T) {
	tests := []struct {
		name         string
		hostPath     string
		wantContains string // container path should contain this
	}{
		{
			name:         "simple file",
			hostPath:     "/host/data/input.bam",
			wantContains: "input.bam",
		},
		{
			name:         "nested path",
			hostPath:     "/host/deep/nested/file.txt",
			wantContains: "file.txt",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pm := task.NewPathMap()
			container := pm.AddFile(tc.hostPath)
			if !strings.HasPrefix(container, "/mnt/inputs/") {
				t.Errorf("container path %q does not start with /mnt/inputs/", container)
			}
			if !strings.Contains(container, tc.wantContains) {
				t.Errorf("container path %q does not contain %q", container, tc.wantContains)
			}
		})
	}
}

func TestPathMapAddFileIdempotent(t *testing.T) {
	pm := task.NewPathMap()
	path1 := pm.AddFile("/host/data/input.bam")
	path2 := pm.AddFile("/host/data/input.bam")
	if path1 != path2 {
		t.Errorf("AddFile not idempotent: first=%q second=%q", path1, path2)
	}
}

func TestPathMapAddFileCollision(t *testing.T) {
	pm := task.NewPathMap()
	// Two different host paths with the same basename
	path1 := pm.AddFile("/host/data/input.bam")
	path2 := pm.AddFile("/host/other/input.bam")
	if path1 == path2 {
		t.Errorf("collision not handled: both got %q", path1)
	}
	// Both should still be under /mnt/inputs/
	if !strings.HasPrefix(path1, "/mnt/inputs/") {
		t.Errorf("path1 %q not under /mnt/inputs/", path1)
	}
	if !strings.HasPrefix(path2, "/mnt/inputs/") {
		t.Errorf("path2 %q not under /mnt/inputs/", path2)
	}
}

func TestPathMapAddDir(t *testing.T) {
	pm := task.NewPathMap()
	container := pm.AddDir("/host/refdata/")
	if !strings.HasPrefix(container, "/mnt/inputs/") {
		t.Errorf("container dir %q does not start with /mnt/inputs/", container)
	}
	if !strings.HasSuffix(container, "/") {
		t.Errorf("container dir %q does not end with /", container)
	}
	if !strings.Contains(container, "refdata") {
		t.Errorf("container dir %q does not contain 'refdata'", container)
	}
}

func TestPathMapTranslateValue(t *testing.T) {
	pm := task.NewPathMap()
	hostPath := "/host/data/input.bam"
	containerPath := pm.AddFile(hostPath)

	v := pm.TranslateValue(values.FileValue{Type: types.FileType{}, Value: hostPath})
	fv, ok := v.(values.FileValue)
	if !ok {
		t.Fatalf("TranslateValue returned %T, want FileValue", v)
	}
	if fv.Value != containerPath {
		t.Errorf("TranslateValue got %q, want %q", fv.Value, containerPath)
	}
}

func TestPathMapUntranslateValue(t *testing.T) {
	pm := task.NewPathMap()
	hostPath := "/host/data/input.bam"
	containerPath := pm.AddFile(hostPath)

	v := pm.UntranslateValue(values.FileValue{Type: types.FileType{}, Value: containerPath})
	fv, ok := v.(values.FileValue)
	if !ok {
		t.Fatalf("UntranslateValue returned %T, want FileValue", v)
	}
	if fv.Value != hostPath {
		t.Errorf("UntranslateValue got %q, want %q", fv.Value, hostPath)
	}
}

func TestPathMapTranslateValueArray(t *testing.T) {
	pm := task.NewPathMap()
	host1 := "/host/data/a.bam"
	host2 := "/host/data/b.bam"
	c1 := pm.AddFile(host1)
	c2 := pm.AddFile(host2)

	arr := values.ArrayValue{
		Type: types.ArrayType{ItemType: types.FileType{}},
		Items: []values.Value{
			values.FileValue{Type: types.FileType{}, Value: host1},
			values.FileValue{Type: types.FileType{}, Value: host2},
		},
	}
	result := pm.TranslateValue(arr)
	av, ok := result.(values.ArrayValue)
	if !ok {
		t.Fatalf("TranslateValue(ArrayValue) returned %T", result)
	}
	if len(av.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(av.Items))
	}
	fv1, _ := av.Items[0].(values.FileValue)
	fv2, _ := av.Items[1].(values.FileValue)
	if fv1.Value != c1 {
		t.Errorf("item[0] got %q, want %q", fv1.Value, c1)
	}
	if fv2.Value != c2 {
		t.Errorf("item[1] got %q, want %q", fv2.Value, c2)
	}
}

func TestPathMapBinds(t *testing.T) {
	pm := task.NewPathMap()
	_ = pm.AddFile("/host/data/input.bam")
	_ = pm.AddDir("/host/refdata/")

	binds := pm.Binds()
	if len(binds) != 2 {
		t.Fatalf("expected 2 binds, got %d: %v", len(binds), binds)
	}
	for _, b := range binds {
		// Each bind should be "host:container" or "host:container:ro"
		if !strings.Contains(b, ":") {
			t.Errorf("bind spec %q does not contain ':'", b)
		}
	}
}

func TestTaskError(t *testing.T) {
	err := &task.TaskError{
		TaskName: "my_task",
		ExitCode: 1,
		Stderr:   []byte("error: file not found"),
	}
	msg := err.Error()
	if !strings.Contains(msg, "my_task") {
		t.Errorf("Error() %q does not contain task name", msg)
	}
	if !strings.Contains(msg, "1") {
		t.Errorf("Error() %q does not contain exit code", msg)
	}
	if !strings.Contains(msg, "error: file not found") {
		t.Errorf("Error() %q does not contain stderr text", msg)
	}
}
