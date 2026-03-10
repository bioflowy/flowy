package eval

// ExecContext carries the execution context for a running WDL task.
// Fields are empty when evaluation occurs outside a task context (e.g., workflow inputs).
type ExecContext struct {
	WorkDir    string // task execution directory; empty = not in task context
	StdoutPath string // path to captured stdout file; empty = not in task context
	StderrPath string // path to captured stderr file; empty = not in task context
}
