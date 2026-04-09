# Pitfalls Research

**Domain:** Managed file library added to existing WDL workflow engine (Go + SQLite + S3/MinIO + React)
**Researched:** 2026-04-09
**Confidence:** HIGH — based on direct codebase inspection plus domain analysis

---

## Critical Pitfalls

### Pitfall 1: Non-Atomic Output File Registration (Race Between State Write and File Insert)

**What goes wrong:**
The run goroutine in `dispatch.go` calls `UpdateRunOutputs` and then `UpdateRunState(COMPLETED)` as two separate, non-transactional writes. If auto-registration of output files is inserted between them — or after them — a crash, panic, or goroutine cancellation leaves the run COMPLETED in the DB with no managed file records. A client polling the run sees COMPLETED but the Files tab is empty. Re-running registration has no idempotent path.

**Why it happens:**
The current pattern in `runWorkflow` (dispatch.go lines 277–295) runs `UpdateRunOutputs`, writes `outputs.json` to storage, then calls `UpdateRunState(COMPLETED)`. Developers naturally append `RegisterOutputFiles(...)` at the end of the success branch. But that fourth step is outside any transaction and runs after the state is already COMPLETED. On partial failure the system is permanently inconsistent.

**How to avoid:**
Add a single atomic `CompleteRun(ctx, runID, outputsJSON string, files []ManagedFileInput) error` method to the `Store` interface. The `SQLiteStore` implementation wraps all three mutations — `UPDATE runs SET outputs_json`, `INSERT OR IGNORE INTO managed_files (one row per file)`, and `UPDATE runs SET state = 'COMPLETED'` — in a single `BEGIN TRANSACTION ... COMMIT` using the single-writer pool (`SQLiteStore.writer`).

Important constraint: the `Server` struct holds `store.Store` (the interface), not `*sqlite.SQLiteStore`. The concrete `WriterDB()` method on `SQLiteStore` is not accessible from the server. Calling individual Store methods in sequence does NOT provide atomicity. The only correct design is to express atomicity as a single new interface method.

Concretely:
```
BEGIN TRANSACTION
  UPDATE runs SET outputs_json = ?, updated_at = now() WHERE id = ?
  INSERT OR IGNORE INTO managed_files ... (one row per File/Directory output)
  UPDATE runs SET state = 'COMPLETED', updated_at = now() WHERE id = ?
COMMIT
```

**Warning signs:**
- Files tab is empty on completed runs during integration testing.
- Run state is COMPLETED but `managed_files` table has zero rows for that run.
- Test teardown introduces random sleep and intermittent failures appear.
- `runWorkflow` calls `store.UpdateRunOutputs`, then a separate file insert function, then `store.UpdateRunState` — three separate store method calls.

**Phase to address:**
Phase that implements `runWorkflow` completion hook — the `CompleteRun` atomic method must be added to the `Store` interface here, not retrofitted later.

---

### Pitfall 2: Path Traversal in Server Local-Path Import

**What goes wrong:**
The "import from server local path" feature accepts a user-supplied path (e.g. `/data/samples/reads.fastq`). If the server blindly calls `os.Open(userPath)` or `s.backend.DownloadFile(userPath)`, a malicious executor role user can supply `../../../../etc/passwd` or any absolute path the server process can read. The existing upload handler already validates against `BaseDir` prefix (`handlers_uploads.go` lines 47–50), but that protection does not carry over to a new import-from-path endpoint if it is written separately.

**Why it happens:**
Developers treat server local-path import as "just open a file on disk," copy the happy path from `DownloadFile`, and forget to reapply the prefix-confinement guard. The existing `handleFileDownload` uses a weaker `strings.Contains(filePath, "..")` check (line 665) that stops relative traversal but permits absolute paths outside allowed directories.

**How to avoid:**
Define an explicit "allowed import root" (config field, e.g. `ImportRoot`). On every local-path import request:
1. `filepath.Clean` the user path.
2. Assert it has `filepath.Clean(cfg.ImportRoot) + "/"` prefix.
3. Return 400 if the assertion fails — do not log the attempted path in error responses (information leak).

Reuse the same guard pattern already in `handleUpload`.

**Warning signs:**
- Handler uses `os.Open(r.URL.Query().Get("path"))` directly without prefix check.
- Integration test with `path=../../etc/passwd` returns 200 instead of 400.
- Absolute paths outside `BaseDir` are accepted.

**Phase to address:**
Phase implementing file import endpoint. Must be in the security review checklist for that phase.

---

### Pitfall 3: Template Injection in Label Expression Evaluation

**What goes wrong:**
Label templates like `${fastq%%.fastq*}.bam` must be evaluated as shell-style parameter expansions against workflow input values, not as Go `text/template` or `html/template` strings. If a developer reaches for `text/template` because it already does `${...}` substitution, a label value containing `{{range .}}...{{end}}` or `{{.Method}}` causes template execution to call arbitrary Go methods on the data object — a code injection vector. Even `fmt.Sprintf` with user-controlled format strings is dangerous.

**Why it happens:**
Shell `${var%%suffix}` syntax looks like a mini-template language. The obvious Go implementation is `text/template`. Developers who are not fluent in shell parameter expansion operators (`%`, `%%`, `#`, `##`) reach for a general template engine.

**How to avoid:**
Implement a dedicated, narrow shell-parameter-expansion evaluator that supports only the operators required by the spec: `${var}`, `${var%suffix}`, `${var%%suffix}`, `${var#prefix}`, `${var##prefix}`. This is ~50 lines of Go with a small lexer that reads `${...}` spans and dispatches on the operator character. The evaluator takes a `map[string]string` of input names to string-rendered values. It never calls `os/exec`, never interprets `$(...)`, and never dereferences method chains. Unit-test it with adversarial inputs like `${x}; rm -rf /`.

**Warning signs:**
- Label evaluation imports `"text/template"` or `"html/template"`.
- Evaluation path calls `os.Expand` with the user expression as a `${}` template (Go's `os.Expand` does not support `%%` operators so callers try to pre-process).
- Label value containing `{{` causes a panic or unexpected output.

**Phase to address:**
Phase implementing parameter_meta parsing and label template expansion. The evaluator must have its own unit test file before the feature is wired up.

---

### Pitfall 4: S3 File Size Requires HeadObject — Not Available on Current Backend Interface

**What goes wrong:**
The `managed_files` table needs a `size` column. For local-filesystem files, `os.Stat(path).Size()` is trivial. For S3-registered files, there is no `Stat` or `HeadObject` method on the current `Backend` interface (13 methods; none return metadata). A developer who adds a `managed_files` row with `size = 0` or skips size entirely ships a silent correctness bug: the Files tab shows 0 bytes for all S3-originated files.

**Why it happens:**
The `Backend` interface was designed for data movement (upload/download), not metadata retrieval. S3Backend internally uses `GetObject` (which returns `ContentLength` in the response header) but that is not surfaced. Adding `Stat` to the interface seems like scope creep during the feature phase, so it gets deferred and the size field is left empty.

**How to avoid:**
Add a `Stat(path string) (FileInfo, error)` method to the `Backend` interface in the same phase that introduces managed files. `FileInfo` is a small struct: `{Size int64, ModTime time.Time}`. `LocalBackend` implements it with `os.Stat`. `S3Backend` implements it with `HeadObject` (AWS SDK v2: `s3.HeadObjectInput{Bucket, Key}`). This is the correct time to extend the interface — before managed_files rows are written — because retrofitting requires a data migration to backfill sizes.

Note: `S3APIClient` interface currently only has `PutObject`, `GetObject`, and `ListObjectsV2`. `HeadObject` must also be added to the `S3APIClient` interface.

**Warning signs:**
- `managed_files` migration adds `size INTEGER NOT NULL DEFAULT 0` — the DEFAULT 0 is a smell that size will not be populated.
- S3 file registration code calls `UploadFile` but not `Stat`.
- All S3-sourced files show 0 bytes in UI.

**Phase to address:**
Phase introducing the `managed_files` table migration. The `Backend.Stat` method must be added in the same commit before the first `INSERT INTO managed_files` is written.

---

### Pitfall 5: Duplicate File Registration on Retry or Re-Registration

**What goes wrong:**
If the run goroutine is retried (e.g., server restart with PENDING run recovery) or a user manually triggers re-registration of outputs, `INSERT INTO managed_files` fails on a UNIQUE constraint (path + project_id) or silently creates duplicate rows. Depending on which happens, the Files tab shows duplicates or the registration fails silently and no files are shown.

**Why it happens:**
The managed file model has an obvious uniqueness invariant — one row per (project, path) — but the migration may forget to enforce it, or the INSERT uses plain `INSERT` instead of `INSERT OR IGNORE` / `INSERT OR REPLACE` / `ON CONFLICT DO UPDATE`.

**How to avoid:**
The migration must include `UNIQUE (project_id, path)`. Registration code must use `INSERT OR IGNORE INTO managed_files ...` semantics (idempotent: silently skip if same path already registered for this project). Registration triggered by run completion must be idempotent by design. Add a test that calls the registration function twice for the same run and asserts exactly one row exists.

**Warning signs:**
- Migration lacks UNIQUE constraint on `(project_id, path)`.
- Test with two identical registrations produces two rows.
- Server-restart recovery test shows doubled entries.

**Phase to address:**
Phase implementing `managed_files` migration and `CreateManagedFile` store method.

---

### Pitfall 6: WDL Files Without parameter_meta Treated as Errors Instead of Graceful Fallback

**What goes wrong:**
The existing `extractInputs` function in `handlers_registry.go` (lines 354–373) reads `doc.Workflows[0].Inputs` without touching `ParamMeta`. If the new code that extracts `type` and `label` hints from `ParamMeta` uses `paramMeta["reads"].(map[string]any)["type"].(string)` with unchecked type assertions on a WDL file where `parameter_meta` is absent or uses a different value structure (string hint instead of object hint), it panics. All 41,483 LOC of existing WDL runs break.

**Why it happens:**
`ParamMeta` is `map[string]any` (ast.go line 79). Existing conformance WDL files use `parameter_meta { infile: { help: "..." } }` — the value is a map. But the WDL spec also allows scalar string hints: `parameter_meta { reads: "FASTQ input" }`. A type assertion on a string value as `map[string]any` panics. The developer tests against their new-style WDL files only.

The real conformance test case `ex_paramter_meta_task.wdl` uses `parameter_meta { infile: { help: "..." }, lines_only: { help: "..." }, region: { help: "...", suggestions: [...] } }` — object hints with `help` and `suggestions` but no `type` or `label` fields. This must continue to work correctly.

**How to avoid:**
The `extractInputs` function must use safe type-switch extraction, never direct type assertions:
```go
func paramMetaString(paramMeta map[string]any, inputName, field string) string {
    if paramMeta == nil { return "" }
    entry, ok := paramMeta[inputName]
    if !ok { return "" }
    m, ok := entry.(map[string]any)
    if !ok { return "" }  // scalar hint — ignore gracefully
    s, _ := m[field].(string)
    return s
}
```
Any missing key or wrong type returns empty string, never panics. The feature degrades gracefully: inputs without hints use filename as label and no type filter. Add a conformance test asserting `ex_paramter_meta_task.wdl` (which has old-style hints with `help` but no `type`/`label`) still works correctly after changes.

**Warning signs:**
- Code contains `paramMeta["key"].(map[string]any)` without a comma-ok check.
- Test suite only uses newly-written WDL files with the new hint format.
- Running the inputs endpoint against the existing conformance WDL files produces a 500.

**Phase to address:**
Phase implementing `parameter_meta` parsing for type/label hints in the inputs introspection endpoint.

---

### Pitfall 7: Large outputs_json Walking — O(n) File Detection Without AST

**What goes wrong:**
Auto-registration of output files requires finding all `File` and `Directory` typed values in `outputs_json`. The naive implementation is to unmarshal the JSON blob, walk the full value tree recursively, and collect string values for keys where the WDL output declaration type is `File` or `Directory`. This requires re-parsing the WDL source at run completion time to get type information — which requires fetching the registry version content from the DB and parsing it — adding latency and complexity to the critical completion path.

**Why it happens:**
At the point `runWorkflow` calls the registration hook, the `*ast.Document` is still in memory (it was used to execute the workflow). Developers either do not realize this, or the document is discarded before the completion hook is called (e.g., if the hook is implemented as a separate function that only receives `runID` and `outputsJSON`).

**How to avoid:**
Pass the `*ast.Document` (or a pre-extracted `map[string]types.Type` of output names to WDL types) directly to the registration hook. The document is already in scope in `runWorkflow` at the point outputs are available. Extracting output types from the in-memory AST (e.g., `doc.Workflows[0].Outputs`) costs zero I/O and zero re-parsing. Do not re-fetch from the registry or re-parse WDL at completion time.

**Warning signs:**
- Registration hook signature is `func(ctx, runID, outputsJSON string)` with no document reference.
- Completion path calls `store.GetRegistryVersion` or `parser.Parse` inside `runWorkflow`.
- Run completion latency grows with WDL source size.

**Phase to address:**
Phase implementing output file auto-registration hook in `dispatch.go`.

---

### Pitfall 8: outputs_json Contains File Paths as Plain Strings — Type Cannot Be Inferred from JSON Alone

**What goes wrong:**
`ValueToInterface` in `wdl/values/convert.go` serializes both `FileValue` and `StringValue` as plain Go `string`. The `outputs_json` blob stored in the DB looks like `{"workflow.output_bam": "/path/to/file.bam", "workflow.sample_name": "NA12878"}` — both values are JSON strings. Without the WDL output declaration types, auto-registration cannot distinguish which strings are file paths and which are plain strings. If the implementation guesses by heuristic (e.g., "strings containing `/` are files"), it will register non-file outputs as managed files.

**Why it happens:**
The serialization format was designed for human readability and API responses, not for machine type-reconstruction. This is intentional and correct; the type information lives in the WDL AST, not in the JSON. `DirectoryValue` is also serialized as a plain string by `ValueToInterface` (convert.go line 25).

**How to avoid:**
Always correlate `outputs_json` values against `doc.Workflows[0].Outputs` (or `doc.Tasks[0].Outputs`) to determine which outputs are `File` or `Directory` typed before inserting into `managed_files`. Never use path heuristics. For `Array[File]` outputs, the JSON value is a `[]interface{}` where each element is a string; these elements must each be registered individually.

**Warning signs:**
- Registration logic does not import or reference the WDL AST package.
- Registration uses `strings.HasPrefix(v, "/")` or similar heuristic.
- `Array[File]` outputs register as a single entry (the JSON array representation) instead of N file entries.

**Phase to address:**
Phase implementing output file auto-registration hook.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Store file size as 0 for S3 files initially | Skips Backend interface extension | Requires data migration to backfill; shows wrong sizes forever until fixed | Never — add `Backend.Stat` in same phase |
| `INSERT INTO managed_files` without UNIQUE constraint | Simpler migration | Duplicate rows on retry; no idempotent recovery path | Never |
| Re-parse WDL at run completion to detect File outputs | Decouples registration from dispatch | Adds latency; re-fetch from DB; failure if registry version deleted | Never — pass AST reference |
| Skip `ImportRoot` config; use `BaseDir` as upload root only | No config change | Local-path import has no confinement; path traversal risk | Never for server local-path import |
| Use Go `text/template` for label expansion | Familiar API | Template injection; `{{.Method}}` calls arbitrary Go methods | Never |
| Label = filename fallback when `parameter_meta` absent | Graceful degradation; zero breaking changes | N/A — this is the correct default | Always |
| Add atomicity via three separate store method calls instead of one atomic method | No Store interface change | Three separate writes are not atomic; crash between any two leaves DB inconsistent | Never — add `CompleteRun` to Store interface |

---

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| S3Backend file size | Using `ContentLength` from `GetObjectOutput` (requires downloading the object) | Add `HeadObject` to `S3APIClient` interface; implement `Backend.Stat` via `HeadObject` |
| SQLite single-writer | Separate `ExecContext` calls for outputs + file insert + state update | Add `CompleteRun` method to `Store` interface; implement with `BEGIN TRANSACTION ... COMMIT` in SQLiteStore using the `writer` pool |
| Store interface vs. concrete type | Calling `SQLiteStore.WriterDB()` from the server to begin a transaction | The server holds `store.Store` (interface), not the concrete type; atomicity must be expressed as a Store interface method, not via raw DB access from the server layer |
| MinIO HeadObject | MinIO responds identically to AWS S3 for HeadObject; no special case | Verify with MinIO testcontainer integration test |
| `outputs_json` type detection | File values are plain strings; no `"__type": "File"` annotation | Must use WDL AST output declarations as the authoritative type source |
| Browser upload → managed file | `handleUpload` returns a server-local/S3-key path; managed file record must be created after upload, not just on run completion | Add optional `RegisterFile` call in upload handler or make it a separate explicit step |
| `Array[File]` in outputs_json | JSON array serialized as `[]interface{}`; each element is a string path | Walk array, register each element individually |

---

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Full `managed_files` table scan for project | Files tab slow to load | Add `INDEX ON managed_files(project_id)` in migration | ~10K files per project |
| No pagination on Files tab API | GET /projects/{slug}/files returns all rows; large JSON payload | Add `LIMIT`/`OFFSET` or cursor pagination from day one | ~1K files |
| Synchronous S3 `HeadObject` per file during scatter output registration | Large scatter (100 shards × 10 File outputs) = 1000 sequential HeadObject calls at run completion | Accept size from upload return value where available; batch or parallelize HeadObject calls | ~100 File outputs per run |
| Walking deeply nested JSON for Array[Array[File]] | Unbounded recursion | Pre-flatten outputs using AST type info before registration; no recursive JSON walking | Nested array types |
| Re-parsing WDL source at completion | Latency proportional to WDL size | Pass `*ast.Document` in-memory to registration hook | Every run completion |

---

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| No `ImportRoot` confinement on server local-path import | Executor-role user reads `/etc/passwd`, private keys, other project data | Require `ImportRoot` config; assert `filepath.Clean(path)` has `filepath.Clean(ImportRoot)+"/"` prefix; return 400 on failure |
| Label template expression evaluated with `os/exec` or shell | Arbitrary command execution on server | Implement pure Go shell-expansion evaluator; never spawn subprocess for label expansion |
| Returning attempted traversal path in 400 error response | Information leak: attacker learns which paths exist | Return generic "invalid path" message; log internally only |
| Managed file path stored without canonicalization | Two entries for `/data/./reads.fastq` and `/data/reads.fastq`; UNIQUE fails to deduplicate | `filepath.Clean` all paths before INSERT |
| S3 import accepts paths from any bucket | User imports file from a path in a different project or external bucket | Validate that imported S3 URI is within `cfg.Storage.Bucket` + `cfg.Storage.Prefix` |
| Presigned S3 URLs exposed in managed file API response | Credentials embedded in URLs leak in browser history/logs | This project uses server-proxied downloads (existing ADR); do not add presigned URLs |

---

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| File picker shows all managed files regardless of type | User scrolls past irrelevant files when workflow expects `fastq` | Filter by `type` from parameter_meta; include "All files" fallback when no type hint |
| No manual path fallback in file picker | Users with files not yet in library cannot run workflows | Always keep "Enter path manually" escape hatch (stated requirement) |
| Label defaults to full S3 key path | Keys like `runs/abc123/tasks/align_0/output.bam` are unreadable | Default label = `filepath.Base(path)` (basename); override with template if parameter_meta provides one |
| Import from S3 path accepts any bucket path | User accidentally imports files from external location | Validate S3 URI is within deployment bucket+prefix |
| No indication of how many files were registered after run completes | User does not know if file registration succeeded | Add `registered_files` count to run detail API response or show count in run outputs section |

---

## "Looks Done But Isn't" Checklist

- [ ] **Auto-registration atomicity:** Verify registration runs inside the same `CompleteRun` transaction as `UpdateRunState(COMPLETED)` — not after it. Three separate Store calls is not atomic.
- [ ] **S3 file size:** Verify that managed_files rows for S3-originated files have non-zero `size` values in integration tests with MinIO.
- [ ] **parameter_meta graceful fallback:** Verify the existing conformance WDL files (`ex_paramter_meta_task.wdl`, `person_struct_task.wdl`) still return correct inputs from `GET /registry/{name}/inputs` after the new parsing is added.
- [ ] **Path traversal:** Verify that `POST /projects/{slug}/files/import?path=../../etc/passwd` returns 400, not 200.
- [ ] **Label template safety:** Verify that a label template containing `{{.SomeMethod}}` or `;rm -rf /` evaluates to a safe literal string, not executed code.
- [ ] **Duplicate registration:** Verify that running registration twice for the same run yields exactly one `managed_files` row per output file.
- [ ] **Directory type:** Verify that `Directory`-typed WDL outputs are also registered (not just `File`); `ValueToInterface` returns a plain string for both.
- [ ] **Array[File] outputs:** Verify that `Array[File]` output values have each element registered individually, not the JSON array as a single entry.
- [ ] **File picker type filter:** Verify that when parameter_meta provides `type: "bam"`, the file picker shows only managed files with matching type annotation.
- [ ] **Backend.Stat interface:** Verify both `LocalBackend` and `S3Backend` implement the new `Stat` method; verify compile-time interface assertion passes.
- [ ] **Store.CompleteRun interface:** Verify the new method is defined on `store.Store` (not just `SQLiteStore`); verify the mock/stub used in server unit tests also implements it.

---

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Non-atomic registration (files missing for completed runs) | HIGH | Write a one-time admin endpoint or script that re-reads `outputs_json` for all COMPLETED runs, re-extracts File paths by correlating against registry WDL source, and inserts missing `managed_files` rows with `INSERT OR IGNORE` |
| Path traversal exploit discovered post-ship | HIGH | Audit server access logs; rotate any credentials readable by server process; patch handler with prefix check; redeploy |
| Duplicate file rows from missing UNIQUE constraint | MEDIUM | `DELETE FROM managed_files WHERE rowid NOT IN (SELECT MIN(rowid) FROM managed_files GROUP BY project_id, path)`; add `CREATE UNIQUE INDEX` in a new migration |
| Wrong file sizes (all zero) for S3 files | MEDIUM | Add `Backend.Stat` with `HeadObject`; write backfill script that HeadObjects each S3-originated managed file and runs `UPDATE managed_files SET size = ? WHERE id = ?` |
| Label template injection (text/template used) | HIGH | Remove `text/template` import; replace with narrow evaluator; audit any label values stored in DB that were generated by the unsafe evaluator |

---

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Non-atomic output file registration | Phase: auto-register workflow outputs on completion | Integration test: kill server mid-registration; restart; assert no partial state |
| Path traversal in local-path import | Phase: file import endpoint | Test: `path=../../etc/passwd` returns 400 |
| Template injection in label expressions | Phase: parameter_meta parsing + label template expansion | Test: adversarial template with `{{` and `;rm` produces safe literal string |
| S3 size missing (no Backend.Stat) | Phase: managed_files migration + Backend interface extension | Test: S3 import shows correct byte count matching MinIO object size |
| Duplicate file registration | Phase: managed_files migration + CreateManagedFile store method | Test: double-register same run; assert 1 row per output file |
| parameter_meta backwards compatibility | Phase: inputs introspection + parameter_meta hints | Test: `GET /inputs` against all existing conformance WDL files returns 200 with correct schema |
| O(n) output walking without AST | Phase: auto-register workflow outputs | Code review gate: registration hook must accept `*ast.Document` or equivalent type map |
| File/String ambiguity in outputs_json | Phase: auto-register workflow outputs | Test: workflow with mixed File + String outputs registers only File-typed values |
| Store interface atomicity gap | Phase: managed_files migration + auto-registration | Code review gate: no three-separate-Store-calls pattern; `CompleteRun` must exist on `store.Store` interface |

---

## Sources

- Direct codebase inspection: `/home/uehara/flowy/internal/server/dispatch.go` (runWorkflow completion sequence — UpdateRunOutputs then UpdateRunState as separate writes)
- Direct codebase inspection: `/home/uehara/flowy/internal/server/server.go` (Server.store field is `store.Store` interface, not concrete `*sqlite.SQLiteStore` — WriterDB() is inaccessible from server layer)
- Direct codebase inspection: `/home/uehara/flowy/internal/server/handlers_uploads.go` (existing prefix-confinement path traversal guard)
- Direct codebase inspection: `/home/uehara/flowy/internal/server/handlers.go` (handleFileDownload weak `..` check at line 665 — insufficient for absolute path traversal)
- Direct codebase inspection: `/home/uehara/flowy/internal/storage/backend.go` (Backend interface — 13 methods, no Stat)
- Direct codebase inspection: `/home/uehara/flowy/internal/storage/s3.go` (S3APIClient interface — PutObject/GetObject/ListObjectsV2 only; no HeadObject)
- Direct codebase inspection: `/home/uehara/flowy/internal/store/sqlite/sqlite.go` (SQLiteStore.WriterDB() returns writer pool; single-writer constraint; not accessible via store.Store interface)
- Direct codebase inspection: `/home/uehara/flowy/internal/store/store.go` (Store interface — no transaction method, no CompleteRun method)
- Direct codebase inspection: `/home/uehara/flowy/wdl/ast/ast.go` (ParamMeta map[string]any — both scalar and object values are valid per parser)
- Direct codebase inspection: `/home/uehara/flowy/wdl/parser/parser.go` (parseMetaValue — scalar string and map both accepted)
- Direct codebase inspection: `/home/uehara/flowy/wdl/values/convert.go` (ValueToInterface — FileValue and StringValue both serialize as Go string; DirectoryValue also serializes as string at line 25; no type tag in JSON output)
- Direct codebase inspection: `/home/uehara/flowy/conformance-tests/flowy-data/.../ex_paramter_meta_task.wdl` (existing parameter_meta with `help` and `suggestions` fields — no type/label; must not break)

---
*Pitfalls research for: Managed File Library feature (v2.2) on flowy WDL engine*
*Researched: 2026-04-09*
