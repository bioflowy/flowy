---
phase: 25
slug: infrastructure-foundation
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-20
---

# Phase 25 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go test (`go test ./...`) — existing; no Jest/Vitest for Phase 25 |
| **Config file** | none (existing `go test` infrastructure) |
| **Quick run command** | `go test ./cmd/flowy-ui/... ./internal/config/... ./internal/server/...` |
| **Full suite command** | `go test ./... -count=1` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./cmd/flowy-ui/... ./internal/config/... ./internal/server/...`
- **After every plan wave:** Run `go test ./... -count=1`
- **Before `/gsd:verify-work`:** Full suite must be green + `make build-ui` smoke test passes
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 25-01-01 | 01 | 1 | INFRA-01 | smoke | `go build ./cmd/flowy-ui && ./flowy-ui & curl -s http://localhost:3000/projects \| head -1` | ❌ W0 | ⬜ pending |
| 25-01-02 | 01 | 1 | INFRA-01 | manual | manual browser test (F5 on /projects returns React app) | N/A | ⬜ pending |
| 25-02-01 | 02 | 1 | INFRA-02 | smoke | `curl -s http://localhost:3000/api/health` | ❌ W0 | ⬜ pending |
| 25-03-01 | 03 | 1 | INFRA-03 | smoke | `cd web && pnpm dev --port 5173` | ❌ W0 | ⬜ pending |
| 25-03-02 | 03 | 1 | INFRA-03 | smoke | `cd web && pnpm build` | ❌ W0 | ⬜ pending |
| 25-04-01 | 04 | 1 | INFRA-04 | smoke | `curl -s -X OPTIONS -H "Origin: http://localhost:5173" http://localhost:8124/auth/login -v 2>&1 \| grep Access-Control` | ❌ W0 | ⬜ pending |
| 25-04-02 | 04 | 1 | INFRA-04 | unit | `go test ./internal/server/...` | ❌ W0 | ⬜ pending |
| 25-05-01 | 05 | 1 | INFRA-05 | smoke | `make build-ui` | ❌ W0 | ⬜ pending |
| 25-05-02 | 05 | 1 | INFRA-05 | smoke | `./flowy-ui & sleep 1 && curl -s http://localhost:3000/ \| grep -c "<!doctype html"` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `cmd/flowy-ui/main.go` — does not exist yet; must be created as foundation
- [ ] `web/` directory — does not exist yet; Vite scaffold must be created
- [ ] `go-chi/cors` in `go.mod` — must be added with `go get github.com/go-chi/cors`
- [ ] `internal/config/config.go` — CORSConfig struct addition required
- [ ] `internal/server/server.go` — CORS middleware registration required

*(No new test files needed — existing `go test ./...` infrastructure covers all new Go server changes once code exists.)*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| F5 on `/projects` returns React app | INFRA-01 | Requires real browser rendering check | 1. Start `flowy-ui`, 2. Navigate to `/projects`, 3. Press F5, 4. Verify React app renders |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
