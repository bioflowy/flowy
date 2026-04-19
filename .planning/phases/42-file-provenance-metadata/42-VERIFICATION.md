# Verification 42

## Automated Checks

- `GOCACHE=/tmp/flowy-go-cache go test ./internal/server ./internal/store/sqlite`
- `cd web && npm test -- --run src/features/projects/__tests__/ProjectDetailPage.test.tsx src/features/projects/__tests__/useProjects.test.ts src/features/registry/__tests__/RunWorkflowDialog.test.tsx`

## Covered Behavior

- Managed files store explicit source metadata.
- Workflow-generated rows expose `run_id` for provenance links.
- Import paths keep sensible source values.
