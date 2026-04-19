# UI Spec: Phase 40

## Surfaces

- `ProjectDetailPage`
- `ImportFileDialog`
- `RunWorkflowDialog`

## Contracts

### ProjectDetailPage / Files Tab

- Must expose a visible `Files` tab beside `Members`.
- Must show columns for path, file type, size, and created date.
- Must provide `Import File` for `admin` and `executor`.
- Must confirm delete and clarify that only the record is removed.

### ImportFileDialog

- Must offer three explicit import modes: browser upload, S3 URI, server path.
- Must keep copy concise and operational rather than decorative.
- Must reuse existing dialog/footer/button interaction patterns.

### RunWorkflowDialog

- File inputs must show a managed-file picker before the manual path field.
- Picker must surface only matching `file_type` rows when a constraint exists.
- Manual text entry must remain available for server paths and advanced cases.
