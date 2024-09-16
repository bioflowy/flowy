package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFileMap(t *testing.T) {
	t.Run("collect dir entries", func(t *testing.T) {
		m := map[string]interface{}{

			"class":    "File",
			"location": "file:///tmp/test.txt",
		}
		dm := DataMap(m)
		assert.Equal(t, "file:///tmp/test.txt", dm.GetLocation())
		var m2 map[string]interface{} = dm
		assert.Equal(t, "file:///tmp/test.txt", m2["location"])
	})
}
