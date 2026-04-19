# Summary 41-01

- Added `internal/server/managed_file_labels.go` with a safe evaluator for the supported expansion forms.
- Added `internal/server/parameter_meta.go` so `type` and `label` metadata use one defensive parser.
- Wired output registration to evaluate labels during workflow completion.
