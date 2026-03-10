package task

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bioflowy/flowy/wdl/values"
)

// PathMap maintains a bidirectional mapping between host filesystem paths and
// container-internal paths under /mnt/inputs/. It accumulates Docker bind-mount
// specifications for all registered files and directories.
type PathMap struct {
	hostToContainer map[string]string // host path -> container path
	containerToHost map[string]string // container path -> host path
	binds           []string          // accumulated Docker bind-mount specs
}

// NewPathMap creates an empty PathMap ready for use.
func NewPathMap() *PathMap {
	return &PathMap{
		hostToContainer: make(map[string]string),
		containerToHost: make(map[string]string),
	}
}

// AddFile registers a host file path and returns the corresponding container path.
// Container paths are assigned under /mnt/inputs/ using the file's basename.
// When two different host paths share the same basename, the second path receives
// a prefix derived from the first 8 hex characters of the host path's SHA-256 hash
// to avoid collisions (e.g., /mnt/inputs/a1b2c3d4-input.bam).
// Calling AddFile with the same host path twice is idempotent.
func (pm *PathMap) AddFile(hostPath string) string {
	if c, ok := pm.hostToContainer[hostPath]; ok {
		return c
	}
	base := filepath.Base(hostPath)
	containerPath := "/mnt/inputs/" + base
	if _, taken := pm.containerToHost[containerPath]; taken {
		// Collision: prefix with first 8 hex chars of SHA-256 of hostPath
		h := sha256.Sum256([]byte(hostPath))
		prefix := fmt.Sprintf("%x", h[:4]) // 4 bytes = 8 hex chars
		containerPath = "/mnt/inputs/" + prefix + "-" + base
	}
	pm.hostToContainer[hostPath] = containerPath
	pm.containerToHost[containerPath] = hostPath
	pm.binds = append(pm.binds, hostPath+":"+containerPath+":ro")
	return containerPath
}

// AddDir registers a host directory path and returns the corresponding container path.
// Directory container paths end with "/" and follow the same collision-avoidance
// strategy as AddFile. Calling AddDir with the same host path twice is idempotent.
func (pm *PathMap) AddDir(hostPath string) string {
	if c, ok := pm.hostToContainer[hostPath]; ok {
		return c
	}
	// Normalize: ensure host path ends with "/"
	normalHost := hostPath
	if !strings.HasSuffix(normalHost, "/") {
		normalHost = normalHost + "/"
	}
	// Use filepath.Base on the path without trailing slash to get dirname
	dirName := filepath.Base(strings.TrimSuffix(normalHost, "/"))
	containerPath := "/mnt/inputs/" + dirName + "/"
	if _, taken := pm.containerToHost[containerPath]; taken {
		h := sha256.Sum256([]byte(hostPath))
		prefix := fmt.Sprintf("%x", h[:4])
		containerPath = "/mnt/inputs/" + prefix + "-" + dirName + "/"
	}
	pm.hostToContainer[hostPath] = containerPath
	pm.containerToHost[containerPath] = hostPath
	pm.binds = append(pm.binds, strings.TrimSuffix(hostPath, "/")+":"+strings.TrimSuffix(containerPath, "/")+":ro")
	return containerPath
}

// TranslateValue replaces all FileValue and DirectoryValue host paths within v
// with their corresponding container paths. Compound values (Array, Map, Pair, Struct)
// are traversed recursively. Paths not registered in the PathMap are returned unchanged.
func (pm *PathMap) TranslateValue(v values.Value) values.Value {
	switch val := v.(type) {
	case values.FileValue:
		if c, ok := pm.hostToContainer[val.Value]; ok {
			return values.FileValue{Type: val.Type, Value: c}
		}
		return val
	case values.DirectoryValue:
		if c, ok := pm.hostToContainer[val.Value]; ok {
			return values.DirectoryValue{Type: val.Type, Value: c}
		}
		return val
	case values.ArrayValue:
		items := make([]values.Value, len(val.Items))
		for i, item := range val.Items {
			items[i] = pm.TranslateValue(item)
		}
		return values.ArrayValue{Type: val.Type, Items: items}
	case values.MapValue:
		entries := make([]values.MapEntry, len(val.Entries))
		for i, e := range val.Entries {
			entries[i] = values.MapEntry{
				Key:   pm.TranslateValue(e.Key),
				Value: pm.TranslateValue(e.Value),
			}
		}
		return values.MapValue{Type: val.Type, Entries: entries}
	case values.PairValue:
		return values.PairValue{
			Type:  val.Type,
			Left:  pm.TranslateValue(val.Left),
			Right: pm.TranslateValue(val.Right),
		}
	case values.StructValue:
		fields := make(map[string]values.Value, len(val.Fields))
		for k, fv := range val.Fields {
			fields[k] = pm.TranslateValue(fv)
		}
		return values.StructValue{Type: val.Type, Fields: fields}
	default:
		return v
	}
}

// UntranslateValue is the inverse of TranslateValue: it replaces container paths
// with the original host paths. Used when translating task output File values
// back to host paths after container execution.
func (pm *PathMap) UntranslateValue(v values.Value) values.Value {
	switch val := v.(type) {
	case values.FileValue:
		if h, ok := pm.containerToHost[val.Value]; ok {
			return values.FileValue{Type: val.Type, Value: h}
		}
		return val
	case values.DirectoryValue:
		if h, ok := pm.containerToHost[val.Value]; ok {
			return values.DirectoryValue{Type: val.Type, Value: h}
		}
		return val
	case values.ArrayValue:
		items := make([]values.Value, len(val.Items))
		for i, item := range val.Items {
			items[i] = pm.UntranslateValue(item)
		}
		return values.ArrayValue{Type: val.Type, Items: items}
	case values.MapValue:
		entries := make([]values.MapEntry, len(val.Entries))
		for i, e := range val.Entries {
			entries[i] = values.MapEntry{
				Key:   pm.UntranslateValue(e.Key),
				Value: pm.UntranslateValue(e.Value),
			}
		}
		return values.MapValue{Type: val.Type, Entries: entries}
	case values.PairValue:
		return values.PairValue{
			Type:  val.Type,
			Left:  pm.UntranslateValue(val.Left),
			Right: pm.UntranslateValue(val.Right),
		}
	case values.StructValue:
		fields := make(map[string]values.Value, len(val.Fields))
		for k, fv := range val.Fields {
			fields[k] = pm.UntranslateValue(fv)
		}
		return values.StructValue{Type: val.Type, Fields: fields}
	default:
		return v
	}
}

// Binds returns the accumulated Docker bind-mount specification strings.
// Each entry has the format "hostPath:containerPath:ro".
func (pm *PathMap) Binds() []string {
	result := make([]string, len(pm.binds))
	copy(result, pm.binds)
	return result
}
