---
phase: 8
slug: add-spectool-integration-for-wdl-specification-compliance-testing
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-11
---

# Phase 8 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing (`go test`) |
| **Config file** | none — standard `go test ./...` |
| **Quick run command** | `go test ./cmd/flowy/ ./wdl/workflow/` |
| **Full suite command** | `go test ./... && make spectool` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./cmd/flowy/ ./wdl/workflow/ -count=1`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `/gsd:verify-work`:** Full suite must be green (`go test ./... && make spectool`)
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 08-01-01 | 01 | 0 | Output format | unit | `go test ./wdl/workflow/ -run TestWriteOutputsJSON_FileAbsolutePath` | ❌ W0 | ⬜ pending |
| 08-01-02 | 01 | 0 | Stdout JSON | unit | `go test ./cmd/flowy/ -run TestRunWorkflowStdoutJSON` | ❌ W0 | ⬜ pending |
| 08-01-03 | 01 | 0 | Output flag | unit | `go test ./cmd/flowy/ -run TestRunWithOutputFlag` | ❌ W0 | ⬜ pending |
| 08-01-04 | 01 | 0 | Task stdout | unit | `go test ./cmd/flowy/ -run TestRunTaskStdoutJSON` | ❌ W0 | ⬜ pending |
| 08-01-05 | 01 | 0 | Clean JSON | unit | `go test ./cmd/flowy/ -run TestRunStdoutIsValidJSON` | ❌ W0 | ⬜ pending |
| 08-02-01 | 02 | 1 | Makefile | integration | `make spectool` | ❌ W0 | ⬜ pending |
| 08-02-02 | 02 | 1 | Spectool pass | integration | `make spectool` | ❌ depends on spectool | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] Update `TestWriteOutputsJSON_FileRelativePath` → `TestWriteOutputsJSON_FileAbsolutePath` in `wdl/workflow/output_test.go`
- [ ] `TestRunWorkflowStdoutJSON` — stdout is valid JSON, keys have `wfName.` prefix
- [ ] `TestRunWithOutputFlag` — `--output <path>` writes JSON to that file, NOT stdout
- [ ] `TestRunTaskStdoutJSON` — `--task` stdout JSON has `taskName.outputName` keys
- [ ] `TestRunStdoutIsValidJSON` — `json.Unmarshal(stdout)` succeeds, no extra text
- [ ] `Makefile` — targets: `build`, `spectool`; needs to exist before spectool integration

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| spectool passes non-excluded tests | Spec compliance | Requires spectool binary + wdl-spec clone | `cargo install spectool && make spectool` |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
