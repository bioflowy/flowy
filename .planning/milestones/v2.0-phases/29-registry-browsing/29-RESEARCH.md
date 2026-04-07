# Phase 29: Registry Browsing - Research

**Researched:** 2026-04-07
**Domain:** React frontend — registry browsing UI (table, detail page, WDL source viewer, tag management)
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- Table layout on `/projects/{slug}/registry` — consistent with Phase 27 (Users) and Phase 28 (Members) table pattern
- Columns: Name, Kind (colored badge — blue for tool, purple for workflow), Versions (count), Tags (comma-separated text, dash when none), Created date
- Clicking a row navigates to `/projects/{slug}/registry/{name}` detail page
- Admin-only row actions: three-dot dropdown menu with "Delete" option + confirmation dialog (AlertDialog, destructive red button)
- No "push" or "create" action in the UI — registry entries are created via CLI push only
- Version history: table on the entry detail page showing versions with digest (truncated), tag badges, created date
- Default: show all versions sorted by newest first (API already returns newest first)
- WDL source viewer: read-only code block with syntax highlighting, copy button, and line numbers
- Displayed on the entry detail page when a version is selected (or latest by default)
- Tag management: admin-only actions for creating/moving/deleting tags
- Delete tag: confirmation dialog (destructive action pattern from Phase 27)
- Create/move tag: dialog with tag name input + version digest selector

### Claude's Discretion
- Loading/empty states for registry entry table and version list
- Exact badge colors for kind (tool/workflow)
- Entry detail page layout (version table + source viewer arrangement)
- Syntax highlighting library choice for WDL source
- Tag badge styling in version table
- Pagination or scroll strategy if entry/version count grows

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| REG-01 | User can view list of registry entries in a project | `GET /projects/{slug}/registry` returns `RegistryEntryResponse[]`; table pattern from UsersPage replicates directly |
| REG-02 | User can view versions and tags for a registry entry | `GET /projects/{slug}/registry/{name}/versions` returns `RegistryVersionResponse[]`; version table on detail page |
| REG-03 | User can view WDL source code of a registry version | `GET /projects/{slug}/registry/{name}?digest=...` returns `RegistryPullResponse.content`; rendered in code viewer component |
| REG-04 | Admin can create/move/delete tags on registry entries | `POST /projects/{slug}/registry/{name}/tags` (upsert) + `DELETE /projects/{slug}/registry/{name}/tags/{tag}`; gated by `isProjectAdmin` pattern from Phase 28 |
</phase_requirements>

---

## Summary

Phase 29 is a pure frontend phase. All backend endpoints already exist and are tested. The work is building the React UI for browsing WDL registry entries within a project — a list page, a detail page, and two admin dialogs.

The codebase already has two near-identical examples of the exact patterns needed: `UsersPage.tsx` (table + badge + three-dot dropdown) and `ProjectDetailPage.tsx` (detail page + AlertDialog + Dialog). The registry feature folder follows these same conventions. The main new element is the WDL source viewer, which requires a syntax highlighting library decision.

WDL is not supported as a named language by any major syntax highlighter (Shiki, react-syntax-highlighter/Prism, highlight.js). The pragmatic choice is `react-syntax-highlighter` with Prism light build, using `bash` as a fallback language that provides meaningful keyword and string highlighting for WDL-like syntax. Alternatively, plain `<pre><code>` with Tailwind styling and a copy button avoids a new dependency entirely and is acceptable given WDL is not a widely-highlighted language.

**Primary recommendation:** Use `react-syntax-highlighter` (Prism light) with `bash` language fallback for WDL source display. If bundle size is a concern, ship a plain styled `<pre>` block with a copy button using the Clipboard API — zero new dependencies.

---

## Standard Stack

### Core (already installed)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `@tanstack/react-query` | ^5.64.2 | Data fetching + caching | Project-wide standard; all existing hooks use it |
| `react-hook-form` + `zod` | ^7.72.1 / ^4.3.6 | Tag creation dialog form + validation | Phase 27/28 pattern; already installed |
| `sonner` | ^1.7.4 | Toast notifications on mutation success/failure | Project-wide standard |
| `lucide-react` | ^0.475.0 | Icons (MoreHorizontal, Copy, Check) | Project-wide icon library |
| `shadcn/ui` components | — | Table, Badge, Dialog, AlertDialog, DropdownMenu, Select, Button | All already in `web/src/components/ui/` |

### New Dependency (one choice required)

**Option A — react-syntax-highlighter (RECOMMENDED):**
| Library | Version | Purpose | Notes |
|---------|---------|---------|-------|
| `react-syntax-highlighter` | 16.1.1 | WDL source display with syntax colors | Use Prism light + `bash` language; ~37KB min+gz |
| `@types/react-syntax-highlighter` | latest | TypeScript types | devDependency |

**Option B — No new dependency (zero-cost fallback):**
Use a styled `<pre><code>` block with Tailwind `font-mono` + a copy button (Clipboard API). No syntax colors, but no new dependency. Fully consistent with "read-only code block with copy button and line numbers" requirement.

**Recommendation: Start with Option B** during implementation. WDL is not supported by any highlighter as a named language anyway — `bash` as a fallback gives marginal benefit over a clean plain pre block. If team prefers colors, add `react-syntax-highlighter` in a follow-up task.

**Installation (if Option A):**
```bash
cd /home/uehara/flowy/web && pnpm add react-syntax-highlighter && pnpm add -D @types/react-syntax-highlighter
```

**Version verification:**
```bash
npm view react-syntax-highlighter version   # 16.1.1 as of 2026-04-07
```

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| react-syntax-highlighter | shiki | Shiki has no WDL grammar; larger bundle; async initialization adds complexity in React 19 |
| bash fallback | Custom WDL TextMate grammar | Too complex; WDL grammar not available in shiki or prism; out of scope |
| plain `<pre>` | Monaco editor | Monaco is >1MB; massive overkill for read-only display |

---

## Architecture Patterns

### Recommended Project Structure
```
web/src/features/registry/
├── useRegistry.ts          # All React Query hooks (list, versions, pull, tag, deleteTag, deleteEntry)
├── RegistryPage.tsx         # List page — /projects/:slug/registry
├── RegistryEntryPage.tsx    # Detail page — /projects/:slug/registry/:name
├── CreateTagDialog.tsx      # Admin: create/move tag dialog (Dialog + react-hook-form + zod)
├── DeleteTagDialog.tsx      # Admin: delete tag confirmation (AlertDialog)
├── WDLSourceViewer.tsx      # Read-only code block (pre/code or react-syntax-highlighter)
└── __tests__/
    ├── useRegistry.test.ts
    ├── RegistryPage.test.tsx
    └── RegistryEntryPage.test.tsx
```

### Pattern 1: React Query Hooks (useRegistry.ts)
**What:** Single file exporting all registry-related hooks following `useUsers.ts` / `useProjects.ts` patterns.
**When to use:** All API calls go through these hooks; pages never call `apiClient` directly.

```typescript
// Source: web/src/features/users/useUsers.ts (established pattern)
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiClient } from '@/lib/apiClient'

export interface RegistryEntryResponse {
  name: string
  kind: string            // "tool" | "workflow"
  version_count: number
  tags: string[]
  created_at: string
}

export interface RegistryVersionResponse {
  id: string
  digest: string          // "sha256:<hex>" — truncate for display
  tags: string[]
  created_at: string
}

export interface RegistryPullResponse {
  name: string
  digest: string
  kind: string
  content: string         // raw WDL source
}

export function useRegistryEntries(slug: string) {
  return useQuery<RegistryEntryResponse[]>({
    queryKey: ['projects', slug, 'registry'],
    queryFn: () => apiClient.get(`/projects/${slug}/registry`),
    enabled: !!slug,
  })
}

export function useRegistryVersions(slug: string, name: string) {
  return useQuery<RegistryVersionResponse[]>({
    queryKey: ['projects', slug, 'registry', name, 'versions'],
    queryFn: () => apiClient.get(`/projects/${slug}/registry/${name}/versions`),
    enabled: !!slug && !!name,
  })
}

export function useRegistryContent(slug: string, name: string, digest: string) {
  return useQuery<RegistryPullResponse>({
    queryKey: ['projects', slug, 'registry', name, 'content', digest],
    queryFn: () => apiClient.get(`/projects/${slug}/registry/${name}?digest=${encodeURIComponent(digest)}`),
    enabled: !!slug && !!name && !!digest,
  })
}

export function useDeleteRegistryEntry(slug: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (name: string) => apiClient.delete(`/projects/${slug}/registry/${name}`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['projects', slug, 'registry'] }),
  })
}

export function useUpsertTag(slug: string, name: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (data: { tag: string; digest: string }) =>
      apiClient.post(`/projects/${slug}/registry/${name}/tags`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', slug, 'registry', name] })
      queryClient.invalidateQueries({ queryKey: ['projects', slug, 'registry'] })
    },
  })
}

export function useDeleteTag(slug: string, name: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (tag: string) => apiClient.delete(`/projects/${slug}/registry/${name}/tags/${tag}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', slug, 'registry', name] })
      queryClient.invalidateQueries({ queryKey: ['projects', slug, 'registry'] })
    },
  })
}
```

### Pattern 2: Registry List Page (RegistryPage.tsx)
**What:** Table page at `/projects/:slug/registry` — direct clone of UsersPage.tsx structure.
**When to use:** REG-01 (list entries with kind badge, tags, version count, created date).

Key decisions:
- `useNavigate` + row `onClick` for row click → detail page navigation
- Admin check: same `currentMember?.role === 'admin'` pattern from Phase 28
- Three-dot menu with "Delete" only (no push/create in UI)
- Kind badge: `variant="outline"` with className for color: `tool` → blue, `workflow` → purple
- Tags: comma-separated text inline; if `tags.length === 0`, render `—`

```typescript
// Row click navigation pattern
<TableRow
  key={entry.name}
  className="cursor-pointer hover:bg-muted/50"
  onClick={() => navigate(`/projects/${slug}/registry/${entry.name}`)}
>
```

### Pattern 3: Registry Entry Detail Page (RegistryEntryPage.tsx)
**What:** Two-panel detail page at `/projects/:slug/registry/:name` — versions table (top) + WDL source viewer (bottom).
**When to use:** REG-02 (version list) + REG-03 (WDL source).

Key decisions:
- `useParams` to get both `slug` and `name`
- `selectedDigest` state — initialized to first version's digest (newest first from API)
- Clicking a version row sets `selectedDigest` → triggers `useRegistryContent` query
- Version digest display: truncate to first 16 chars after "sha256:" → `sha256:${digest.slice(7, 23)}...`
- Tag badges in version table: inline `<Badge>` per tag

### Pattern 4: Admin-only gating
**What:** Same pattern as ProjectDetailPage — derive `isProjectAdmin` from members list.
**When to use:** Conditionally render three-dot dropdown (delete entry), tag management buttons.

```typescript
// Source: web/src/features/projects/ProjectDetailPage.tsx (established pattern)
const { data: members } = useProjectMembers(slug!)
const currentUsername = useAuthStore((s) => s.username)
const currentMember = members?.find((m) => m.username === currentUsername)
const isProjectAdmin = currentMember?.role === 'admin'
```

### Pattern 5: WDLSourceViewer Component
**What:** Read-only code display with copy button and line numbers.
**Option B (no new dep — recommended):**

```typescript
// web/src/features/registry/WDLSourceViewer.tsx
import { useState } from 'react'
import { Copy, Check } from 'lucide-react'
import { Button } from '@/components/ui/button'

interface WDLSourceViewerProps {
  content: string
}

export function WDLSourceViewer({ content }: WDLSourceViewerProps) {
  const [copied, setCopied] = useState(false)

  async function handleCopy() {
    await navigator.clipboard.writeText(content)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const lines = content.split('\n')

  return (
    <div className="relative rounded-md border bg-muted font-mono text-sm overflow-auto max-h-[500px]">
      <div className="sticky top-0 right-0 flex justify-end p-1 bg-muted/80">
        <Button variant="ghost" size="icon" className="h-7 w-7" onClick={handleCopy} aria-label="Copy code">
          {copied ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
        </Button>
      </div>
      <table className="w-full border-separate border-spacing-0">
        <tbody>
          {lines.map((line, i) => (
            <tr key={i}>
              <td className="select-none text-right pr-4 pl-4 text-muted-foreground w-[3rem] text-xs">
                {i + 1}
              </td>
              <td className="pr-4 whitespace-pre">{line || ' '}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
```

### Pattern 6: Route Registration (App.tsx)
**What:** Add two new routes inside the existing ProtectedRoute > AppShell structure.

```typescript
// Add after existing project routes — source: web/src/App.tsx
<Route path="projects/:slug/registry" element={<RegistryPage />} />
<Route path="projects/:slug/registry/:name" element={<RegistryEntryPage />} />
```

**Note:** Sidebar.tsx already has the Registry nav link at `/projects/${activeSlug}/registry` — no changes to Sidebar required.

### Pattern 7: CreateTagDialog (admin only)
**What:** Dialog with tag name text input + digest selector (Select component with version digests).

Key decisions:
- `react-hook-form` + `zod` schema (same as CreateUserDialog pattern)
- Select shows all version digests from `useRegistryVersions`, formatted as truncated digest + tag names
- POST to `/projects/{slug}/registry/{name}/tags` with `{tag, digest}` — this is create AND move (upsert)
- On success: invalidate both `registry` and `registry/{name}` query keys

### Anti-Patterns to Avoid
- **Calling apiClient directly in components:** All API calls go through hooks in `useRegistry.ts`.
- **Using global `authStore.role` for project-admin check:** Use `currentMember?.role === 'admin'` from members list (Phase 28 established pattern).
- **Deriving isProjectAdmin in multiple places:** Compute once per page, pass to child components as prop if needed.
- **Not encoding the digest in URL query string:** `encodeURIComponent()` on the digest value — it contains `sha256:` which has a colon.
- **Loading all WDL content on page load:** Only fetch content for the selected version — `enabled: !!digest` in the query.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Confirmation dialogs | Custom modal | AlertDialog (shadcn/ui) | Accessible, keyboard-navigable, established pattern |
| Form state for tag dialog | Manual useState | react-hook-form + zod | Already installed; provides validation, error states |
| Copy-to-clipboard | Manual selection | `navigator.clipboard.writeText()` | Modern browser API, no library needed |
| Badge variants for kind | CSS string logic | Tailwind className on shadcn Badge | Consistent with existing badge usage |
| Digest display truncation | Complex regex | `digest.slice(7, 23)` after `sha256:` | Simple string slice; 16 hex chars uniquely identify a version |

---

## Common Pitfalls

### Pitfall 1: Digest encoding in URL query parameter
**What goes wrong:** `?digest=sha256:abc123` — the colon and content are sent literally, some servers reject the unencoded colon.
**Why it happens:** `fetch` does not auto-encode query string parameters.
**How to avoid:** Always `encodeURIComponent(digest)` when building the URL.
**Warning signs:** 400 or 404 responses when fetching by digest.

### Pitfall 2: Stale query cache after tag mutation
**What goes wrong:** Tag is created/deleted but the entry list still shows old tags; version table still shows old tag badges.
**Why it happens:** React Query invalidation must cover all affected query keys.
**How to avoid:** After tag mutations, invalidate both `['projects', slug, 'registry']` (entry list) and `['projects', slug, 'registry', name, 'versions']` (version list). The entry list query returns `tags[]` per entry which reflects all tags on all versions.

### Pitfall 3: `isProjectAdmin` during loading
**What goes wrong:** `members` is `undefined` during initial load → `currentMember` is `undefined` → `isProjectAdmin` is `false` → admin controls flash as hidden then appear.
**Why it happens:** Members query is async.
**How to avoid:** Guard admin-only buttons with `{isProjectAdmin && ...}` — the brief flash of non-admin state is acceptable and consistent with Phase 28 behavior.

### Pitfall 4: Selected version state on page navigation
**What goes wrong:** User navigates back to registry list then returns to the same entry — stale `selectedDigest` state.
**Why it happens:** Component re-mounts with useState initialized to first version.
**How to avoid:** Initialize `selectedDigest` from `versions?.[0]?.digest` using the query data — this auto-resets correctly on each mount.

### Pitfall 5: Row click conflicts with three-dot dropdown
**What goes wrong:** Clicking the three-dot dropdown button on a row also triggers row navigation.
**Why it happens:** Click event bubbles up to the `TableRow onClick`.
**How to avoid:** Call `e.stopPropagation()` in the DropdownMenuTrigger's onClick handler (or use `onClick={(e) => e.stopPropagation()}` wrapping the button).

---

## Code Examples

### API response shapes (from types.go)
```typescript
// Source: internal/api/types.go lines 289-312

// GET /projects/{slug}/registry
// Response: RegistryEntryResponse[]
interface RegistryEntryResponse {
  name: string           // entry slug name
  kind: string           // "tool" | "workflow"
  version_count: number
  tags: string[]         // all tags across all versions
  created_at: string     // ISO 8601
}

// GET /projects/{slug}/registry/{name}/versions
// Response: RegistryVersionResponse[]
interface RegistryVersionResponse {
  id: string
  digest: string         // "sha256:<64-char hex>"
  tags: string[]         // tags pointing to this specific version
  created_at: string
}

// GET /projects/{slug}/registry/{name}?digest=sha256:...
// Response: RegistryPullResponse
interface RegistryPullResponse {
  name: string
  digest: string
  kind: string
  content: string        // raw WDL source text
}

// POST /projects/{slug}/registry/{name}/tags (create or move tag)
// Request: { tag: string, digest: string }
// Response: { tag, digest, entry }

// DELETE /projects/{slug}/registry/{name}/tags/{tag}
// Response: 204 No Content

// DELETE /projects/{slug}/registry/{name}
// Response: 204 No Content
```

### Badge color pattern for kind
```typescript
// Consistent with shadcn Badge usage in UsersPage — use className override
function KindBadge({ kind }: { kind: string }) {
  return (
    <Badge
      variant="outline"
      className={kind === 'workflow'
        ? 'border-purple-300 text-purple-700 bg-purple-50'
        : 'border-blue-300 text-blue-700 bg-blue-50'
      }
    >
      {kind}
    </Badge>
  )
}
```

### Digest truncation pattern
```typescript
// Displays as "sha256:abcd1234efgh5678..." — 16 hex chars after "sha256:"
function truncateDigest(digest: string): string {
  if (digest.startsWith('sha256:')) {
    return `sha256:${digest.slice(7, 23)}...`
  }
  return digest
}
```

### Zod schema for CreateTagDialog
```typescript
import { z } from 'zod'

const tagSchema = z.object({
  tag: z.string().min(1).max(128).refine(
    (v) => !v.includes('@') && !v.includes(':'),
    { message: "Tag must not contain '@' or ':'" }
  ),
  digest: z.string().min(1, 'Select a version'),
})
type TagFormValues = z.infer<typeof tagSchema>
```

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Vitest 3.2.4 + @testing-library/react 16.3.2 |
| Config file | `web/vitest.config.ts` |
| Quick run command | `cd /home/uehara/flowy/web && pnpm test --run` |
| Full suite command | `cd /home/uehara/flowy/web && pnpm test --run` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REG-01 | Registry entry list renders with name, kind badge, versions count, tags, date | unit | `pnpm test --run -- RegistryPage` | ❌ Wave 0 |
| REG-01 | Empty state renders when no entries | unit | `pnpm test --run -- RegistryPage` | ❌ Wave 0 |
| REG-01 | Loading skeleton renders during fetch | unit | `pnpm test --run -- RegistryPage` | ❌ Wave 0 |
| REG-01 | Admin sees three-dot delete menu; viewer does not | unit | `pnpm test --run -- RegistryPage` | ❌ Wave 0 |
| REG-02 | Entry detail page renders version table with digest, tags, date | unit | `pnpm test --run -- RegistryEntryPage` | ❌ Wave 0 |
| REG-02 | First version selected by default | unit | `pnpm test --run -- RegistryEntryPage` | ❌ Wave 0 |
| REG-03 | WDL source is displayed when version selected | unit | `pnpm test --run -- RegistryEntryPage` | ❌ Wave 0 |
| REG-04 | Admin sees "Create Tag" / tag delete actions; viewer does not | unit | `pnpm test --run -- RegistryEntryPage` | ❌ Wave 0 |
| REG-04 | useRegistry hooks call correct API endpoints | unit | `pnpm test --run -- useRegistry` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `cd /home/uehara/flowy/web && pnpm test --run`
- **Per wave merge:** `cd /home/uehara/flowy/web && pnpm test --run`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `web/src/features/registry/__tests__/useRegistry.test.ts` — covers REG-01, REG-02, REG-03, REG-04 hook behavior
- [ ] `web/src/features/registry/__tests__/RegistryPage.test.tsx` — covers REG-01 list page rendering
- [ ] `web/src/features/registry/__tests__/RegistryEntryPage.test.tsx` — covers REG-02, REG-03, REG-04 detail page

---

## State of the Art

| Old Approach | Current Approach | Impact |
|--------------|------------------|--------|
| Global admin role for project actions | Project-scoped `currentMember?.role === 'admin'` from members API (Phase 28) | Must use members list, not authStore.role |
| Direct apiClient calls in components | React Query hooks in useXxx.ts files | All API calls via hooks pattern |
| shadcn CLI auto-placement | Manual move to `src/components/ui/` | Phase 27 established this; all ui/ components already present |

---

## Open Questions

1. **WDL syntax highlighting quality**
   - What we know: WDL is not supported in Shiki, react-syntax-highlighter/Prism, or highlight.js as a named language. `bash` fallback gives some highlighting.
   - What's unclear: Whether `bash` highlighting provides enough visual benefit to justify adding the dependency.
   - Recommendation: Implement with plain `<pre>` first (zero deps). If stakeholders want colors, add react-syntax-highlighter in a separate commit.

2. **Pagination for large version lists**
   - What we know: API returns all versions, no pagination parameters documented.
   - What's unclear: How many versions a high-traffic registry entry might accumulate.
   - Recommendation: Render all versions in the table (no pagination). CONTEXT.md marks this as Claude's Discretion; scrollable container with `max-h` is sufficient for v2.0.

---

## Sources

### Primary (HIGH confidence)
- `internal/server/handlers_registry.go` — All registry handler implementations; request/response behavior verified
- `internal/api/types.go` lines 263-312 — Exact API response shapes verified
- `web/src/features/users/UsersPage.tsx` — Table pattern to replicate
- `web/src/features/users/useUsers.ts` — Hook pattern to replicate
- `web/src/features/projects/ProjectDetailPage.tsx` — Detail page + admin gating pattern
- `web/src/lib/apiClient.ts` — API client interface verified
- `web/src/App.tsx` — Route registration structure verified
- `web/src/components/layout/Sidebar.tsx` — Registry nav link confirmed present, no changes needed
- `web/package.json` — Installed dependencies verified (no syntax highlighter present)
- `web/vitest.config.ts` — Test framework configuration verified

### Secondary (MEDIUM confidence)
- [npm: react-syntax-highlighter](https://www.npmjs.com/package/react-syntax-highlighter) — Version 16.1.1 confirmed; Prism light build for tree-shaking
- [npm view react-syntax-highlighter version] — 16.1.1 confirmed from registry

### Tertiary (LOW confidence)
- [Shiki languages documentation](https://shiki.style/languages) — WDL absence confirmed by inspection; no official negative statement found
- [react-syntax-highlighter AVAILABLE_LANGUAGES_PRISM.MD](https://github.com/react-syntax-highlighter/react-syntax-highlighter/blob/master/AVAILABLE_LANGUAGES_PRISM.MD) — WDL absence confirmed by full-list inspection (298 languages, no WDL entry)

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all dependencies verified from package.json; patterns verified from existing source files
- API shapes: HIGH — verified directly from handlers_registry.go and types.go
- Architecture: HIGH — direct derivation from established Phase 27/28 patterns in codebase
- WDL syntax highlighting: HIGH (negative) — verified WDL is absent from both Shiki and Prism language lists
- Pitfalls: MEDIUM — derived from codebase patterns and React Query behavior; not all verified by official sources

**Research date:** 2026-04-07
**Valid until:** 2026-05-07 (stable libraries; API is project-internal so no expiry)
