---
phase: 30
slug: file-browsing
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-07
---

# Phase 30 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | vitest (frontend), go test (backend) |
| **Config file** | `web/vitest.config.ts`, Go test tooling built-in |
| **Quick run command** | `cd web && npx vitest run --reporter=verbose` |
| **Full suite command** | `cd web && npx vitest run --reporter=verbose && cd .. && go test ./internal/server/... -run TestHandle -v` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run quick command for changed layer (frontend or backend)
- **After every plan wave:** Run full suite command
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| TBD | TBD | TBD | FILE-01 | unit | `go test ./internal/server/... -run TestListRuns -v` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | FILE-02 | unit | `cd web && npx vitest run` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | FILE-03 | unit | `go test ./internal/server/... -run TestFileDownload -v` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | FILE-04 | unit | `cd web && npx vitest run` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] Backend test stubs for new handlers (list runs, file download)
- [ ] Frontend test directory `web/src/features/runs/__tests__/`

*Existing infrastructure covers framework and config.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| File download triggers browser save dialog | FILE-03 | Browser download behavior | Click Download button on a file output, verify browser download prompt appears |
| Inline text preview renders correctly | FILE-04 | Visual rendering verification | Click Preview on a small text file, verify code block appears below row with content |
| S3 backend download works | FILE-03 | Requires S3/MinIO backend | Configure S3 storage, run workflow, download output file through browser |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
