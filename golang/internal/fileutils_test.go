package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCollectDirEntries(t *testing.T) {
	t.Run("collect dir entries", func(t *testing.T) {
		m := map[string]interface{}{
			"Files": []interface{}{
				map[string]interface{}{
					"class":    "File",
					"location": "file:///tmp/test.txt",
				},
				map[string]interface{}{
					"class":    "Directory",
					"location": "file:///tmp/testdir",
				},
			},
			"Directory": map[string]interface{}{
				"class":    "Directory",
				"location": "file:///tmp/testdir2",
				"listing": []interface{}{
					map[string]interface{}{
						"class":    "File",
						"location": "file:///tmp/testdir2/test.txt",
					},
				},
			},
			"File": map[string]interface{}{
				"class":    "File",
				"location": "file:///tmp/test.bam",
				"secondaryFiles": []interface{}{
					map[string]interface{}{
						"class":    "File",
						"location": "file:///tmp/test.bam.bai",
					},
				},
			},
		}
		files := CollectDirEntries(m)
		assert.Equal(t, 4, len(files))
		assert.ElementsMatch(t, []string{"file:///tmp/test.bam", "file:///tmp/testdir", "file:///tmp/test.txt", "file:///tmp/testdir2"}, []string{files[0].GetLocation(), files[1].GetLocation(), files[2].GetLocation(), files[3].GetLocation()})
	})
}
