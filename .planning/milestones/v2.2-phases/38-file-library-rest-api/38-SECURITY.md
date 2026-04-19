---
phase: 38
slug: file-library-rest-api
status: verified
threats_open: 0
asvs_level: 1
created: 2026-04-19
---

# Phase 38 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| client -> API | Untrusted HTTP requests cross into the project-scoped managed file endpoints | JWT bearer token, project slug, file IDs |
| client -> handleImportFile | Untrusted multipart file content and JSON URIs reach the import handlers | Uploaded bytes, browser-supplied filenames, `s3://` URIs, local filesystem paths |
| handler -> store | The handler layer loads and mutates managed file records using user-selected IDs and project scope | Managed file IDs, project IDs, file metadata |
| handler -> backend.Stat | Import handlers validate user-supplied S3 keys and local paths against the storage backend | S3 object keys, absolute local filesystem paths |
| handler -> backend.UploadFileStream | Browser-uploaded file content is persisted to the configured storage backend | Arbitrary user file content and sanitized filenames |

---

## Threat Register

| Threat ID | Category | Component | Disposition | Mitigation | Status |
|-----------|----------|-----------|-------------|------------|--------|
| T-38-01 | Elevation of Privilege | `handleListFiles`, `handleDeleteFile` | mitigate | Route-level RBAC in [internal/server/server.go](internal/server/server.go) lines 188-190 enforces viewer access for `GET /files` and executor access for `POST/DELETE`; auth coverage is exercised by [internal/server/handlers_files_test.go](internal/server/handlers_files_test.go) lines 188-221 and 262-275. | closed |
| T-38-02 | Elevation of Privilege | `handleDeleteFile` | mitigate | Cross-project delete is rejected by the ownership check in [internal/server/handlers_files.go](internal/server/handlers_files.go) lines 64-68, returning `404` for foreign IDs; verified by [internal/server/handlers_files_test.go](internal/server/handlers_files_test.go) lines 244-260. | closed |
| T-38-03 (plan 01) | Information Disclosure | `handleListFiles` | accept | Listing is constrained to the current project via `projectContext` plus `ListManagedFiles(..., proj.ID)` in [internal/server/handlers_files.go](internal/server/handlers_files.go) lines 33-53. Residual disclosure is limited to authorized project viewers and is documented in the accepted risks log. | closed |
| T-38-03 (plan 02) | Tampering | `handleImportFileBrowser` | mitigate | Upload filenames are sanitized with `filepath.Base(...)`, storage paths are anchored under `BaseDir`, and prefix-checked before persistence in [internal/server/handlers_files.go](internal/server/handlers_files.go) lines 95-117; traversal sanitization is covered by [internal/server/handlers_files_test.go](internal/server/handlers_files_test.go) lines 315-332. | closed |
| T-38-04 | Tampering / Elevation of Privilege | `handleImportLocalPath` | mitigate | Local imports require an absolute `ImportRoot`, convert the target to an absolute path, and reject any path outside the root with `isWithinRoot(...)` in [internal/server/handlers_files.go](internal/server/handlers_files.go) lines 183-220 and 252-260; outside-root rejection is covered by [internal/server/handlers_files_test.go](internal/server/handlers_files_test.go) lines 419-434. | closed |
| T-38-05 | Denial of Service | `handleImportFileBrowser` | mitigate | Browser uploads are capped with `http.MaxBytesReader(w, r.Body, 100<<20)` before multipart parsing in [internal/server/handlers_files.go](internal/server/handlers_files.go) lines 95-100. | closed |
| T-38-06 | Information Disclosure | `handleImportS3URI` | accept | S3 imports are executor-only and bucket mismatches are rejected before `Stat`; the remaining error-surface risk on inaccessible objects is limited and documented as an accepted risk. Evidence: [internal/server/server.go](internal/server/server.go) lines 188-190 and [internal/server/handlers_files.go](internal/server/handlers_files.go) lines 157-171. | closed |
| T-38-07 | Spoofing | `handleImportLocalPath` | mitigate | Local path imports are disabled when `ImportRoot` is unset and otherwise constrained to that root in [internal/server/handlers_files.go](internal/server/handlers_files.go) lines 183-201; the disabled-path case is covered by [internal/server/handlers_files_test.go](internal/server/handlers_files_test.go) lines 436-450. | closed |

*Status: open · closed*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-38-01 | T-38-03 (plan 01) | The list endpoint intentionally exposes managed-file metadata to authenticated project viewers. Project scoping is enforced, and this visibility is required product behavior for the file library. | Codex security audit | 2026-04-19 |
| AR-38-02 | T-38-06 | Even with generic request handling, inaccessible S3 imports can still reveal that an object is unusable to an executor. This is acceptable because the endpoint is executor-only and does not expose data beyond the caller's configured bucket scope. | Codex security audit | 2026-04-19 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-04-19 | 8 | 8 | 0 | Codex |

Note: re-running the phase's `go test` commands in this sandbox timed out on 2026-04-19. Threat status is based on direct code inspection, the phase summaries' recorded green test runs, [38-UAT.md](.planning/phases/38-file-library-rest-api/38-UAT.md), and the handler test coverage in [internal/server/handlers_files_test.go](internal/server/handlers_files_test.go).

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-04-19
