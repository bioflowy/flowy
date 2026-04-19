# Requirements: flowy v2.3 Managed File Labels & UX

**Defined:** 2026-04-19
**Core Value:** WDL 1.2で記述されたワークフローをDockerコンテナ上で正確に実行できること

## v2.3 Requirements

Requirements for the managed-file label and UX follow-on milestone. Each maps to roadmap phases.

### Label Templates

- [ ] **LABEL-01**: Workflow outputs can define a managed-file label template via `parameter_meta { output_name: { label: "..." } }`
- [ ] **LABEL-02**: Label templates support `${var}`, `${var%suffix}`, `${var%%suffix}`, `${var#prefix}`, and `${var##prefix}` against workflow input string values
- [ ] **LABEL-03**: Invalid, unsupported, or missing-variable label templates never invoke a shell and never fail workflow completion; the system falls back to a safe default label

### File Library UX

- [ ] **FILEUX-01**: User can search/filter managed files in the Files tab by label, file type, and path
- [ ] **FILEUX-02**: User can see whether a managed file came from workflow output or manual import, and can open the originating run when available
- [ ] **FILEUX-03**: Managed file picker in workflow execution shows the file label prominently while still exposing the underlying path

## v2.4+ Requirements

- Manual label editing for imported files
- Pagination and bulk actions for very large file libraries
- Wider shell-compatibility beyond the supported prefix/suffix trimming operators

## Out of Scope

| Feature | Reason |
|---------|--------|
| Executing a real shell or subprocess for label evaluation | Security boundary; evaluator must stay pure Go |
| Full POSIX shell parameter expansion compatibility | Too broad for this milestone; only the documented operators are needed |
| User-editable label templates from the browser | WDL-driven output labeling is the immediate need |
| Pagination in the Files tab | Search/filter is sufficient for the expected near-term library sizes |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| LABEL-01 | Phase 41 | Pending |
| LABEL-02 | Phase 41 | Pending |
| LABEL-03 | Phase 41 | Pending |
| FILEUX-02 | Phase 42 | Pending |
| FILEUX-01 | Phase 43 | Pending |
| FILEUX-03 | Phase 43 | Pending |

**Coverage:**
- v2.3 requirements: 6 total
- Mapped to phases: 6
- Unmapped: 0 ✓

---
*Requirements defined: 2026-04-19*
*Last updated: 2026-04-19 after initial definition*
