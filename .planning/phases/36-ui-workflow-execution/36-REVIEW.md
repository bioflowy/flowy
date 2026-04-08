---
phase: 36-ui-workflow-execution
reviewed: 2026-04-07T00:00:00Z
depth: standard
files_reviewed: 7
files_reviewed_list:
  - web/src/components/ui/checkbox.tsx
  - web/src/features/registry/RegistryEntryPage.tsx
  - web/src/features/registry/RunWorkflowDialog.tsx
  - web/src/features/registry/__tests__/RegistryEntryPage.test.tsx
  - web/src/features/registry/__tests__/RunWorkflowDialog.test.tsx
  - web/src/features/registry/useRegistry.ts
  - web/src/test/setup.ts
findings:
  critical: 0
  warning: 4
  info: 4
  total: 8
status: issues_found
---

# Phase 36: Code Review Report

**Reviewed:** 2026-04-07T00:00:00Z
**Depth:** standard
**Files Reviewed:** 7
**Status:** issues_found

## Summary

This phase introduces `RunWorkflowDialog` — a dynamic form that fetches WDL input schemas and submits workflow executions — along with role-gated "Run" button integration in `RegistryEntryPage`. The overall structure is sound: the dynamic Zod schema construction, optional-field preprocessing for numeric types, and the `onSubmit` JSON-parsing for complex types are all well-implemented. The role gate logic, query key design, and error-toast mapping are correct.

Four warnings were found, all behavioural correctness issues (not crashes): one silent UX dead-end for tagless versions, one potential runtime throw from an unchecked property access, one type-safety escape hatch that bypasses compile-time checks, and one test that can pass vacuously due to a fragile DOM query with an unconditional guard. Four informational items cover a minor redundancy, a lint-disabled effect, and coding style improvements.

---

## Warnings

### WR-01: Tagless registry versions are silently unselectable in the Run dialog

**File:** `web/src/features/registry/RunWorkflowDialog.tsx:257-263`

**Issue:** The version `<SelectContent>` only renders items for tagged versions — it iterates `versions.map(v => v.tags.map(...))`. Versions with no tags (which are valid and visible in the `RegistryEntryPage` table) produce zero `SelectItem` elements and are completely invisible to the user. If all versions are tagless, `getDefaultTag` returns `''`, `useRegistryInputs` is disabled (its `enabled: !!tag` guard fires), and the user sees an empty selector with no inputs and no feedback about why.

**Fix:** Include tagless versions in the selector using the truncated digest as the display value and the full digest as the select value, matching how `RegistryEntryPage` presents them:

```tsx
{versions.map((v) => {
  if (v.tags.length > 0) {
    return v.tags.map((tag) => (
      <SelectItem key={tag} value={tag}>
        {tag}
      </SelectItem>
    ))
  }
  // Tagless version — use digest as both key and value
  const shortDigest = truncateDigest(v.digest)
  return (
    <SelectItem key={v.digest} value={v.digest}>
      {shortDigest}
    </SelectItem>
  )
})}
```

`getDefaultTag` and `getDigestForTag` would also need corresponding updates for digest-valued selections.

---

### WR-02: Unchecked property access on `inputsData.inputs` could throw at runtime

**File:** `web/src/features/registry/RunWorkflowDialog.tsx:291`

**Issue:** The expression `inputsData?.inputs.length === 0` uses optional chaining on `inputsData` but not on `inputs`. If the API ever returns `{ inputs: undefined }` (schema drift, API change, or a future conditional field), this throws `TypeError: Cannot read properties of undefined (reading 'length')`. The same pattern recurs on line 297 (`inputsData?.inputs && inputsData.inputs.length > 0`), which is safe because of the truthiness guard, but line 291 is not.

**Fix:**
```tsx
{!inputsLoading && !inputsError && inputsData?.inputs?.length === 0 && (
```

---

### WR-03: Form `handleSubmit` type cast silences a type error

**File:** `web/src/features/registry/RunWorkflowDialog.tsx:248`

**Issue:** `form.handleSubmit(onSubmit as Parameters<typeof form.handleSubmit>[0])` suppresses a TypeScript error that arises because `useForm` is called without a generic type parameter and `onSubmit` accepts `Record<string, unknown>`. This means the compiler does not verify that `onSubmit`'s parameter type matches the schema shape. If the schema is changed (e.g., a new field type added), the type mismatch would be invisible at compile time.

**Fix:** Provide an explicit type to `useForm` using `z.infer` so the resolver and submit handler types align:

```tsx
type FormValues = z.infer<typeof schema>

const form = useForm<FormValues>({
  resolver: zodResolver(schema),
  defaultValues: computedDefaults as FormValues,
})

async function onSubmit(data: FormValues) { ... }
```

Because `schema` is computed inside `useMemo`, you would need to define the type outside the component or use a stable base schema type. An intermediate approach that avoids the cast and keeps the dynamic schema is:

```tsx
const form = useForm<Record<string, unknown>>({
  resolver: zodResolver(schema),
  defaultValues: computedDefaults,
})

async function onSubmit(data: Record<string, unknown>) { ... }

// In JSX — no cast needed:
<form onSubmit={form.handleSubmit(onSubmit)}>
```

---

### WR-04: Tooltip test passes vacuously when Info icon is not found

**File:** `web/src/features/registry/__tests__/RunWorkflowDialog.test.tsx:185-194`

**Issue:** The test uses `document.querySelector('svg.lucide-info')` to find the tooltip trigger icon, then only hovers it `if (infoIcon)`. If the selector returns `null` (e.g., Lucide changes its class naming, or JSDOM doesn't render SVGs with that class), the hover is skipped entirely. The `waitFor` then checks for the tooltip content — but Radix UI renders tooltip content in the DOM even without hover in JSDOM (portals render immediately). The test therefore always passes regardless of whether the Info icon actually exists or the tooltip is actually triggered by interaction.

**Fix:** Use a `data-testid` on the `TooltipTrigger` element and assert it exists before hovering, making the test fail explicitly if the icon is absent:

```tsx
// In RunWorkflowDialog.tsx
<TooltipTrigger asChild>
  <Info
    data-testid="file-input-tooltip-trigger"
    className="ml-1 inline h-4 w-4 text-muted-foreground cursor-help"
  />
</TooltipTrigger>
```

```tsx
// In RunWorkflowDialog.test.tsx
const infoIcon = screen.getByTestId('file-input-tooltip-trigger')
await user.hover(infoIcon)
await waitFor(() => {
  expect(screen.getByText(/Enter a server-accessible path/i)).toBeInTheDocument()
})
```

---

## Info

### IN-01: Redundant project-admin check in `canRun`

**File:** `web/src/features/registry/RegistryEntryPage.tsx:59-61`

**Issue:** `isProjectAdmin` is computed as `currentMember?.role === 'admin'` on line 59. Then `canRun` on line 61 independently repeats `currentMember?.role === 'admin'` instead of reusing `isProjectAdmin`. The two checks are always in sync, but any future change to the admin condition (e.g., adding another admin role) would need to be applied in both places.

**Fix:**
```tsx
const canRun = isGlobalAdmin || isProjectAdmin || currentMember?.role === 'executor'
```

---

### IN-02: `useEffect` for form reset has lint-disabled dependency

**File:** `web/src/features/registry/RunWorkflowDialog.tsx:181-183`

**Issue:** `form` is excluded from the `useEffect` dependency array with `// eslint-disable-line react-hooks/exhaustive-deps`. React Hook Form guarantees `form` is stable across renders for the same `useForm` call, so this is safe in practice. However, suppressing the lint rule rather than explaining the stability assumption leaves the intent unclear and may mask real issues if the suppression is copy-pasted to another context.

**Fix:** Add a comment documenting the exclusion reason, or restructure using `form.reset` captured in a ref:

```tsx
// form is intentionally excluded: RHF guarantees stable form identity
// across renders for the same useForm call instance.
useEffect(() => {
  form.reset(computedDefaults)
}, [computedDefaults]) // eslint-disable-line react-hooks/exhaustive-deps
```

---

### IN-03: `truncateDigest` function is duplicated across two files

**File:** `web/src/features/registry/RegistryEntryPage.tsx:32-35`, `web/src/features/registry/RunWorkflowDialog.tsx:58-61`

**Issue:** The `truncateDigest` function is defined identically in both `RegistryEntryPage.tsx` and `RunWorkflowDialog.tsx`. Any change to the truncation logic must be applied in both places.

**Fix:** Extract to a shared utility, for example `web/src/features/registry/registryUtils.ts`:

```ts
export function truncateDigest(digest: string): string {
  if (digest.startsWith('sha256:')) return `sha256:${digest.slice(7, 23)}...`
  return digest
}
```

---

### IN-04: Boolean default parsing is brittle string comparison

**File:** `web/src/features/registry/RunWorkflowDialog.tsx:167`

**Issue:** `defaults[input.name] = input.default === 'true'` converts the `default` string to a boolean by exact string comparison with `'true'`. If the backend ever returns `'True'`, `'1'`, or `'yes'` for a truthy boolean default, the form will silently default to `false`. The same comparison pattern is not replicated in the schema validation layer, creating an asymmetry.

**Fix:** Use a case-insensitive comparison and document the expected API contract:

```ts
defaults[input.name] = input.default?.toLowerCase() === 'true'
```

Or validate on the schema side to ensure the contract is explicit.

---

_Reviewed: 2026-04-07T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
