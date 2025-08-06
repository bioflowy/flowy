package values

import (
	"testing"

	"github.com/bioflowy/flowy/pkg/types"
)

func TestRewritePaths(t *testing.T) {
	// Test rewriting File value
	fileVal := NewFile("/old/path/file.txt", false)
	rewriter := func(path string) string {
		if path == "/old/path/file.txt" {
			return "/new/path/file.txt"
		}
		return path
	}

	rewritten := RewritePaths(fileVal, rewriter)
	if fileRewritten, ok := rewritten.(*FileValue); ok {
		if fileRewritten.Value().(string) != "/new/path/file.txt" {
			t.Errorf("Expected '/new/path/file.txt', got '%s'", fileRewritten.Value())
		}
	} else {
		t.Error("Rewritten value should be FileValue")
	}

	// Test rewriting Directory value
	dirVal := NewDirectory("/old/dir", false)
	rewriter = func(path string) string {
		if path == "/old/dir" {
			return "/new/dir"
		}
		return path
	}

	rewritten = RewritePaths(dirVal, rewriter)
	if dirRewritten, ok := rewritten.(*DirectoryValue); ok {
		if dirRewritten.Value().(string) != "/new/dir" {
			t.Errorf("Expected '/new/dir', got '%s'", dirRewritten.Value())
		}
	} else {
		t.Error("Rewritten value should be DirectoryValue")
	}

	// Test that non-path values are not affected
	intVal := NewInt(42, false)
	rewritten = RewritePaths(intVal, rewriter)
	if intRewritten, ok := rewritten.(*IntValue); ok {
		if intRewritten.Value().(int64) != 42 {
			t.Errorf("Expected 42, got %v", intRewritten.Value())
		}
	} else {
		t.Error("Rewritten value should be IntValue")
	}
}

func TestRewritePathsInArray(t *testing.T) {
	fileType := types.NewFile(false)
	array := NewArray(fileType, false, false)
	array.Add(NewFile("/old/file1.txt", false))
	array.Add(NewFile("/old/file2.txt", false))
	array.Add(NewFile("/new/file3.txt", false))

	rewriter := func(path string) string {
		if len(path) > 4 && path[:4] == "/old" {
			return "/rewritten" + path[4:]
		}
		return path
	}

	rewritten := RewritePaths(array, rewriter)
	if arrayRewritten, ok := rewritten.(*ArrayValue); ok {
		items := arrayRewritten.Items()
		if len(items) != 3 {
			t.Fatalf("Expected 3 items, got %d", len(items))
		}

		// Check first item
		if file1, ok := items[0].(*FileValue); ok {
			if file1.Value().(string) != "/rewritten/file1.txt" {
				t.Errorf("Expected '/rewritten/file1.txt', got '%s'", file1.Value())
			}
		} else {
			t.Error("First item should be FileValue")
		}

		// Check second item
		if file2, ok := items[1].(*FileValue); ok {
			if file2.Value().(string) != "/rewritten/file2.txt" {
				t.Errorf("Expected '/rewritten/file2.txt', got '%s'", file2.Value())
			}
		} else {
			t.Error("Second item should be FileValue")
		}

		// Check third item (unchanged)
		if file3, ok := items[2].(*FileValue); ok {
			if file3.Value().(string) != "/new/file3.txt" {
				t.Errorf("Expected '/new/file3.txt', got '%s'", file3.Value())
			}
		} else {
			t.Error("Third item should be FileValue")
		}
	} else {
		t.Error("Rewritten value should be ArrayValue")
	}
}

func TestRewritePathsInMap(t *testing.T) {
	stringType := types.NewString(false)
	fileType := types.NewFile(false)
	mapVal := NewMap(stringType, fileType, false)
	mapVal.Set("file1", NewFile("/old/file1.txt", false))
	mapVal.Set("file2", NewFile("/old/file2.txt", false))
	mapVal.Set("file3", NewFile("/new/file3.txt", false))

	rewriter := func(path string) string {
		if len(path) > 4 && path[:4] == "/old" {
			return "/rewritten" + path[4:]
		}
		return path
	}

	rewritten := RewritePaths(mapVal, rewriter)
	if mapRewritten, ok := rewritten.(*MapValue); ok {
		// Check file1
		if file1, ok := mapRewritten.Get("file1"); ok {
			if fileVal, ok := file1.(*FileValue); ok {
				if fileVal.Value().(string) != "/rewritten/file1.txt" {
					t.Errorf("Expected '/rewritten/file1.txt', got '%s'", fileVal.Value())
				}
			} else {
				t.Error("file1 should be FileValue")
			}
		} else {
			t.Error("Expected to find key 'file1'")
		}

		// Check file3 (unchanged)
		if file3, ok := mapRewritten.Get("file3"); ok {
			if fileVal, ok := file3.(*FileValue); ok {
				if fileVal.Value().(string) != "/new/file3.txt" {
					t.Errorf("Expected '/new/file3.txt', got '%s'", fileVal.Value())
				}
			} else {
				t.Error("file3 should be FileValue")
			}
		} else {
			t.Error("Expected to find key 'file3'")
		}
	} else {
		t.Error("Rewritten value should be MapValue")
	}
}

func TestRewritePathsInPair(t *testing.T) {
	fileType := types.NewFile(false)
	dirType := types.NewDirectory(false)

	left := NewFile("/old/file.txt", false)
	right := NewDirectory("/old/dir", false)
	pairVal := NewPair(fileType, dirType, left, right, false)

	rewriter := func(path string) string {
		if len(path) > 4 && path[:4] == "/old" {
			return "/new" + path[4:]
		}
		return path
	}

	rewritten := RewritePaths(pairVal, rewriter)
	if pairRewritten, ok := rewritten.(*PairValue); ok {
		if fileLeft, ok := pairRewritten.Left().(*FileValue); ok {
			if fileLeft.Value().(string) != "/new/file.txt" {
				t.Errorf("Expected '/new/file.txt', got '%s'", fileLeft.Value())
			}
		} else {
			t.Error("Left should be FileValue")
		}

		if dirRight, ok := pairRewritten.Right().(*DirectoryValue); ok {
			if dirRight.Value().(string) != "/new/dir" {
				t.Errorf("Expected '/new/dir', got '%s'", dirRight.Value())
			}
		} else {
			t.Error("Right should be DirectoryValue")
		}
	} else {
		t.Error("Rewritten value should be PairValue")
	}
}

func TestRewritePathsInStruct(t *testing.T) {
	memberTypes := map[string]types.Base{
		"name":   types.NewString(false),
		"file":   types.NewFile(false),
		"folder": types.NewDirectory(false),
	}

	members := map[string]Base{
		"name":   NewString("test", false),
		"file":   NewFile("/old/file.txt", false),
		"folder": NewDirectory("/old/dir", false),
	}

	structVal := NewStruct("FileInfo", memberTypes, members, false)

	rewriter := func(path string) string {
		if len(path) > 4 && path[:4] == "/old" {
			return "/new" + path[4:]
		}
		return path
	}

	rewritten := RewritePaths(structVal, rewriter)
	if structRewritten, ok := rewritten.(*StructValue); ok {
		// Check name (unchanged)
		if nameVal, ok := structRewritten.Get("name"); ok {
			if strVal, ok := nameVal.(*StringValue); ok {
				if strVal.Value().(string) != "test" {
					t.Errorf("Expected 'test', got '%s'", strVal.Value())
				}
			} else {
				t.Error("name should be StringValue")
			}
		} else {
			t.Error("Expected to find member 'name'")
		}

		// Check file (rewritten)
		if fileVal, ok := structRewritten.Get("file"); ok {
			if file, ok := fileVal.(*FileValue); ok {
				if file.Value().(string) != "/new/file.txt" {
					t.Errorf("Expected '/new/file.txt', got '%s'", file.Value())
				}
			} else {
				t.Error("file should be FileValue")
			}
		} else {
			t.Error("Expected to find member 'file'")
		}

		// Check folder (rewritten)
		if folderVal, ok := structRewritten.Get("folder"); ok {
			if dir, ok := folderVal.(*DirectoryValue); ok {
				if dir.Value().(string) != "/new/dir" {
					t.Errorf("Expected '/new/dir', got '%s'", dir.Value())
				}
			} else {
				t.Error("folder should be DirectoryValue")
			}
		} else {
			t.Error("Expected to find member 'folder'")
		}
	} else {
		t.Error("Rewritten value should be StructValue")
	}
}

func TestMakePathRewriter(t *testing.T) {
	rewriter := MakePathRewriter("/old/base", "/new/base")

	// Test absolute path that starts with fromDir
	result := rewriter("/old/base/file.txt")
	expected := "/new/base/file.txt"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}

	// Test absolute path that doesn't start with fromDir
	result = rewriter("/other/path/file.txt")
	expected = "/other/path/file.txt"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}

	// Test relative path (unchanged)
	result = rewriter("relative/path/file.txt")
	expected = "relative/path/file.txt"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}

	// Test subdirectory
	result = rewriter("/old/base/subdir/file.txt")
	expected = "/new/base/subdir/file.txt"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}
