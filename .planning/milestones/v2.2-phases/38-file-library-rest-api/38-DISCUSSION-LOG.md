# Phase 38: File Library REST API - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-09
**Phase:** 38-file-library-rest-api
**Areas discussed:** POST body format, ImportRoot, RBAC roles, File label

---

## POST Body Format

| Option | Description | Selected |
|--------|-------------|----------|
| Single `uri` field | `{"uri": "s3://..."}` or `{"uri": "/path/..."}` — server detects mode by `s3://` prefix | ✓ |
| Separate `s3_uri` / `path` fields | One field set, the other omitted | |
| `type` discriminator | `{"type": "s3", "uri": "..."}` or `{"type": "local", "path": "..."}` | |

**User's choice:** Single `uri` field (content-type sniffing for upload vs JSON)
**Notes:** Recommended option accepted.

---

## Browser Upload Handler

| Option | Description | Selected |
|--------|-------------|----------|
| Call backend directly | `backend.UploadFileStream` directly in the files handler | ✓ |
| Refactor /uploads into shared func | Extract upload logic into shared helper | |

**User's choice:** Call backend directly
**Notes:** Keeps handlers independent; no refactoring of existing code.

---

## ImportRoot

| Option | Description | Selected |
|--------|-------------|----------|
| New `import_root` config field | Explicit operator-controlled config field; if unset, local path import disabled | ✓ |
| Reuse cfg.BaseDir | No new config; BaseDir as path guard root | |
| No path guard | Trust operator / network-level security | |

**User's choice:** New `import_root` config field
**Notes:** Recommended option accepted.

---

## ImportRoot — When Unset

| Option | Description | Selected |
|--------|-------------|----------|
| HTTP 400 "local path import not configured" | Clear error; consistent with missing config behavior | ✓ |
| HTTP 501 Not Implemented | Signals feature is disabled | |
| Fall back to BaseDir | Silent fallback | |

**User's choice:** HTTP 400 "local path import not configured"

---

## RBAC Roles

| Option | Description | Selected |
|--------|-------------|----------|
| viewer=LIST, executor=POST+DELETE | Consistent with runs endpoint pattern | ✓ |
| viewer=LIST, executor=POST, admin=DELETE | Elevated delete protection | |
| executor for all | Only executors can see files tab | |

**User's choice:** viewer=LIST, executor=POST+DELETE
**Notes:** Recommended option accepted.

---

## File Label Derivation

| Option | Description | Selected |
|--------|-------------|----------|
| filepath.Base for all modes | Consistent rule across upload/S3/local | ✓ |
| Mode-specific logic | Explicit per-mode handling | |

**User's choice:** filepath.Base for all modes

---

## Label Override at Import

| Option | Description | Selected |
|--------|-------------|----------|
| No — derive automatically | Label always auto-derived | ✓ |
| Yes — optional `label` field | Client can override at import time | |

**User's choice:** No — derive automatically
**Notes:** Editing labels deferred to future milestone.

---

## Claude's Discretion

- Handler file naming convention (`handlers_files.go`)
- Whether optional `file_type` field is accepted in POST JSON body

## Deferred Ideas

None.
