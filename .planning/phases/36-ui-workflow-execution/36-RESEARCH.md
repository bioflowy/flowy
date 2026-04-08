# Phase 36: UI Workflow Execution - Research

**Researched:** 2026-04-07
**Domain:** React / shadcn/ui — dynamic form generation, dialog composition, API integration
**Confidence:** HIGH

## Summary

Phase 36 adds a "Run" button to RegistryEntryPage that opens a dialog generating a dynamic form from the workflow input schema, submitting the run, and navigating to RunDetailPage. All infrastructure is in place: the inputs endpoint (`GET /projects/{slug}/registry/{name}/inputs?tag=`) is implemented and tested in Go, the submit endpoint (`POST /projects/{slug}/workflows`) is implemented with executor role gating, and the existing frontend patterns (CreateTagDialog, PushWDLDialog, usePollingRuns) provide direct templates.

The primary novelty is building a dynamically-typed form whose zod schema is computed at runtime from the API response — the schema shape changes when the user picks a different version tag. This is the central technical challenge. Everything else follows directly from existing patterns.

The UI specification is fully pre-defined in `36-UI-SPEC.md`: exact copywriting, field types per WDL type, interaction contracts, loading/error states, and color usage are locked. The Checkbox shadcn component must be installed before implementing Boolean fields (not yet present in `web/src/components/ui/`).

**Primary recommendation:** Implement in two tasks — (1) add the Checkbox component + create `RunWorkflowDialog` with full form logic, (2) wire the "Run" button into `RegistryEntryPage` and add tests. A single `useRunWorkflow` mutation hook in `useRuns.ts` is the cleanest approach for the submission.

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Run Dialog UI Design**
- "Run" button placement: next to entry name in RegistryEntryPage header, visible only for executor+ role — uses same role-check pattern as admin features (members list + useAuthStore)
- Dialog follows existing CreateTagDialog pattern: shadcn Dialog + react-hook-form + zod schema — consistent with PushWDLDialog and CreateTagDialog
- Version/tag selection via dropdown showing available tags (default "latest"), with selected version's digest displayed below — user picks which version to execute

**Input Form Field Types**
- Per-type controls using shadcn/ui components: text Input for String, number Input for Int/Float, Checkbox for Boolean, path-entry text Input for File, JSON Textarea for Array/Map/Struct
- Client-side validation via zod: required inputs (no default value in schema) are mandatory fields. Optional inputs left empty are omitted from submission JSON
- Pre-fill form fields with default values from the inputs endpoint response when available

**Post-Submission Flow**
- After successful submit: toast notification "Run submitted" via sonner, then navigate to RunDetailPage for the new run — Phase 35 polling shows live status automatically
- Server errors mapped to user-friendly messages in dialog: 400 → "Invalid inputs", 404 → "Workflow not found", 500 → "Server error" — same pattern as PushWDLDialog
- File-typed inputs: text Input with placeholder "e.g., s3://bucket/path or /absolute/path" and tooltip icon explaining server-accessible paths required, not browser-local files (per SC-4)

### Claude's Discretion
- Hook structure: single `useRunWorkflow` mutation hook or inline mutation — whichever is cleaner
- How to fetch and cache the input schema from GET /projects/{slug}/registry/{name}/inputs endpoint
- Whether to extract RunDetailPage output viewer into a shared component or import directly

### Deferred Ideas (OUT OF SCOPE)
- Dynamic form for compound types (Array, Map, Struct) with dedicated UI widgets — v2.2+ per REQUIREMENTS.md. Phase 36 uses JSON textarea for these types.
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| EXEC-01 | User can select a workflow from registry to execute (dialog with entry/version selection) | RegistryEntryPage + useRegistryVersions hook already available; tag-based Select in CreateTagDialog provides exact pattern |
| EXEC-02 | User can input workflow parameters via dynamically generated form (String, Int, Float, Boolean, File with specialized controls; Array/Map/Struct via JSON textarea) | Inputs endpoint returns `RegistryInputItem[]` with Name/Type/Optional/Default; zod schema built dynamically from this list at render time |
| EXEC-03 | User can submit workflow execution from the UI and see confirmation | POST /projects/{slug}/workflows with `{registry_ref, inputs}`; 201 response returns `{run_id}`; navigate to RunDetailPage |
| EXEC-05 | User can view execution results and output files after completion (reuses Phase 30 output viewer) | RunDetailPage already implements outputs table + preview/download; Phase 35 polling is already wired into RunDetailPage — no new code needed in RunDetailPage itself |
</phase_requirements>

---

## Standard Stack

### Core (already installed)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| react-hook-form | ^7.72.1 | Form state management | Project-established; used in all dialogs |
| zod | ^4.3.6 | Schema validation | Project-established; all forms use zodResolver |
| @hookform/resolvers | ^5.2.2 | Connects zod to react-hook-form | Project-established |
| @tanstack/react-query | ^5.64.2 | Data fetching / mutations | Project-established; all API calls use this |
| sonner | ^1.7.4 | Toast notifications | Project-established; toast.success/error used in all dialogs |
| lucide-react | ^0.475.0 | Icon set | Project-established; Info icon for File tooltip |

[VERIFIED: /home/uehara/flowy/web/package.json]

### New Component Required

| Component | Install Command | Purpose |
|-----------|----------------|---------|
| shadcn Checkbox | `npx shadcn@latest add checkbox` | Boolean input fields in RunWorkflowDialog |

**Not yet present** in `web/src/components/ui/`. Must be installed in Wave 0.
[VERIFIED: /home/uehara/flowy/web/src/components/ui/ directory listing — no checkbox.tsx]

### Existing shadcn/ui Components Available

All already installed and used in existing dialogs: Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter, Form, FormField, FormItem, FormLabel, FormControl, FormMessage, Input, Textarea, Select, SelectTrigger, SelectContent, SelectItem, SelectValue, Button, Badge, Separator, Tooltip, TooltipProvider, TooltipTrigger, TooltipContent.
[VERIFIED: /home/uehara/flowy/web/src/components/ui/ listing]

---

## Architecture Patterns

### Project Structure for This Phase

```
web/src/features/registry/
├── RunWorkflowDialog.tsx        # NEW — the dynamic input form dialog
├── useRegistry.ts               # EXTEND — add useRegistryInputs + useRunWorkflow
├── RegistryEntryPage.tsx        # MODIFY — add "Run" button + RunWorkflowDialog render
└── __tests__/
    └── RunWorkflowDialog.test.tsx  # NEW
```

No new directories. All new code lives within existing features.

### Pattern 1: Dialog Composition (from CreateTagDialog)

All dialogs follow this exact structure — RunWorkflowDialog must match it:

```typescript
// Source: web/src/features/registry/CreateTagDialog.tsx
export function RunWorkflowDialog({ open, onOpenChange, slug, name, versions }: Props) {
  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { ... },
  })

  function handleOpenChange(nextOpen: boolean) {
    if (!nextOpen) { form.reset() }
    onOpenChange(nextOpen)
  }

  async function onSubmit(data: FormValues) {
    try {
      await mutation.mutateAsync(...)
      toast.success('Run submitted.')
      form.reset()
      onOpenChange(false)
      navigate(`/projects/${slug}/runs/${runId}`)
    } catch (err) {
      if (err instanceof ApiError) {
        // map status → user-friendly message
      }
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-lg">
        ...
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
            ...
            <DialogFooter>
              <Button type="button" variant="ghost" onClick={() => handleOpenChange(false)}>Discard</Button>
              <Button type="submit" disabled={isPending}>
                {isPending ? <><Loader2 className="mr-2 h-4 w-4 animate-spin" />Submitting...</> : 'Run Workflow'}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
```

[VERIFIED: web/src/features/registry/CreateTagDialog.tsx]

### Pattern 2: Mutation Hook (from usePushWDL)

New `useRunWorkflow` mutation in `useRegistry.ts` (or `useRuns.ts`) follows this template:

```typescript
// Source: web/src/features/registry/useRegistry.ts (usePushWDL pattern)
export function useRunWorkflow(slug: string) {
  const queryClient = useQueryClient()
  return useMutation<SubmitResponse, ApiError, { registry_ref: string; inputs: Record<string, unknown> }>({
    mutationFn: ({ registry_ref, inputs }) =>
      apiClient.post(`/projects/${slug}/workflows`, { registry_ref, inputs }),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: ['projects', slug, 'runs'] }),
  })
}

interface SubmitResponse {
  run_id: string
}
```

[VERIFIED: web/src/lib/apiClient.ts — apiClient.post available; internal/api/types.go — SubmitResponse.RunID]

### Pattern 3: Role Check (executor gating)

The existing RegistryPage uses this exact pattern for canPush — use the same for "can run":

```typescript
// Source: web/src/features/registry/RegistryPage.tsx line 58
const canRun = isGlobalAdmin || currentMember?.role === 'admin' || currentMember?.role === 'executor'
```

`isGlobalAdmin` comes from `useAuthStore((s) => s.role) === 'admin'`. `currentMember` from `useProjectMembers(slug!)`.
[VERIFIED: web/src/features/registry/RegistryPage.tsx, web/src/store/authStore.ts]

### Pattern 4: API Error Mapping (from PushWDLDialog)

```typescript
// Source: web/src/features/registry/PushWDLDialog.tsx
catch (err) {
  if (err instanceof ApiError) {
    if (err.status === 400) toast.error('Invalid inputs. Check the values and try again.')
    else if (err.status === 404) toast.error('Workflow version not found.')
    else toast.error('Server error. Please try again.')
  } else {
    toast.error('Server error. Please try again.')
  }
}
```

Import `ApiError` from `@/lib/apiClient`.
[VERIFIED: web/src/features/registry/PushWDLDialog.tsx, web/src/lib/apiClient.ts]

### Pattern 5: Dynamic Zod Schema from API Response

The form schema must be rebuilt whenever the inputs list changes (version change). Use `useMemo` to derive schema and `useEffect` to reset form:

```typescript
const schema = useMemo(() => {
  const shape: Record<string, z.ZodTypeAny> = {}
  for (const input of inputs ?? []) {
    let field: z.ZodTypeAny
    switch (input.type) {
      case 'String': field = z.string().min(1, `${input.name} is required`); break
      case 'Int':    field = z.coerce.number().int(`${input.name} must be an integer`); break
      case 'Float':  field = z.coerce.number(`${input.name} must be a number`); break
      case 'Boolean': field = z.boolean(); break
      case 'File':   field = z.string().min(1, `${input.name} is required`); break
      default:       field = z.string().min(1).refine(isValidJSON, `${input.name} must be valid JSON`)
    }
    if (input.optional) field = field.optional()
    shape[input.name] = field
  }
  return z.object(shape)
}, [inputs])
```

**Critical:** `useForm` must receive the current schema as resolver; call `form.reset(defaultValues)` whenever inputs change. react-hook-form supports dynamic schemas via `resolver` prop that reads current schema at validation time.

[ASSUMED: The exact react-hook-form dynamic schema pattern — verified against project usage of zodResolver but the dynamic reset approach is based on training knowledge. Standard approach is to key the form on inputs array identity or call form.reset when inputs change.]

### Pattern 6: Input Schema Fetch

```typescript
// New hook in useRegistry.ts
export function useRegistryInputs(slug: string, name: string, tag: string) {
  return useQuery<RegistryInputsResponse>({
    queryKey: ['projects', slug, 'registry', name, 'inputs', tag],
    queryFn: () => apiClient.get(`/projects/${slug}/registry/${name}/inputs?tag=${encodeURIComponent(tag)}`),
    enabled: !!slug && !!name && !!tag,
  })
}

export interface RegistryInputsResponse {
  inputs: RegistryInputItem[]
}

export interface RegistryInputItem {
  name: string
  type: string
  optional: boolean
  default: string | null
}
```

[VERIFIED: internal/api/types.go — RegistryInputsResponse.Inputs, RegistryInputItem fields]

### API Endpoint Contracts

**Inputs endpoint:**
- `GET /projects/{slug}/registry/{name}/inputs?tag={tag}`
- Returns `{ inputs: [{ name, type, optional, default }] }`
- `type` values: "String", "Int", "Float", "Boolean", "File", "Array", "Map", "Struct" (and optional variants like "String?")
- `tag` parameter is required (400 if missing)
- Accessible by viewer role
[VERIFIED: internal/server/handlers_registry.go:321-352, internal/api/types.go:324-335]

**Submit endpoint:**
- `POST /projects/{slug}/workflows`
- Body: `{ registry_ref: "name:tag", inputs: { key: value } }`
- Response 201: `{ run_id: "uuid" }`
- Requires executor role
- Navigate to `/projects/{slug}/runs/{run_id}` after success
[VERIFIED: internal/server/server.go:163, internal/server/handlers.go:159-280, internal/api/types.go:6-31]

### Anti-Patterns to Avoid

- **Building a new Checkbox from scratch:** Use `npx shadcn@latest add checkbox` — @radix-ui/react-checkbox gives correct keyboard/accessibility behavior
- **Static zod schema:** The schema must be `useMemo`-derived from the API response, not defined at module level
- **Submitting empty optional fields as `null`:** Omit them entirely from the inputs JSON (undefined or absent key)
- **Using `/projects/{slug}/runs/submit`** (the other route): Use `/projects/{slug}/workflows` — consistent with the Context.md decision and the UI spec interaction contract
- **Coercing Int/Float from string without `z.coerce`:** `<input type="number">` returns string from HTML form; zod coerce is needed

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Form field accessibility | Custom input wrappers | shadcn Form/FormField/FormLabel/FormMessage | ARIA labels, error association auto-wired |
| Boolean toggle | `<input type="checkbox">` directly | shadcn Checkbox + `@radix-ui/react-checkbox` | Proper checked/indeterminate states, keyboard nav |
| Toast notifications | Custom toast state | `toast` from sonner | Already used project-wide; consistent UX |
| Mutation state | Manual loading/error state | `useMutation` isPending/isError | Automatic; avoids double-submit bugs |
| JSON validation | Custom parser | `JSON.parse` inside zod `.refine()` | Safe and sufficient |

---

## Common Pitfalls

### Pitfall 1: Dynamic Form Schema and react-hook-form

**What goes wrong:** If `useForm` is called once at component mount with an empty schema, adding inputs to the schema later does not update form validation — previously-submitted runs silently bypass validation.

**Why it happens:** `useForm` captures the resolver reference on first render; if the `zodResolver(schema)` argument uses a stale `schema`, validations run against the old shape.

**How to avoid:** Pass the resolver as a function that reads the current schema, or re-mount the form (key the `<Form>` on the inputs array hash). Simplest pattern: `resolver: zodResolver(schema)` where `schema` is a `useMemo` — react-hook-form reads the resolver on each submit, not just once.

**Warning signs:** Optional fields validating as required, or required fields allowing empty submit.

### Pitfall 2: Tag Selection Drives Input Schema — Must Sync on Change

**What goes wrong:** User selects "latest" (3 inputs), form renders. User changes to "v2.0" (5 inputs). Old form values for fields not in v2.0 get submitted in inputs JSON.

**Why it happens:** react-hook-form retains field values across renders unless `form.reset()` is called.

**How to avoid:** `useEffect(() => { form.reset(buildDefaults(inputs)) }, [inputs])` — reset form whenever the inputs array identity changes (i.e., when the query returns new data for a new tag).

### Pitfall 3: `?tag=` is Required by the Inputs Endpoint

**What goes wrong:** Calling `/inputs` without `?tag=` returns 400 "must specify ?tag= query parameter".

**Why it happens:** The handler explicitly requires the tag parameter.

**How to avoid:** Set `enabled: !!tag` in the useRegistryInputs query. Only enable the query when a tag is selected. Default to "latest" if available in the versions list, else first tag.

[VERIFIED: internal/server/handlers_registry.go:332-336]

### Pitfall 4: Checkbox with react-hook-form Needs Controller pattern

**What goes wrong:** Using `{...field}` spread on a Checkbox component fails because Checkbox uses `checked`/`onCheckedChange` props, not `value`/`onChange`.

**Why it happens:** shadcn Checkbox is a Radix UI controlled component, not a native HTML checkbox.

**How to avoid:** Use `FormField` with render prop:
```typescript
<FormField
  control={form.control}
  name={input.name}
  render={({ field }) => (
    <FormItem className="flex flex-row items-start space-x-3 space-y-0">
      <FormControl>
        <Checkbox
          checked={field.value}
          onCheckedChange={field.onChange}
        />
      </FormControl>
      <FormLabel>{input.name}</FormLabel>
    </FormItem>
  )}
/>
```

[ASSUMED: Radix UI Checkbox API — standard pattern from Radix docs; consistent with how shadcn Checkbox is documented.]

### Pitfall 5: TypeScript build check after each change

**What goes wrong:** Unused imports, type mismatches between RegistryInputItem (frontend) and RegistryInputItem (Go API) accumulate silently until `make build-ui` fails.

**How to avoid:** Run `cd web && npx tsc --noEmit` after each file modified. This is a CLAUDE.md-mandated step for all frontend tasks.

[VERIFIED: CLAUDE.md — "include cd web && npx tsc --noEmit in every task's verify/automated block"]

### Pitfall 6: Optional Fields — Omit from Submission, Not Submit null

**What goes wrong:** Sending `{ "optionalField": "" }` to the server causes WDL engine to receive an empty string for an optional input, potentially failing the workflow.

**How to avoid:** In the submit handler, iterate inputs and only include a key if the value is non-empty (for string fields) or explicitly provided (for boolean/number fields). For boolean optional fields, include only if the user toggled the checkbox.

---

## Code Examples

### Fetching Inputs with Proper Tag Default

```typescript
// Select default tag: prefer "latest", else first tag in versions list
const defaultTag = useMemo(() => {
  if (!versions) return ''
  const latestVersion = versions.find((v) => v.tags.includes('latest'))
  if (latestVersion) return 'latest'
  const firstTagged = versions.find((v) => v.tags.length > 0)
  return firstTagged?.tags[0] ?? ''
}, [versions])

const [selectedTag, setSelectedTag] = useState(defaultTag)
useEffect(() => { setSelectedTag(defaultTag) }, [defaultTag])

const { data: inputsData, isLoading: inputsLoading, isError: inputsError } =
  useRegistryInputs(slug, name, selectedTag)
```

### Building Submission Inputs (omit empty optional)

```typescript
function buildInputs(data: Record<string, unknown>, items: RegistryInputItem[]): Record<string, unknown> {
  const result: Record<string, unknown> = {}
  for (const item of items) {
    const value = data[item.name]
    if (item.optional && (value === '' || value === undefined || value === null)) continue
    // For Array/Map/Struct stored as JSON string, parse before sending
    if (['Array', 'Map', 'Struct'].some(t => item.type.startsWith(t))) {
      try { result[item.name] = JSON.parse(value as string) } catch { result[item.name] = value }
    } else {
      result[item.name] = value
    }
  }
  return result
}
```

### Tooltip for File Inputs

```typescript
// Source: RunDetailPage.tsx uses TooltipProvider/Tooltip/TooltipTrigger/TooltipContent pattern
import { Info } from 'lucide-react'

<TooltipProvider>
  <Tooltip>
    <TooltipTrigger asChild>
      <Info className="h-4 w-4 text-muted-foreground inline-block ml-1 cursor-help" />
    </TooltipTrigger>
    <TooltipContent>
      Enter a server-accessible path. Browser file paths are not supported.
      Use s3://, /absolute/path, or another path the server can resolve.
    </TooltipContent>
  </Tooltip>
</TooltipProvider>
```

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | vitest 3.2.4 + @testing-library/react 16.3.2 |
| Config file | `web/vitest.config.ts` |
| Quick run command | `cd web && npx vitest run src/features/registry/__tests__/RunWorkflowDialog.test.tsx` |
| Full suite command | `cd web && npx vitest run` |

[VERIFIED: web/vitest.config.ts, web/package.json]

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| EXEC-01 | "Run" button visible for executor/admin, absent for viewer | unit | `cd web && npx vitest run src/features/registry/__tests__/RegistryEntryPage.test.tsx` | ✅ (extend existing) |
| EXEC-01 | Clicking "Run" opens RunWorkflowDialog | unit | same | ✅ (extend existing) |
| EXEC-01 | Version selector shows available tags, defaults to "latest" | unit | `cd web && npx vitest run src/features/registry/__tests__/RunWorkflowDialog.test.tsx` | ❌ Wave 0 |
| EXEC-02 | Form renders correct control type per WDL type | unit | same | ❌ Wave 0 |
| EXEC-02 | Required fields show validation errors when empty | unit | same | ❌ Wave 0 |
| EXEC-02 | Default values pre-populate fields | unit | same | ❌ Wave 0 |
| EXEC-02 | File field shows tooltip with correct text | unit | same | ❌ Wave 0 |
| EXEC-03 | Submit calls POST /projects/{slug}/workflows with correct body | unit | same | ❌ Wave 0 |
| EXEC-03 | Success: toast "Run submitted." and navigate to RunDetailPage | unit | same | ❌ Wave 0 |
| EXEC-03 | 400/404/500 errors show correct toast messages | unit | same | ❌ Wave 0 |
| EXEC-05 | RunDetailPage outputs section exists and renders (already tested) | unit | `cd web && npx vitest run src/features/runs/__tests__/RunDetailPage.test.tsx` | ✅ |

### Sampling Rate
- **Per task commit:** `cd web && npx tsc --noEmit && npx vitest run src/features/registry/__tests__/`
- **Per wave merge:** `cd web && npx vitest run`
- **Phase gate:** Full suite green + `make build-ui` green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `web/src/features/registry/__tests__/RunWorkflowDialog.test.tsx` — covers EXEC-01, EXEC-02, EXEC-03
- [ ] `npx shadcn@latest add checkbox` — install Checkbox component before implementing Boolean fields

---

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | n/a — Bearer token handled by apiClient |
| V3 Session Management | no | n/a — session managed by authStore |
| V4 Access Control | yes | Role check: `canRun = role === 'admin' || role === 'executor'` — server also enforces executor role on POST /projects/{slug}/workflows |
| V5 Input Validation | yes | zod schema client-side; server validates WDL inputs server-side |
| V6 Cryptography | no | n/a |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Role bypass (viewer clicks hidden Run button) | Elevation of privilege | Button is conditionally not rendered; server enforces executor role middleware — defense in depth |
| JSON injection in Array/Map/Struct textarea | Tampering | `JSON.parse` inside zod refine — malformed JSON rejected client-side; server treats inputs as opaque values |
| Path traversal via File input | Tampering | Server-side only — file input is a string; daemon resolves paths, not the browser |

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Node.js | npx shadcn add, vitest | ✓ | — | — |
| shadcn Checkbox | Boolean form fields | ✗ | — | Install via `npx shadcn@latest add checkbox` in Wave 0 |

[VERIFIED: web/src/components/ui/ listing — checkbox.tsx absent]

**Missing dependencies with no fallback:**
- shadcn Checkbox — required for Boolean input fields; must be installed before implementing RunWorkflowDialog

---

## Project Constraints (from CLAUDE.md)

- All documentation, comments, commit messages, and planning files must be in **English**
- After modifying frontend code (`web/` directory), always verify: `cd web && npx tsc --noEmit`
- For final verification of a frontend phase, use: `make build-ui`
- Every task's `<verify>` / `<automated>` block must include `cd web && npx tsc --noEmit`
- Technical debt workarounds must be documented in `docs/technical-debt/`

[VERIFIED: CLAUDE.md]

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | react-hook-form reads the resolver callback on each submit (not just mount), so a `useMemo`-derived schema works without re-mounting the form | Architecture Patterns / Pitfall 1 | Form validates against wrong schema; fix: add form key on inputs identity |
| A2 | shadcn Checkbox uses `checked`/`onCheckedChange` props (Radix UI Checkbox API) | Pitfall 4 / Code Examples | Checkbox not controlled correctly; fix: check shadcn Checkbox source after `npx shadcn add checkbox` |

---

## Open Questions

1. **Tag with no tags: what if the selected version has no tags?**
   - What we know: `useRegistryVersions` returns versions, each with a `tags: string[]` array. Some versions have empty tags arrays.
   - What's unclear: If all versions have empty tags, there's nothing to pass as `?tag=`. The inputs endpoint requires `?tag=`.
   - Recommendation: The "Run" button should use the tag from the version dropdown selection. If no tags exist at all, disable the "Run" button with a tooltip: "No tagged version available to run." The UI spec does not address this edge case — leave it for implementation discretion.

2. **WDL type string format for optional types**
   - What we know: The backend returns `type: "String"`, `"Int"`, `"Boolean?"` etc. from `TypeName()`.
   - What's unclear: Exact strings for optional compound types (e.g., `"Array[String]?"`, `"Map[String, Int]?"`).
   - Recommendation: Match on prefix — `type.startsWith('Array')`, `type.startsWith('Map')`, `type.startsWith('Struct')`. For optional detection, use `item.optional` field (not the `?` suffix in type string).

---

## Sources

### Primary (HIGH confidence)
- `/home/uehara/flowy/internal/api/types.go` — RegistryInputItem, RegistryInputsResponse, SubmitRequest, SubmitResponse shapes
- `/home/uehara/flowy/internal/server/handlers_registry.go:321-373` — inputs endpoint implementation
- `/home/uehara/flowy/internal/server/server.go:163` — submit endpoint route
- `/home/uehara/flowy/web/src/features/registry/CreateTagDialog.tsx` — dialog pattern template
- `/home/uehara/flowy/web/src/features/registry/PushWDLDialog.tsx` — error mapping pattern
- `/home/uehara/flowy/web/src/features/registry/RegistryPage.tsx` — executor role check pattern
- `/home/uehara/flowy/web/src/features/registry/useRegistry.ts` — hook patterns
- `/home/uehara/flowy/web/src/lib/apiClient.ts` — API client
- `/home/uehara/flowy/web/package.json` — installed dependencies confirmed
- `/home/uehara/flowy/.planning/phases/36-ui-workflow-execution/36-UI-SPEC.md` — UI specification

### Secondary (MEDIUM confidence)
- Go handler test `handlers_registry_inputs_test.go` — confirms API contract for inputs endpoint
- `internal/server/handlers_projects_test.go` — confirms `/runs/submit` route exists alongside `/workflows`

### Tertiary (LOW confidence)
- None

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all packages verified in package.json; shadcn absence of Checkbox verified by directory listing
- Architecture: HIGH — all patterns drawn directly from existing codebase files
- API contract: HIGH — verified from Go source and type definitions
- Pitfalls: MEDIUM-HIGH — most verified from source; A1/A2 tagged as ASSUMED

**Research date:** 2026-04-07
**Valid until:** 2026-05-07 (stable stack)
