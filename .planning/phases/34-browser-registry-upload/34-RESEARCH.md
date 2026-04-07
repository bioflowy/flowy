# Phase 34: Browser Registry Upload - Research

**Researched:** 2026-04-07
**Domain:** React frontend — file upload dialog with role-gated button on RegistryPage
**Confidence:** HIGH

## Summary

Phase 34 adds a single new component, `PushWDLDialog`, to the existing `RegistryPage`. The backend push endpoint (`POST /projects/{slug}/registry/`) already exists and is guarded by `projectRoleRequired("executor")` middleware, which means global admins bypass the membership check automatically. The frontend needs no API changes — it calls the existing endpoint by reading WDL file content with the FileReader API and posting a JSON body `{name, content}`.

All required UI primitives (Dialog, Form, Input, Button, toast) are already installed in the project. The dialog pattern is fully established in `CreateTagDialog.tsx` — `react-hook-form` + `zod` resolver, `Loader2` spinner during submission, `form.reset()` on close, and `toast.success` / `toast.error` for feedback. The only novel element in this phase is the `<input type="file">` field and the FileReader-based content extraction, plus the executor-role visibility check on the page-level trigger button.

The empty state paragraph in `RegistryPage` must also be updated to mention the "Push WDL button above."

**Primary recommendation:** Implement `PushWDLDialog` following `CreateTagDialog` exactly, wire a `usePushWDL` mutation into `useRegistry.ts`, add the executor role check to derive `canPush` from the members list + `useAuthStore`, then render the trigger button conditionally in `RegistryPage`.

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- File picker using `<input type="file" accept=".wdl">` — read file content via FileReader API, send as string to existing push API (`POST /projects/{slug}/registry/`)
- Dialog layout: entry name text input + file picker + submit button — follows existing CreateTagDialog pattern (shadcn/ui Dialog + react-hook-form + zod)
- Success feedback: toast notification showing entry name + digest + kind badge, then auto-refresh entry list via `useRegistryEntries` invalidation
- "Push WDL" button placement: top of RegistryPage next to existing controls, visible only for executor+ role
- Client-side `.wdl` extension check before upload — inline validation message in dialog (per success criteria SC-4)
- Server errors mapped to user-friendly messages: 422 → "Invalid WDL file: {message}", 409 → "Name already used for a different kind"
- No client-side file size limit — WDL files are text, typically small
- Use project members list + current username from `useAuthStore` to check executor+ role — same pattern as admin check in RegistryPage
- Global admin can always push (consistent with backend RBAC)

### Claude's Discretion
None — all implementation decisions are locked.

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope.
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| REG-UI-01 | User can upload a WDL file from browser to push to registry (FileReader + existing push API) | FileReader API verified; `POST /projects/{slug}/registry/` with `{name, content}` body confirmed from backend types and tests; role gating confirmed via `projectRoleRequired("executor")` in server.go:168 |
</phase_requirements>

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| react-hook-form | ^7.72.1 | Form state, validation, submit | Already installed; used in all dialogs |
| zod | ^4.3.6 | Schema + runtime validation | Already installed; used everywhere |
| @hookform/resolvers | ^5.2.2 | Bridge zod → react-hook-form | Already installed |
| sonner | ^1.7.4 | Toast notifications | Already installed; `toast.success` / `toast.error` in use |
| shadcn/ui (Dialog, Form, Input, Button) | installed | UI primitives | All components confirmed in `web/components.json` |
| lucide-react | ^0.475.0 | Icons (Loader2, Upload) | Already installed |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| @tanstack/react-query | ^5.64.2 | Mutation + cache invalidation | `useMutation` for push, `invalidateQueries` for list refresh |
| FileReader API (browser built-in) | — | Read local file as text | Called inside form `onSubmit` before POST |

### Alternatives Considered
None — all choices are locked decisions from CONTEXT.md.

**Installation:** No new packages required. All dependencies are already installed.

---

## Architecture Patterns

### Recommended Project Structure

No new directories needed. New files land in the existing registry feature folder:

```
web/src/features/registry/
├── RegistryPage.tsx          # Modified — add button + PushWDLDialog mount
├── PushWDLDialog.tsx         # NEW component
├── useRegistry.ts            # Modified — add usePushWDL mutation
├── CreateTagDialog.tsx       # Reference (not modified)
└── __tests__/
    ├── RegistryPage.test.tsx # Modified — new role-gate tests + button visibility
    └── PushWDLDialog.test.tsx # NEW — dialog unit tests
```

### Pattern 1: Dialog with react-hook-form + zod (from CreateTagDialog)

**What:** Controlled dialog with a zod schema, `useForm` + `zodResolver`, form fields rendered via `FormField`, async submission with loading state, `form.reset()` on close.
**When to use:** All mutation dialogs in this project — this is the established pattern.

```typescript
// Source: web/src/features/registry/CreateTagDialog.tsx
const schema = z.object({ ... })
type FormValues = z.infer<typeof schema>

const form = useForm<FormValues>({
  resolver: zodResolver(schema),
  defaultValues: { ... },
})

function handleOpenChange(nextOpen: boolean) {
  if (!nextOpen) form.reset()
  onOpenChange(nextOpen)
}

async function onSubmit(data: FormValues) {
  try {
    await mutation.mutateAsync(data)
    toast.success('...')
    onOpenChange(false)
  } catch {
    toast.error('Something went wrong. Please try again.')
  }
}
```

### Pattern 2: File field with FileReader inside onSubmit

**What:** `<input type="file" accept=".wdl">` rendered inside `<FormControl>`. The file object is stored via `form.setValue` or accessed via `ref`/`watch`. On submit, FileReader reads the text content asynchronously before the API call.
**When to use:** Whenever file content must be sent as a string, not multipart.

```typescript
// zod schema for the file field — refine validates extension
file: z.instanceof(FileList)
  .refine((fl) => fl.length > 0, 'Select a file')
  .refine(
    (fl) => fl[0]?.name.endsWith('.wdl'),
    'Only .wdl files are accepted. Please select a valid WDL file.'
  ),

// Inside onSubmit:
async function onSubmit(data: FormValues) {
  const file = data.file[0]
  const content = await new Promise<string>((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = (e) => resolve(e.target!.result as string)
    reader.onerror = () => reject(new Error('Failed to read file'))
    reader.readAsText(file)
  })
  // ... POST { name: data.name, content }
}
```

**Note:** `z.instanceof(FileList)` works in the browser. In tests, supply a mock FileList or use a custom transformer. Alternatively, use `z.custom<FileList>()` for wider compatibility.

### Pattern 3: Executor+ role check (new — extends existing admin pattern)

**What:** Derive `canPush` from the members list combined with the global role from `useAuthStore`. Global admin always passes. Project executor and admin pass. Viewer does not.

```typescript
// Source: mirrors web/src/features/registry/RegistryPage.tsx lines 50-54
const currentUsername = useAuthStore((s) => s.username)
const globalRole = useAuthStore((s) => s.role)

const currentMember = members?.find((m) => m.username === currentUsername)
const isGlobalAdmin = globalRole === 'admin'
const canPush =
  isGlobalAdmin ||
  currentMember?.role === 'admin' ||
  currentMember?.role === 'executor'
```

This matches the backend's `projectRoleRequired("executor")` which lets global admins bypass the membership check (confirmed in `middleware.go` lines 75-78).

### Pattern 4: useMutation with cache invalidation (from useRegistry.ts)

**What:** Add a new `usePushWDL` hook following the existing `useUpsertTag` / `useDeleteRegistryEntry` pattern.

```typescript
// Source: web/src/features/registry/useRegistry.ts (useDeleteRegistryEntry pattern)
export interface RegistryPushResponse {
  name: string
  digest: string
  kind: string
  version_id: string
}

export function usePushWDL(slug: string) {
  const queryClient = useQueryClient()
  return useMutation<RegistryPushResponse, Error, { name: string; content: string }>({
    mutationFn: ({ name, content }) =>
      apiClient.post(`/projects/${slug}/registry/`, { name, content }),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: ['projects', slug, 'registry'] }),
  })
}
```

**URL note:** The backend route is registered as `POST("/")` under the registry group (server.go:168). Verify whether `/projects/{slug}/registry/` (with trailing slash) or `/projects/{slug}/registry` (without) is correct. Existing `useRegistryEntries` uses the no-trailing-slash form for GET, so use the same: `/projects/${slug}/registry`.

### Anti-Patterns to Avoid

- **Reading file content outside onSubmit:** FileReader is async; reading it in a zod refine or onChange handler creates race conditions. Read in `onSubmit` only.
- **Using `e.target.files` directly in a zod refine:** `FileList` is not serializable. Store the FileList in form state, validate extension in zod refine, read content in submit handler.
- **Resetting form inside `onSuccess` of the mutation instead of after await:** Causes flicker if the dialog is still open. Reset after `onOpenChange(false)`.
- **Showing success toast before list refresh:** Invalidation is async; the toast appears immediately after invalidation is triggered. This is fine — it matches the existing pattern.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Form validation | Custom validation logic | `zod` + `zodResolver` | Already installed, handles nested errors and async |
| Loading state | Manual `isLoading` boolean | `form.formState.isSubmitting` | Already the pattern in CreateTagDialog |
| Toast notifications | Custom notification component | `sonner` `toast.success` / `toast.error` | Already installed; consistent with all other pages |
| Cache refresh after push | Manual re-fetch | `queryClient.invalidateQueries` | Existing pattern; React Query handles stale state |
| File extension validation UI | Custom error rendering | `<FormMessage />` + zod refine | FormMessage reads errors from form state automatically |

---

## Common Pitfalls

### Pitfall 1: FileList in zod schema breaks in test environment
**What goes wrong:** `z.instanceof(FileList)` fails in jsdom because `FileList` is not a real class.
**Why it happens:** jsdom's `FileList` is different from the browser's native class.
**How to avoid:** Use `z.custom<FileList>((val) => val instanceof FileList || (val && typeof val === 'object' && 'length' in val))` for test compatibility, or mock `FileList` in `vitest.setup.ts`. Alternatively, use `z.any()` for the file field and rely only on the `.refine()` checks.
**Warning signs:** Tests throw "Expected FileList but received Object" — indicates schema incompatibility.

### Pitfall 2: File input value cannot be reset via react-hook-form setValue
**What goes wrong:** `form.reset()` clears controlled fields but `<input type="file">` is uncontrolled in the DOM. The file picker still shows the previously selected file name after reset.
**Why it happens:** Browser security prevents programmatic value assignment to file inputs.
**How to avoid:** Keep a `ref` on the file input and call `inputRef.current.value = ''` inside `handleOpenChange` when the dialog closes. Or remount the dialog by key.
**Warning signs:** After closing and reopening the dialog, the file picker still shows the old file name.

### Pitfall 3: Missing trailing-slash in POST URL causes redirect/405
**What goes wrong:** If the backend route expects a trailing slash and `fetch` sends without it (or vice versa), the vite proxy may return a 301/308 redirect that drops the request body.
**Why it happens:** Chi router and vite dev proxy handle trailing slashes differently.
**How to avoid:** Match exactly the URL pattern used in the backend tests: `POST /projects/{slug}/registry` (no trailing slash, confirmed in `handlers_registry_test.go` line 112).

### Pitfall 4: Global admin not in the project members list
**What goes wrong:** `members?.find(m => m.username === currentUsername)` returns `undefined` for a global admin who has not been added as a project member. If only the membership check is used, the global admin sees no button.
**Why it happens:** Global admins bypass the membership requirement on the backend but may not appear in the members list.
**How to avoid:** Check `globalRole === 'admin'` from `useAuthStore` as an OR condition before the membership check. This is confirmed by the CONTEXT.md locked decision and the backend middleware at `middleware.go:76`.

### Pitfall 5: Submit button enabled before both fields are filled
**What goes wrong:** Button is enabled when only the name is filled but no file is selected (or vice versa).
**How to avoid:** The UI-SPEC requires submit disabled until name is non-empty AND file is selected. Use `form.formState.isValid` (requires `mode: 'onChange'` in `useForm`) or derive disabled state explicitly.

---

## Code Examples

### Zod schema for PushWDLDialog

```typescript
// Adapted from CreateTagDialog.tsx pattern + file input specifics
const pushWDLSchema = z.object({
  name: z
    .string()
    .min(1, 'Entry name is required')
    .max(64)
    .regex(/^[a-z0-9-]+$/, 'Only lowercase letters, numbers, and hyphens allowed'),
  file: z.any()
    .refine((fl) => fl instanceof FileList && fl.length > 0, 'Select a file')
    .refine(
      (fl) => fl instanceof FileList && fl.length > 0 && fl[0].name.endsWith('.wdl'),
      'Only .wdl files are accepted. Please select a valid WDL file.'
    ),
})
```

### FileReader as Promise inside onSubmit

```typescript
async function onSubmit(data: PushWDLFormValues) {
  const file: File = data.file[0]
  let content: string
  try {
    content = await new Promise<string>((resolve, reject) => {
      const reader = new FileReader()
      reader.onload = (e) => resolve(e.target!.result as string)
      reader.onerror = () => reject(new Error('Failed to read file'))
      reader.readAsText(file)
    })
  } catch {
    toast.error('Something went wrong. Please try again.')
    return
  }
  try {
    const result = await pushWDL.mutateAsync({ name: data.name, content })
    const shortDigest = result.digest.startsWith('sha256:')
      ? result.digest.slice(7, 23) + '...'
      : result.digest
    toast.success(`${result.name} pushed. Digest: ${shortDigest}`)
    form.reset()
    onOpenChange(false)
  } catch (err) {
    if (err instanceof ApiError) {
      if (err.status === 422) toast.error(`Invalid WDL: ${err.message}`)
      else if (err.status === 409) toast.error('Name already exists with a different kind. Choose a different entry name.')
      else toast.error('Something went wrong. Please try again.')
    } else {
      toast.error('Something went wrong. Please try again.')
    }
  }
}
```

### Executor role gate in RegistryPage

```typescript
const currentUsername = useAuthStore((s) => s.username)
const globalRole = useAuthStore((s) => s.role)
const currentMember = members?.find((m) => m.username === currentUsername)
const isGlobalAdmin = globalRole === 'admin'
const canPush =
  isGlobalAdmin ||
  currentMember?.role === 'admin' ||
  currentMember?.role === 'executor'

// In JSX:
{canPush && (
  <Button variant="default" onClick={() => setPushOpen(true)}>
    <Upload className="mr-2 h-4 w-4" />
    Push WDL
  </Button>
)}
```

### File input reset on dialog close

```typescript
const fileInputRef = useRef<HTMLInputElement>(null)

function handleOpenChange(nextOpen: boolean) {
  if (!nextOpen) {
    form.reset()
    if (fileInputRef.current) fileInputRef.current.value = ''
  }
  onOpenChange(nextOpen)
}

// In JSX (inside FormField render):
<FormControl>
  <Input
    type="file"
    accept=".wdl"
    ref={fileInputRef}
    onChange={(e) => field.onChange(e.target.files)}
  />
</FormControl>
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `<input type="file">` with `FormData` + multipart | FileReader + JSON string body | Decided in CONTEXT.md | No multipart setup needed; consistent with existing `apiClient.post` |
| Manual role checks per feature | `projectRoleRequired(minRole)` backend middleware + frontend derived check | Established by Phase 31 | Frontend only gates UI; backend enforces authz |

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Vitest 2.x + jsdom + @testing-library/react |
| Config file | `web/vitest.config.ts` |
| Quick run command | `cd web && npx vitest run src/features/registry` |
| Full suite command | `cd web && npx vitest run` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REG-UI-01 | "Push WDL" button visible for executor, hidden for viewer | unit | `cd web && npx vitest run src/features/registry/__tests__/RegistryPage.test.tsx` | ✅ (file exists, needs new tests) |
| REG-UI-01 | Dialog opens on button click, shows name + file fields | unit | `cd web && npx vitest run src/features/registry/__tests__/PushWDLDialog.test.tsx` | ❌ Wave 0 |
| REG-UI-01 | Non-.wdl file shows inline validation error (no network call) | unit | `cd web && npx vitest run src/features/registry/__tests__/PushWDLDialog.test.tsx` | ❌ Wave 0 |
| REG-UI-01 | Successful push shows toast + invalidates query | unit | `cd web && npx vitest run src/features/registry/__tests__/PushWDLDialog.test.tsx` | ❌ Wave 0 |
| REG-UI-01 | TypeScript build passes | build | `cd web && npx tsc --noEmit` | ✅ (command, not file) |

### Sampling Rate

- **Per task commit:** `cd web && npx tsc --noEmit`
- **Per wave merge:** `cd web && npx vitest run src/features/registry`
- **Phase gate:** `cd web && npx vitest run` (full suite green) before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `web/src/features/registry/__tests__/PushWDLDialog.test.tsx` — covers REG-UI-01 dialog behavior (SC-2, SC-3, SC-4)
- [ ] New `usePushWDL` mock in the mock block of `RegistryPage.test.tsx` — needed to test button visibility without real network

---

## Sources

### Primary (HIGH confidence)
- `web/src/features/registry/CreateTagDialog.tsx` — canonical dialog pattern with react-hook-form + zod + Loader2 + toast
- `web/src/features/registry/useRegistry.ts` — mutation and query hook patterns
- `web/src/lib/apiClient.ts` — `ApiError` class with `status` field; `apiClient.post` sends JSON body
- `internal/api/types.go:273-288` — `RegistryPushRequest {name, content}` and `RegistryPushResponse {name, digest, kind, version_id}` confirmed
- `internal/server/server.go:168` — `POST /` under registry group requires executor role
- `internal/server/middleware.go:62-100` — global admin bypass confirmed
- `web/src/features/registry/RegistryPage.tsx:50-54` — existing admin role check pattern
- `web/vitest.config.ts` — test framework config (Vitest + jsdom)
- `.planning/phases/34-browser-registry-upload/34-UI-SPEC.md` — copywriting, component inventory, interaction states

### Secondary (MEDIUM confidence)
- `internal/server/handlers_registry_test.go:112` — POST URL is `/projects/{slug}/registry` (no trailing slash)
- Browser FileReader API documentation (MDN) — well-known stable API, no version concerns

### Tertiary (LOW confidence)
- None

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all packages confirmed installed via `package.json`
- Architecture: HIGH — patterns copied directly from existing files in the codebase
- API contract: HIGH — types confirmed from `internal/api/types.go`
- Role gating: HIGH — middleware source read directly
- Pitfalls: HIGH (file input reset, FileList in jsdom) — well-known browser/jsdom limitations

**Research date:** 2026-04-07
**Valid until:** 2026-05-07 (stable codebase, no fast-moving dependencies being introduced)
