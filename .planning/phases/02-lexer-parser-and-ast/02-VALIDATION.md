---
phase: 2
slug: lexer-parser-and-ast
status: draft
nyquist_compliant: false
wave_0_complete: false
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
| 2-01-01 | 01 | 0 | PARSE-01 | unit | `go test ./wdl/lexer/...` | ❌ W0 | ⬜ pending |
| 2-01-02 | 01 | 0 | PARSE-02 | unit | `go test ./wdl/lexer/...` | ❌ W0 | ⬜ pending |
| 2-01-03 | 01 | 0 | PARSE-03 | unit | `go test ./wdl/lexer/...` | ❌ W0 | ⬜ pending |
| 2-02-01 | 02 | 1 | PARSE-04 | unit | `go test ./wdl/ast/...` | ❌ W0 | ⬜ pending |
| 2-02-02 | 02 | 1 | PARSE-05 | unit | `go test ./wdl/ast/...` | ❌ W0 | ⬜ pending |
| 2-03-01 | 03 | 1 | PARSE-06 | unit | `go test ./wdl/parser/...` | ❌ W0 | ⬜ pending |
| 2-03-02 | 03 | 1 | PARSE-07 | unit | `go test ./wdl/parser/...` | ❌ W0 | ⬜ pending |
| 2-04-01 | 04 | 2 | PARSE-08 | integration | `go test ./wdl/parser/... -run TestImport` | ❌ W0 | ⬜ pending |
| 2-04-02 | 04 | 2 | PARSE-09 | integration | `go test ./wdl/parser/... -run TestError` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `wdl/lexer/lexer_test.go` — stubs for PARSE-01, PARSE-02, PARSE-03
- [ ] `wdl/ast/ast_test.go` — stubs for PARSE-04, PARSE-05
- [ ] `wdl/parser/parser_test.go` — stubs for PARSE-06, PARSE-07, PARSE-08, PARSE-09

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| rustc-style error display with source line | PARSE-09 | Output formatting — visual inspection needed | Parse a file with a deliberate syntax error; verify line/col and source snippet appear in error message |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
