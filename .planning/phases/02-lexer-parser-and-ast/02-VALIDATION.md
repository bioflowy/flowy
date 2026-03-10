---
phase: 2
slug: lexer-parser-and-ast
status: draft
nyquist_compliant: true
wave_0_complete: true
created: 2026-03-10
---

# Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — stdlib |
| **Quick run command** | `go test ./wdl/...` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./wdl/...`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 2-01-01 | 01 | 1 | PARSE-01 | unit | `go test ./wdl/lexer/... ./wdl/parser/...` | ❌ W0 | ⬜ pending |
| 2-01-02 | 01 | 1 | PARSE-02 | unit | `go test ./wdl/lexer/...` | ❌ W0 | ⬜ pending |
| 2-02-01 | 02 | 2 | PARSE-02 | unit | `go test ./wdl/lexer/...` | ❌ W0 | ⬜ pending |
| 2-02-02 | 02 | 2 | PARSE-03 | unit | `go test ./wdl/lexer/...` | ❌ W0 | ⬜ pending |
| 2-02-03 | 02 | 2 | PARSE-04 | unit | `go test ./wdl/lexer/...` | ❌ W0 | ⬜ pending |
| 2-03-01 | 03 | 3 | PARSE-08 | unit | `go test ./wdl/parser/... -run TestParseError` | ❌ W0 | ⬜ pending |
| 2-03-02 | 03 | 3 | PARSE-09 | unit | `go test ./wdl/parser/... -run TestStructDef` | ❌ W0 | ⬜ pending |
| 2-04-01 | 04 | 4 | PARSE-05 | unit | `go test ./wdl/parser/... -run TestStringInterpolation` | ❌ W0 | ⬜ pending |
| 2-04-02 | 04 | 4 | PARSE-06 | unit | `go test ./wdl/parser/... -run TestOperatorPrecedence` | ❌ W0 | ⬜ pending |
| 2-05-01 | 05 | 5 | PARSE-01 | integration | `go test ./wdl/parser/... -run TestParseDocument` | ❌ W0 | ⬜ pending |
| 2-05-02 | 05 | 5 | PARSE-07 | integration | `go test ./wdl/parser/... -run TestImportResolution` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `wdl/lexer/lexer_test.go` — stubs for PARSE-02, PARSE-03, PARSE-04 (created in Plan 01, Task 2)
- [ ] `wdl/parser/parser_test.go` — stubs for PARSE-01, PARSE-05, PARSE-06, PARSE-07, PARSE-08, PARSE-09 (created in Plan 01, Task 2)

> Note: No `wdl/ast/ast_test.go` is required. AST nodes are verified transitively through lexer and parser tests.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| rustc-style error display with source line | PARSE-08 | Output formatting — visual inspection needed | Parse a file with a deliberate syntax error; verify line/col and source snippet appear in error message |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 30s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
