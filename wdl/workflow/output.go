package workflow

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/bioflowy/flowy/wdl/task"
	"github.com/bioflowy/flowy/wdl/values"
)

// writeOutputsJSON writes workflow outputs to a JSON file in miniwdl-compatible format.
// Keys are formatted as "workflowName.outputName". File and Directory values are
// serialized as paths relative to execRoot. The file is written with 0644 permissions.
func writeOutputsJSON(wfName string, outputs map[string]values.Value, execRoot string, destPath string) error {
	data := make(map[string]interface{}, len(outputs))
	for name, v := range outputs {
		key := wfName + "." + name
		data[key] = valueToJSONOutput(v, execRoot)
	}

	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(destPath, encoded, 0644)
}

// valueToJSONOutput converts a WDL value to a JSON-compatible Go value for output serialization.
// File and Directory paths are converted to paths relative to execRoot.
// Other types follow the same rules as valueToJSON.
func valueToJSONOutput(v values.Value, execRoot string) interface{} {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case values.IntValue:
		return val.Value
	case values.FloatValue:
		return val.Value
	case values.StringValue:
		return val.Value
	case values.BooleanValue:
		return val.Value
	case values.NullValue:
		return nil
	case values.FileValue:
		rel, err := filepath.Rel(execRoot, val.Value)
		if err != nil {
			return val.Value
		}
		return rel
	case values.DirectoryValue:
		rel, err := filepath.Rel(execRoot, val.Value)
		if err != nil {
			return val.Value
		}
		return rel
	case values.ArrayValue:
		result := make([]interface{}, len(val.Items))
		for i, item := range val.Items {
			result[i] = valueToJSONOutput(item, execRoot)
		}
		return result
	case values.MapValue:
		result := make(map[string]interface{}, len(val.Entries))
		for _, entry := range val.Entries {
			k := valueToJSONOutput(entry.Key, execRoot)
			var keyStr string
			switch ks := k.(type) {
			case string:
				keyStr = ks
			default:
				keyStr = ""
			}
			result[keyStr] = valueToJSONOutput(entry.Value, execRoot)
		}
		return result
	case values.PairValue:
		return map[string]interface{}{
			"left":  valueToJSONOutput(val.Left, execRoot),
			"right": valueToJSONOutput(val.Right, execRoot),
		}
	case values.StructValue:
		result := make(map[string]interface{}, len(val.Fields))
		for name, fv := range val.Fields {
			result[name] = valueToJSONOutput(fv, execRoot)
		}
		return result
	default:
		return nil
	}
}

// errorOutput is the JSON structure written to error.json on task failure.
type errorOutput struct {
	Error    string `json:"error"`
	Task     string `json:"task"`
	ExitCode int    `json:"exit_code"`
}

// writeErrorJSON writes a structured error.json file describing a workflow failure.
// If err is a *task.TaskError, the task name and exit code are extracted from it.
// For any other error type, task is set to "" and exit_code to -1.
func writeErrorJSON(err error, destPath string) error {
	out := errorOutput{
		Error:    err.Error(),
		Task:     "",
		ExitCode: -1,
	}

	var taskErr *task.TaskError
	if errors.As(err, &taskErr) {
		out.Task = taskErr.TaskName
		out.ExitCode = taskErr.ExitCode
	}

	encoded, encErr := json.MarshalIndent(out, "", "  ")
	if encErr != nil {
		return encErr
	}
	return os.WriteFile(destPath, encoded, 0644)
}
