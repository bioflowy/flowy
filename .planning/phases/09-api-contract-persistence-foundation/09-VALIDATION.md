---
phase: 9
slug: api-contract-persistence-foundation
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-11
---

# Phase 9 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go standard `testing` package (go test) |
| **Config file** | none — standard Go test tooling |
| **Quick run command** | `go test ./internal/... -count=1` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/... -count=1`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 09-01-01 | 01 | 1 | STR-01 | unit | `go test ./internal/config/... -run TestLoad -count=1` | ❌ W0 | ⬜ pending |
| 09-01-02 | 01 | 1 | STR-02 | unit | `go test ./internal/config/... -run TestSections -count=1` | ❌ W0 | ⬜ pending |
| 09-01-03 | 01 | 1 | STR-04 | unit | `go test ./internal/config/... -count=1` | ❌ W0 | ⬜ pending |
| 09-02-01 | 02 | 1 | STR-03 | unit | `go test ./internal/store/... -run TestRunDirs -count=1` | ❌ W0 | ⬜ pending |
| 09-02-02 | 02 | 1 | SRV-07 | integration | `go test ./internal/store/sqlite/... -run TestOpen -count=1` | ❌ W0 | ⬜ pending |
| 09-02-03 | 02 | 1 | SRV-07 | integration | `go test ./internal/store/sqlite/... -run TestSchema -count=1` | ❌ W0 | ⬜ pending |
| 09-02-04 | 02 | 1 | SRV-08 | integration | `go test ./internal/store/sqlite/... -run TestFindStaleTasks -count=1` | ❌ W0 | ⬜ pending |
| 09-03-01 | 03 | 1 | — | compile | `go build ./internal/api/...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/config/config_test.go` — stubs for STR-01, STR-02, STR-04
- [ ] `internal/store/sqlite/sqlite_test.go` — stubs for SRV-07, SRV-08, STR-03
- [ ] `internal/api/types_test.go` — compile check for DTO types (no import cycles)
- [ ] `go get modernc.org/sqlite@v1.46.1 github.com/pressly/goose/v3@v3.27.0 github.com/go-chi/chi/v5@latest` — must precede any store tests

*If none: "Existing infrastructure covers all phase requirements."*

---

## Manual-Only Verifications

*All phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
