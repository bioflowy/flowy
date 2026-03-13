package api_test

import (
	"encoding/json"
	"testing"

	"github.com/bioflowy/flowy/internal/api"
)

func roundTrip[T any](t *testing.T, v T) T {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}
	var out T
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}
	return out
}

func TestAPITypesJSON(t *testing.T) {
	t.Run("SubmitRequest", func(t *testing.T) {
		in := api.SubmitRequest{
			WDLPath:    "/workflows/hello.wdl",
			InputsJSON: map[string]interface{}{"sample": "foo"},
		}
		out := roundTrip(t, in)
		if out.WDLPath != in.WDLPath {
			t.Errorf("WDLPath = %q, want %q", out.WDLPath, in.WDLPath)
		}
	})

	t.Run("SubmitResponse", func(t *testing.T) {
		in := api.SubmitResponse{RunID: "abc-123"}
		out := roundTrip(t, in)
		if out.RunID != in.RunID {
			t.Errorf("RunID = %q, want %q", out.RunID, in.RunID)
		}
	})

	t.Run("StatusResponse", func(t *testing.T) {
		in := api.StatusResponse{RunID: "abc-123", State: "RUNNING"}
		out := roundTrip(t, in)
		if out.RunID != in.RunID || out.State != in.State {
			t.Errorf("StatusResponse mismatch: got %+v, want %+v", out, in)
		}
	})

	t.Run("OutputsResponse", func(t *testing.T) {
		in := api.OutputsResponse{
			RunID:   "abc-123",
			Outputs: map[string]interface{}{"result": "hello.txt"},
		}
		out := roundTrip(t, in)
		if out.RunID != in.RunID {
			t.Errorf("RunID = %q, want %q", out.RunID, in.RunID)
		}
		if out.Outputs["result"] != "hello.txt" {
			t.Errorf("Outputs[result] = %v, want %v", out.Outputs["result"], "hello.txt")
		}
	})

	t.Run("AbortResponse", func(t *testing.T) {
		in := api.AbortResponse{RunID: "abc-123", State: "ABORTED"}
		out := roundTrip(t, in)
		if out.RunID != in.RunID || out.State != in.State {
			t.Errorf("AbortResponse mismatch: got %+v, want %+v", out, in)
		}
	})

	t.Run("ErrorResponse", func(t *testing.T) {
		in := api.ErrorResponse{Error: "something went wrong"}
		out := roundTrip(t, in)
		if out.Error != in.Error {
			t.Errorf("Error = %q, want %q", out.Error, in.Error)
		}
	})

	t.Run("TaskClaimRequest", func(t *testing.T) {
		in := api.TaskClaimRequest{DaemonID: "daemon-001"}
		out := roundTrip(t, in)
		if out.DaemonID != in.DaemonID {
			t.Errorf("DaemonID = %q, want %q", out.DaemonID, in.DaemonID)
		}
	})

	t.Run("TaskClaimResponse", func(t *testing.T) {
		in := api.TaskClaimResponse{
			TaskID:     "42",
			RunID:      "abc-123",
			TaskName:   "hello",
			ShardIndex: 0,
			InputsDir:  "/data/runs/abc-123/inputs",
			OutputsDir: "/data/runs/abc-123/outputs",
		}
		out := roundTrip(t, in)
		if out.TaskID != in.TaskID || out.RunID != in.RunID || out.TaskName != in.TaskName {
			t.Errorf("TaskClaimResponse mismatch: got %+v, want %+v", out, in)
		}
		if out.InputsDir != in.InputsDir || out.OutputsDir != in.OutputsDir {
			t.Errorf("TaskClaimResponse dirs mismatch: got %+v, want %+v", out, in)
		}
	})

	t.Run("HeartbeatRequest", func(t *testing.T) {
		in := api.HeartbeatRequest{DaemonID: "daemon-001"}
		out := roundTrip(t, in)
		if out.DaemonID != in.DaemonID {
			t.Errorf("DaemonID = %q, want %q", out.DaemonID, in.DaemonID)
		}
	})

	t.Run("TaskResultRequest", func(t *testing.T) {
		in := api.TaskResultRequest{State: "COMPLETED", Error: ""}
		out := roundTrip(t, in)
		if out.State != in.State {
			t.Errorf("State = %q, want %q", out.State, in.State)
		}
	})
}

// TestTaskClaimResponse_InputsAndWDLContent verifies that TaskClaimResponse
// carries Inputs map and WDLContent string and they round-trip through JSON.
func TestTaskClaimResponse_InputsAndWDLContent(t *testing.T) {
	in := api.TaskClaimResponse{
		TaskID:     "42",
		RunID:      "abc-123",
		TaskName:   "hello",
		ShardIndex: 0,
		InputsDir:  "/data/runs/abc-123/tasks/hello_0",
		OutputsDir: "/data/runs/abc-123/tasks/hello_0",
		WDLContent: "version 1.0\ntask hello { command <<< echo hi >>> }",
		Inputs: map[string]interface{}{
			"name": "world",
		},
	}
	out := roundTrip(t, in)
	if out.WDLContent != in.WDLContent {
		t.Errorf("WDLContent = %q, want %q", out.WDLContent, in.WDLContent)
	}
	if out.Inputs == nil {
		t.Fatal("Inputs is nil after round-trip")
	}
	if out.Inputs["name"] != "world" {
		t.Errorf("Inputs[name] = %v, want %q", out.Inputs["name"], "world")
	}
}

// TestTaskResultRequest_Outputs verifies that TaskResultRequest carries
// Outputs map and it round-trips through JSON.
func TestTaskResultRequest_Outputs(t *testing.T) {
	in := api.TaskResultRequest{
		State: "COMPLETED",
		Outputs: map[string]interface{}{
			"out_file": "/tmp/result.txt",
		},
	}
	out := roundTrip(t, in)
	if out.Outputs == nil {
		t.Fatal("Outputs is nil after round-trip")
	}
	if out.Outputs["out_file"] != "/tmp/result.txt" {
		t.Errorf("Outputs[out_file] = %v, want %q", out.Outputs["out_file"], "/tmp/result.txt")
	}
}

// TestTaskClaimResponse_OmitEmptyInputs verifies that nil Inputs and empty
// WDLContent are omitted from JSON (omitempty behavior).
func TestTaskClaimResponse_OmitEmptyInputs(t *testing.T) {
	in := api.TaskClaimResponse{
		TaskID:   "1",
		RunID:    "run-1",
		TaskName: "t",
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	raw := string(data)
	// nil Inputs and empty WDLContent must not appear in JSON due to omitempty.
	if contains(raw, `"inputs"`) {
		t.Error("nil Inputs should be omitted from JSON (omitempty)")
	}
	if contains(raw, `"wdl_content"`) {
		t.Error("empty WDLContent should be omitted from JSON (omitempty)")
	}
}

// contains is a helper since strings.Contains is not imported.
func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
