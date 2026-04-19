# Verification 43

## Automated Checks

- `cd web && npm test -- --run src/features/projects/__tests__/ProjectDetailPage.test.tsx src/features/projects/__tests__/useProjects.test.ts src/features/registry/__tests__/RunWorkflowDialog.test.tsx`

## Covered Behavior

- Files tab filters by label, path, and type.
- Label-first file rows and picker rendering are exercised in tests.
- Manual path fallback remains available.
