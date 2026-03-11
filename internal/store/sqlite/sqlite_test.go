package sqlite_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/bioflowy/flowy/internal/store"
	"github.com/bioflowy/flowy/internal/store/sqlite"
)

// openTestStore opens an in-memory SQLite store for testing.
// Each call returns a fresh isolated store.
func openTestStore(t *testing.T) *sqlite.SQLiteStore {
	t.Helper()
	s, err := sqlite.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// newRun creates a Run with required fields set.
func newRun(id string) *store.Run {
	return &store.Run{
		ID:         id,
		WDLPath:    "/path/to/workflow.wdl",
		InputsJSON: `{"input": "value"}`,
		State:      store.RunPending,
	}
}

// TestOpen verifies that OpenMemory succeeds and that migrations create the
// runs and tasks tables.
func TestOpen(t *testing.T) {
	s := openTestStore(t)
	db := s.WriterDB()

	for _, table := range []string{"runs", "tasks"} {
		rows, err := db.QueryContext(context.Background(),
			"SELECT name FROM pragma_table_info(?)", table)
		if err != nil {
			t.Fatalf("pragma_table_info(%s): %v", table, err)
		}
		var cols []string
		for rows.Next() {
			var col string
			if err := rows.Scan(&col); err != nil {
				t.Fatal(err)
			}
			cols = append(cols, col)
		}
		_ = rows.Close()
		if len(cols) == 0 {
			t.Errorf("table %q does not exist or has no columns", table)
		}
	}
}

// TestSchema verifies that the tasks table contains last_heartbeat_at and
// shard_index columns.
func TestSchema(t *testing.T) {
	s := openTestStore(t)
	db := s.WriterDB()

	rows, err := db.QueryContext(context.Background(),
		"SELECT name FROM pragma_table_info('tasks')")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	colSet := make(map[string]bool)
	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err != nil {
			t.Fatal(err)
		}
		colSet[col] = true
	}

	for _, required := range []string{"last_heartbeat_at", "shard_index"} {
		if !colSet[required] {
			t.Errorf("tasks table missing column %q", required)
		}
	}
}

// TestCreateAndGetRun verifies that a run can be stored and retrieved by ID.
func TestCreateAndGetRun(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	r := newRun("run-0001")
	if err := s.CreateRun(ctx, r); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	got, err := s.GetRun(ctx, "run-0001")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.ID != r.ID {
		t.Errorf("ID: got %q, want %q", got.ID, r.ID)
	}
	if got.WDLPath != r.WDLPath {
		t.Errorf("WDLPath: got %q, want %q", got.WDLPath, r.WDLPath)
	}
	if got.State != store.RunPending {
		t.Errorf("State: got %q, want PENDING", got.State)
	}
}

// TestGetRunByPrefix verifies prefix lookup and ambiguity detection.
func TestGetRunByPrefix(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	// Use UUIDs-like IDs to test 8-char prefix lookup.
	r1 := newRun("abcdef01-0000-0000-0000-000000000001")
	r2 := newRun("abcdef02-0000-0000-0000-000000000002")
	r3 := newRun("11111111-0000-0000-0000-000000000003")

	for _, r := range []*store.Run{r1, r2, r3} {
		if err := s.CreateRun(ctx, r); err != nil {
			t.Fatalf("CreateRun(%s): %v", r.ID, err)
		}
	}

	// Unique prefix — should succeed.
	got, err := s.GetRunByPrefix(ctx, "11111111")
	if err != nil {
		t.Fatalf("GetRunByPrefix unique: %v", err)
	}
	if got.ID != r3.ID {
		t.Errorf("got ID %q, want %q", got.ID, r3.ID)
	}

	// Ambiguous prefix — should fail.
	_, err = s.GetRunByPrefix(ctx, "abcdef0")
	if err == nil {
		t.Error("expected error for ambiguous prefix, got nil")
	}

	// Non-existent prefix — should fail.
	_, err = s.GetRunByPrefix(ctx, "ffffffff")
	if err == nil {
		t.Error("expected error for missing prefix, got nil")
	}
}

// TestUpdateRunState verifies that run state transitions are persisted.
func TestUpdateRunState(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	r := newRun("run-state-01")
	if err := s.CreateRun(ctx, r); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateRunState(ctx, r.ID, store.RunRunning); err != nil {
		t.Fatalf("UpdateRunState: %v", err)
	}
	got, err := s.GetRun(ctx, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != store.RunRunning {
		t.Errorf("State: got %q, want RUNNING", got.State)
	}
}

// TestCreateTask verifies that a task can be persisted and listed.
func TestCreateTask(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	r := newRun("run-task-01")
	if err := s.CreateRun(ctx, r); err != nil {
		t.Fatal(err)
	}

	task := &store.Task{
		RunID:      r.ID,
		TaskName:   "hello",
		ShardIndex: 0,
	}
	if err := s.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if task.ID == 0 {
		t.Error("expected task.ID to be set after CreateTask")
	}

	tasks, err := s.ListTasksByRun(ctx, r.ID)
	if err != nil {
		t.Fatalf("ListTasksByRun: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].TaskName != "hello" {
		t.Errorf("TaskName: got %q, want %q", tasks[0].TaskName, "hello")
	}
}

// TestClaimTask verifies that ClaimTask transitions a task to CLAIMED with
// daemon_id set and returns the task.
func TestClaimTask(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	r := newRun("run-claim-01")
	if err := s.CreateRun(ctx, r); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 2; i++ {
		if err := s.CreateTask(ctx, &store.Task{
			RunID: r.ID, TaskName: "scatter", ShardIndex: i,
		}); err != nil {
			t.Fatal(err)
		}
	}

	task, err := s.ClaimTask(ctx, "daemon-A")
	if err != nil {
		t.Fatalf("ClaimTask: %v", err)
	}
	if task == nil {
		t.Fatal("expected a task, got nil")
	}
	if task.State != store.TaskClaimed {
		t.Errorf("State: got %q, want CLAIMED", task.State)
	}
	if task.DaemonID != "daemon-A" {
		t.Errorf("DaemonID: got %q, want daemon-A", task.DaemonID)
	}
}

// TestClaimTaskAtomicNoRace verifies that two concurrent ClaimTask calls on a
// single PENDING task result in exactly one successful claim.
func TestClaimTaskAtomicNoRace(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	r := newRun("run-race-01")
	if err := s.CreateRun(ctx, r); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateTask(ctx, &store.Task{
		RunID: r.ID, TaskName: "only-one", ShardIndex: 0,
	}); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	results := make([]*store.Task, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			task, err := s.ClaimTask(ctx, "daemon-race")
			if err != nil {
				t.Errorf("ClaimTask[%d]: %v", idx, err)
				return
			}
			results[idx] = task
		}(i)
	}
	wg.Wait()

	claimed := 0
	for _, r := range results {
		if r != nil {
			claimed++
		}
	}
	if claimed != 1 {
		t.Errorf("expected exactly 1 claim, got %d", claimed)
	}
}

// TestUpdateHeartbeat verifies that the heartbeat timestamp is refreshed.
func TestUpdateHeartbeat(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	r := newRun("run-hb-01")
	if err := s.CreateRun(ctx, r); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateTask(ctx, &store.Task{
		RunID: r.ID, TaskName: "heartbeat-task", ShardIndex: 0,
	}); err != nil {
		t.Fatal(err)
	}

	task, err := s.ClaimTask(ctx, "daemon-B")
	if err != nil || task == nil {
		t.Fatalf("ClaimTask: err=%v task=%v", err, task)
	}

	first := task.LastHeartbeatAt
	time.Sleep(1100 * time.Millisecond) // ensure datetime resolution

	if err := s.UpdateHeartbeat(ctx, task.ID); err != nil {
		t.Fatalf("UpdateHeartbeat: %v", err)
	}

	tasks, err := s.ListTasksByRun(ctx, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if tasks[0].LastHeartbeatAt == nil {
		t.Fatal("LastHeartbeatAt is nil after UpdateHeartbeat")
	}
	if !tasks[0].LastHeartbeatAt.After(*first) {
		t.Errorf("heartbeat not advanced: first=%v second=%v",
			*first, *tasks[0].LastHeartbeatAt)
	}
}

// TestFindStaleTasks verifies that tasks with old heartbeats are returned.
func TestFindStaleTasks(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	db := s.WriterDB()

	r := newRun("run-stale-01")
	if err := s.CreateRun(ctx, r); err != nil {
		t.Fatal(err)
	}

	// Create two tasks.
	taskA := &store.Task{RunID: r.ID, TaskName: "stale-task", ShardIndex: 0}
	taskB := &store.Task{RunID: r.ID, TaskName: "fresh-task", ShardIndex: 1}
	for _, task := range []*store.Task{taskA, taskB} {
		if err := s.CreateTask(ctx, task); err != nil {
			t.Fatal(err)
		}
	}

	// Claim both tasks.
	claimedA, err := s.ClaimTask(ctx, "daemon-C")
	if err != nil || claimedA == nil {
		t.Fatalf("claim A: err=%v task=%v", err, claimedA)
	}
	claimedB, err := s.ClaimTask(ctx, "daemon-C")
	if err != nil || claimedB == nil {
		t.Fatalf("claim B: err=%v task=%v", err, claimedB)
	}

	// Simulate stale heartbeat on taskA by backdating it 120 seconds.
	_, err = db.ExecContext(ctx,
		"UPDATE tasks SET last_heartbeat_at = datetime('now', '-120 seconds') WHERE id = ?",
		claimedA.ID)
	if err != nil {
		t.Fatalf("backdate heartbeat: %v", err)
	}

	stale, err := s.FindStaleTasks(ctx, 60)
	if err != nil {
		t.Fatalf("FindStaleTasks: %v", err)
	}
	if len(stale) != 1 {
		t.Fatalf("expected 1 stale task, got %d", len(stale))
	}
	if stale[0].ID != claimedA.ID {
		t.Errorf("stale task ID: got %d, want %d", stale[0].ID, claimedA.ID)
	}
}

// TestGetTaskByID verifies that a task can be retrieved by its integer ID.
func TestGetTaskByID(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	r := newRun("run-gettask-01")
	if err := s.CreateRun(ctx, r); err != nil {
		t.Fatal(err)
	}

	task := &store.Task{RunID: r.ID, TaskName: "fetch-me", ShardIndex: 2}
	if err := s.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	// Fetch by ID — should return the exact task.
	got, err := s.GetTaskByID(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTaskByID: %v", err)
	}
	if got.ID != task.ID {
		t.Errorf("ID: got %d, want %d", got.ID, task.ID)
	}
	if got.TaskName != "fetch-me" {
		t.Errorf("TaskName: got %q, want %q", got.TaskName, "fetch-me")
	}
	if got.ShardIndex != 2 {
		t.Errorf("ShardIndex: got %d, want 2", got.ShardIndex)
	}
	if got.RunID != r.ID {
		t.Errorf("RunID: got %q, want %q", got.RunID, r.ID)
	}

	// Fetch nonexistent ID — should return a descriptive error, not nil task.
	_, err = s.GetTaskByID(ctx, 99999)
	if err == nil {
		t.Error("expected error for nonexistent task ID, got nil")
	}
}

// TestUpdateHeartbeatByDaemon verifies that a single call refreshes heartbeats
// for all CLAIMED/RUNNING tasks owned by the given daemon.
func TestUpdateHeartbeatByDaemon(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	r := newRun("run-hbd-01")
	if err := s.CreateRun(ctx, r); err != nil {
		t.Fatal(err)
	}

	// Create two tasks and claim both with the same daemon.
	for i := 0; i < 2; i++ {
		if err := s.CreateTask(ctx, &store.Task{
			RunID: r.ID, TaskName: "parallel", ShardIndex: i,
		}); err != nil {
			t.Fatal(err)
		}
	}

	taskA, err := s.ClaimTask(ctx, "daemon-HBD")
	if err != nil || taskA == nil {
		t.Fatalf("ClaimTask A: err=%v task=%v", err, taskA)
	}
	taskB, err := s.ClaimTask(ctx, "daemon-HBD")
	if err != nil || taskB == nil {
		t.Fatalf("ClaimTask B: err=%v task=%v", err, taskB)
	}

	firstA := taskA.LastHeartbeatAt
	firstB := taskB.LastHeartbeatAt
	time.Sleep(1100 * time.Millisecond)

	// Single call should update both tasks.
	if err := s.UpdateHeartbeatByDaemon(ctx, "daemon-HBD"); err != nil {
		t.Fatalf("UpdateHeartbeatByDaemon: %v", err)
	}

	tasks, err := s.ListTasksByRun(ctx, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
	for _, task := range tasks {
		if task.LastHeartbeatAt == nil {
			t.Errorf("task %d: LastHeartbeatAt is nil", task.ID)
			continue
		}
		var first *time.Time
		if task.ID == taskA.ID {
			first = firstA
		} else {
			first = firstB
		}
		if !task.LastHeartbeatAt.After(*first) {
			t.Errorf("task %d: heartbeat not advanced: first=%v second=%v",
				task.ID, *first, *task.LastHeartbeatAt)
		}
	}

	// Call with a daemon that has no active tasks — should not error.
	if err := s.UpdateHeartbeatByDaemon(ctx, "daemon-nonexistent"); err != nil {
		t.Errorf("UpdateHeartbeatByDaemon with no tasks: unexpected error: %v", err)
	}
}

// TestFindStaleTasksIncludesClaimed verifies that CLAIMED tasks with expired
// heartbeats are included in FindStaleTasks results.
func TestFindStaleTasksIncludesClaimed(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	db := s.WriterDB()

	r := newRun("run-stale-02")
	if err := s.CreateRun(ctx, r); err != nil {
		t.Fatal(err)
	}

	task := &store.Task{RunID: r.ID, TaskName: "claimed-stale", ShardIndex: 0}
	if err := s.CreateTask(ctx, task); err != nil {
		t.Fatal(err)
	}

	claimed, err := s.ClaimTask(ctx, "daemon-D")
	if err != nil || claimed == nil {
		t.Fatalf("ClaimTask: %v %v", err, claimed)
	}
	// State is CLAIMED — backdate heartbeat.
	_, err = db.ExecContext(ctx,
		"UPDATE tasks SET last_heartbeat_at = datetime('now', '-120 seconds') WHERE id = ?",
		claimed.ID)
	if err != nil {
		t.Fatal(err)
	}

	stale, err := s.FindStaleTasks(ctx, 60)
	if err != nil {
		t.Fatalf("FindStaleTasks: %v", err)
	}
	if len(stale) != 1 {
		t.Fatalf("expected 1 stale task, got %d", len(stale))
	}
	if stale[0].State != store.TaskClaimed {
		t.Errorf("expected CLAIMED stale task, got %q", stale[0].State)
	}
}
