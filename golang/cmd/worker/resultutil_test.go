package main

import (
	"testing"

	"github.com/bioflowy/flowy/golang/internal"
	"github.com/google/go-cmp/cmp"
)

type Map1 map[string]interface{}

func SetLocation(file internal.FileOrDirectory) error {
	if file.HasPath() {
		file.SetLocation("s3://flowy/test/" + file.GetPath())
		file.ClearPath()
	}
	return nil
}
func SetLocationFile(file internal.File) error {
	if file.HasPath() {
		file.SetLocation("s3://flowy/test_file/" + file.GetPath())
		file.ClearPath()
	}
	return nil
}

func TestVisitFileOrDirectory(t *testing.T) {
	var map1 = map[string]interface{}{
		"class": "File",
		"path":  "test.txt",
	}
	expected := map[string]interface{}{
		"class":    "File",
		"location": "s3://flowy/test/test.txt",
	}
	internal.VisitFileOrDirectory(map1, true, SetLocation)
	if diff := cmp.Diff(map1, expected); diff != "" {
		t.Errorf("User value is mismatch (-actual +expected):\n%s", diff)
	}
}
func TestVisitFile(t *testing.T) {
	var map1 = map[string]interface{}{
		"class": "File",
		"path":  "test.txt",
	}
	expected := map[string]interface{}{
		"class":    "File",
		"location": "s3://flowy/test_file/test.txt",
	}
	internal.VisitFile(map1, true, SetLocationFile)
	if diff := cmp.Diff(map1, expected); diff != "" {
		t.Errorf("User value is mismatch (-actual +expected):\n%s", diff)
	}
}
