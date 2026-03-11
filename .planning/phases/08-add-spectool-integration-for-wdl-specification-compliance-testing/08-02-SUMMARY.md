---
phase: 08-add-spectool-integration-for-wdl-specification-compliance-testing
plan: "02"
subsystem: testing
tags: [spectool, makefile, wdl-conformance, cargo, gitignore]

requires:
  - phase: 08-01
    provides: [spectool-compatible-stdout-json, absolute-path-outputs, output-flag]
provides:
  - Makefile with build/test/spectool targets
  - spectool-exclude.txt with pattern-based exclusion list
  - .gitignore for flowy binary and /tmp directories
affects: [ci, testing]

tech-stack:
  added: [spectool (via cargo install), GNU Make]
  patterns: [makefile-spectool-target, exclude-file-pattern]

key-files:
  created:
    - Makefile
    - spectool-exclude.txt
    - .gitignore
  modified: []

key-decisions:
  - "spectool has no --exclude-file flag; use --exclude with comma-separated patterns loaded from spectool-exclude.txt via shell"
  - "spectool separates target handling via --workflow-target-args and --task-target-args (not ~{target} in base command)"
  - "--redirect-stdout appends '> ~{output}' to the command — flowy stdout JSON is captured as outputs file"
  - "spectool git2 dependency requires vendored-openssl on systems without libssl-dev; sed patch applied in Makefile install step"
  - "spectool-exclude.txt patterns are comments-only initially; populate after first spectool run by observing failures"

patterns-established:
  - "Pattern 1: Makefile auto-strips comments from spectool-exclude.txt via grep/paste shell command"
  - "Pattern 2: Cargo install with vendored-openssl to avoid system SSL dependency"

requirements-completed:
  - SPEC-05
  - SPEC-06

duration: 5min
completed: "2026-03-11"
---

# Phase 8 Plan 02: Makefile and spectool Conformance Target Summary

**Makefile with `make spectool` target using verified spectool CLI flags: --workflow-target-args/--task-target-args, --redirect-stdout, comma-separated --exclude loaded from spectool-exclude.txt.**

## Performance

- **Duration:** 5 min
- **Started:** 2026-03-11T02:26:05Z
- **Completed:** 2026-03-11T02:31:36Z
- **Tasks:** 1 of 2 (Task 2 is human-verify checkpoint)
- **Files modified:** 3

## Accomplishments

- Installed spectool and verified exact CLI flags (research had LOW confidence on exclude mechanism)
- Discovered spectool uses `--workflow-target-args` / `--task-target-args` instead of `~{target}` in the command template
- Created Makefile with correct flags: `--all-capabilities`, `--redirect-stdout`, shell-loaded `--exclude`
- Applied vendored-openssl workaround for git2 dependency on systems without libssl-dev
- Created spectool-exclude.txt with documented pattern format; patterns populated after first run

## Task Commits

Each task was committed atomically:

1. **Task 1: Create Makefile and spectool-exclude.txt** - `8e5a000` (chore)

**Plan metadata:** (docs commit follows)

## Files Created/Modified

- `/home/uehara/flowy/Makefile` - Build and spectool conformance test targets
- `/home/uehara/flowy/spectool-exclude.txt` - Exclusion pattern file (comments-only; populate after first spectool run)
- `/home/uehara/flowy/.gitignore` - Exclude flowy binary and /tmp clone directories

## Decisions Made

1. **No --exclude-file flag in spectool:** Research noted LOW confidence on this. After installing spectool and running `spectool test --help`, confirmed the flag is `--exclude` (comma-separated list, no file variant). Adapted the Makefile to use `$(shell grep ... | paste -sd ',' -)` to load patterns from spectool-exclude.txt.

2. **--workflow-target-args / --task-target-args:** spectool separates target arguments for workflows vs. tasks. The command template does not use `~{target}` directly. Workflow runs get `--workflow-target-args ""` (no extra args needed) and task runs get `--task-target-args "--task ~{target}"`.

3. **vendored-openssl workaround:** spectool's git2 dependency requires pkg-config + libssl-dev at build time. On this system those are unavailable without sudo. Applied a `sed` patch in the Makefile install step to add the `vendored-openssl` feature to git2, which bundles OpenSSL during compilation.

4. **spectool-exclude.txt patterns deferred:** The plan directed populating patterns after the first spectool run. The file ships with commented-out placeholder patterns (http_import, sub_workflow, object_type) for reference. Exact patterns depend on spectool test naming which is only observable by running spectool.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] vendored-openssl workaround for spectool build**
- **Found during:** Task 1 (installing spectool to verify CLI flags)
- **Issue:** `cargo install` failed with "pkg-config not found" because git2 crate requires system OpenSSL headers
- **Fix:** Patched Cargo.toml in the install step via `sed` to add `features = ["vendored-openssl"]` to git2 dependency; this bundles OpenSSL at compile time
- **Files modified:** Makefile (spectool install step includes the sed patch)
- **Verification:** spectool compiled and installed successfully (`spectool v0.1.8`)
- **Committed in:** 8e5a000

**2. [Rule 1 - Bug / Adaptation] Corrected spectool CLI flag usage based on actual help output**
- **Found during:** Task 1 (running `spectool test --help`)
- **Issue:** Research notes had LOW confidence on `--exclude-file`; actual flag is `--exclude` with comma-separated patterns. Also discovered `--workflow-target-args` / `--task-target-args` replace the `~{target}` pattern in the base command template.
- **Fix:** Makefile uses shell expansion to load spectool-exclude.txt into `--exclude "..."` and uses the correct target-args flags
- **Files modified:** Makefile
- **Verification:** `make build` succeeds; Makefile syntax verified
- **Committed in:** 8e5a000

---

**Total deviations:** 2 auto-fixed (1 blocking, 1 adaptation)
**Impact on plan:** Both necessary to produce a working Makefile. No scope creep.

## Issues Encountered

- spectool build required vendored OpenSSL on this system — handled as blocking auto-fix (Rule 3)
- spectool CLI flags differed from research notes — adapted based on actual `--help` output

## User Setup Required

Task 2 is a `checkpoint:human-verify`. The user must run `make spectool` to:
1. Confirm spectool runs end-to-end (spectool is already installed)
2. Observe which tests pass/fail
3. Populate spectool-exclude.txt with actual test name patterns for unsupported features

## Next Phase Readiness

- `make build` is verified working
- spectool is installed at `~/.cargo/bin/spectool`
- WDL 1.2 spec will be cloned to `/tmp/wdl-spec` on first `make spectool` run
- spectool-exclude.txt needs to be populated with actual test patterns after human verify

---
*Phase: 08-add-spectool-integration-for-wdl-specification-compliance-testing*
*Completed: 2026-03-11*
