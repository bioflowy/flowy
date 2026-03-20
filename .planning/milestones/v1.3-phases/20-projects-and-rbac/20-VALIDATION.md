---
phase: 20
slug: projects-and-rbac
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-17
---

# Phase 20 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing + httptest (stdlib) |
| **Config file** | none — `go test ./...` |
| **Quick run command** | `go test ./internal/store/sqlite/... ./internal/server/... -count=1` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/store/sqlite/... ./internal/server/... -count=1`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 20-01-01 | 01 | 1 | PROJ-01 | integration | `go test ./internal/server/... -run TestCreateProject -count=1` | ❌ W0 | ⬜ pending |
| 20-01-02 | 01 | 1 | PROJ-02 | integration | `go test ./internal/server/... -run TestListProjects -count=1` | ❌ W0 | ⬜ pending |
| 20-01-03 | 01 | 1 | PROJ-03 | integration | `go test ./internal/server/... -run TestGetProject -count=1` | ❌ W0 | ⬜ pending |
| 20-01-04 | 01 | 1 | PROJ-04 | integration | `go test ./internal/server/... -run TestProjectAdminOps -count=1` | ❌ W0 | ⬜ pending |
| 20-01-05 | 01 | 1 | PROJ-05 | integration | `go test ./internal/server/... -run TestMembership -count=1` | ❌ W0 | ⬜ pending |
| 20-01-06 | 01 | 1 | PROJ-06 | integration | `go test ./internal/server/... -run TestUpdateMemberRole -count=1` | ❌ W0 | ⬜ pending |
| 20-01-07 | 01 | 1 | PROJ-07 | integration | `go test ./internal/server/... -run TestProjectRun -count=1` | ❌ W0 | ⬜ pending |
| 20-01-08 | 01 | 1 | PROJ-07 | integration | `go test ./internal/server/... -run TestAuthDisabledBackcompat -count=1` | ❌ W0 | ⬜ pending |
| 20-01-09 | 01 | 1 | PROJ-01-06 | unit | `go test ./internal/store/sqlite/... -run TestProject -count=1` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/server/handlers_projects_test.go` — covers PROJ-01 through PROJ-07 HTTP layer
- [ ] `internal/store/sqlite/projects_test.go` — covers store-layer project CRUD
- [ ] `internal/store/sqlite/members_test.go` — covers store-layer membership CRUD + last-admin protection

*Existing test infrastructure — `sqlite.OpenMemory()`, `newAuthTestServer()`, `doAuthRequest()` — is reusable for all new tests.*

---

## Manual-Only Verifications

*All phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
