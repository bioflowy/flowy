# Summary 39-01

- Reworked workflow completion in `internal/server/dispatch.go` to build one completion record and commit it through `store.CompleteRun(...)`.
- Added declared-output walking for `File`, `Directory`, `Array[File]`, and `Array[Directory]`.
- Deduplicated duplicate output paths inside a single run completion event.
- Added `internal/server/dispatch_test.go` to lock the completion and flattening behavior.
