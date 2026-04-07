# Phase 28: Project Management - Research

**Researched:** 2026-04-06
**Domain:** React + TypeScript frontend — project list, project detail, member management
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### Project list display
- Card grid layout on `/projects` page — responsive grid of project cards
- Each card shows: project name (title), slug (subtitle), description snippet (2 lines max), member count
- Clicking a card sets it as the active project in the sidebar dropdown AND navigates to `/projects/{slug}`
- Default landing page after login remains `/projects` — user picks a project from the grid
- Sidebar has a project dropdown selector at top; selecting a project shows project-scoped nav items (Registry, Runs, Members) below it
- When no project is selected, sidebar shows only global nav (Users for admins) and the project dropdown with placeholder

#### Project detail page
- Single scrollable page at `/projects/{slug}` with two sections: project info header + member list table
- Header shows: project name, slug, description, "Edit Description" and "Delete Project" buttons (visible to project admins only)
- Member table below header with columns: Username, Role (inline Select dropdown), Joined date, actions (three-dot menu)
- "Add Member" button above the member table
- URL structure: `/projects/{slug}` for detail, future sub-pages at `/projects/{slug}/registry`, `/projects/{slug}/runs`
- Project admins can edit description and delete the project (not limited to global admins)

#### Member management
- Add member: Dialog overlay with user dropdown (all active users not yet in project) + role select (admin/executor/viewer, default viewer) + Add button
- Role change: Inline Select dropdown in role column — changing immediately updates via API + success toast (no dialog)
- Remove member: Three-dot dropdown on row with "Remove" option — confirmation dialog "Remove {username} from {project}?" with destructive red button
- Self-protection: hide three-dot menu and disable role dropdown for the logged-in user's own row (same pattern as Phase 27 users table)

#### Project creation
- Dedicated `/projects/new` page (not a dialog) — "Create Project" button on the project list page navigates here
- Fields: Name (required), Slug (auto-generated from name, manually overridable), Description (optional textarea)
- Slug auto-generation: lowercase name with spaces replaced by hyphens, user can edit the slug field manually
- Only global admins can create projects (show "Create Project" button only when `role === 'admin'`)
- Creator is automatically added as project admin (existing backend behavior)
- After creation: navigate to `/projects/{slug}` and set as active project in sidebar

### Claude's Discretion
- Empty state for project list (no projects yet)
- Loading skeletons for card grid and member table
- Project card hover/click styling
- Exact badge colors for roles in member table
- Delete project confirmation dialog wording
- Edit description UX (inline edit vs dialog)

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| PROJ-01 | User can view list of projects they belong to | `GET /projects` API ready; returns filtered list by membership; `ProjectResponse` type confirmed |
| PROJ-02 | User can create a new project | `POST /projects` API ready; `CreateProjectRequest` requires only `slug` + optional `description`; slug validation pattern documented |
| PROJ-03 | Admin can manage project members (add/remove/change role) | `POST/PUT/DELETE /projects/{slug}/members/{username}` all implemented; `AddMemberRequest`, `UpdateMemberRequest`, `MemberResponse` types confirmed |
| PROJ-04 | User can view project dashboard with summary info | `GET /projects/{slug}` + `GET /projects/{slug}/members` both ready; combined render on detail page |
</phase_requirements>

## Summary

Phase 28 is a pure frontend phase — all backend endpoints are fully implemented and tested. The work is creating a `web/src/features/projects/` feature folder following the exact same patterns established in Phase 26 (auth) and Phase 27 (users). No new patterns need to be invented.

The primary complexity is the **project context store**: a new Zustand store is needed to track the active project slug, because `Sidebar.tsx` and project-scoped nav links must respond to which project the user has selected. This store is analogous to `authStore.ts` — simple, persisted (sessionStorage), holds `activeProjectSlug` and a setter.

The secondary complexity is the **sidebar refactor**: `Sidebar.tsx` currently has static project nav items (`/registry`, `/runs`, `/members`) that are placeholder paths. This phase must wire them to become `/projects/{slug}/registry`, etc., gated on whether an active project is selected. The sidebar also needs the project dropdown selector (currently a placeholder div) to become a functional component showing project names and allowing selection.

**Primary recommendation:** Build in this order: (1) projectStore, (2) project hooks + API types, (3) ProjectsPage (list + cards), (4) CreateProjectPage, (5) ProjectDetailPage, (6) AddMemberDialog, (7) Sidebar wiring — each building on the previous.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| React | 19 | UI rendering | Project baseline |
| TypeScript | 5.x | Type safety | Project baseline |
| @tanstack/react-query | (existing) | Server state, mutations, invalidation | Used in useUsers.ts pattern |
| zustand | (existing) | Client state (active project) | Used in authStore.ts |
| react-hook-form | (existing) | Form state for CreateProjectPage, AddMemberDialog | Used in CreateUserDialog.tsx |
| zod | (existing) | Schema validation | Used in createUserSchema |
| @hookform/resolvers | (existing) | Bridge react-hook-form + zod | Used in existing dialogs |
| react-router | (existing) | Client-side routing | Used in App.tsx, Sidebar.tsx |
| sonner | (existing) | Toast notifications | Used throughout |
| lucide-react | (existing) | Icons | Used in Sidebar.tsx |

### Supporting (shadcn/ui components already installed)
| Component | File | Purpose |
|-----------|------|---------|
| Card, CardHeader, CardContent | `components/ui/card.tsx` | Project grid cards |
| Table, TableBody, etc. | `components/ui/table.tsx` | Member table |
| Badge | `components/ui/badge.tsx` | Role badges |
| Select, SelectTrigger, etc. | `components/ui/select.tsx` | Inline role dropdown |
| DropdownMenu, DropdownMenuItem | `components/ui/dropdown-menu.tsx` | Three-dot action menu |
| AlertDialog | `components/ui/alert-dialog.tsx` | Remove member + delete project confirmation |
| Dialog, DialogContent, etc. | `components/ui/dialog.tsx` | Add member dialog |
| Button | `components/ui/button.tsx` | Actions |
| Input | `components/ui/input.tsx` | Name, slug, description fields |
| Form, FormField, etc. | `components/ui/form.tsx` | Validated form fields |

**No new packages to install** — entire stack is already present.

## Architecture Patterns

### Recommended Project Structure
```
web/src/features/projects/
├── useProjects.ts          # React Query hooks + TypeScript interfaces for projects/members
├── ProjectsPage.tsx        # /projects — card grid
├── CreateProjectPage.tsx   # /projects/new — creation form page
├── ProjectDetailPage.tsx   # /projects/{slug} — project info + member table
└── AddMemberDialog.tsx     # Dialog: user dropdown + role select + Add button

web/src/store/
├── authStore.ts            # Existing
└── projectStore.ts         # NEW: active project slug + setter (zustand + persist)
```

### Pattern 1: React Query hooks (replicate useUsers.ts exactly)

```typescript
// Source: web/src/features/users/useUsers.ts (established pattern)
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiClient } from '@/lib/apiClient'

export interface ProjectResponse {
  id: string
  slug: string
  description: string
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface MemberResponse {
  user_id: string
  username: string
  role: string
  created_at: string
  updated_at: string
}

export function useProjects() {
  return useQuery<ProjectResponse[]>({
    queryKey: ['projects'],
    queryFn: () => apiClient.get('/projects'),
  })
}

export function useProjectDetail(slug: string) {
  return useQuery<ProjectResponse>({
    queryKey: ['projects', slug],
    queryFn: () => apiClient.get(`/projects/${slug}`),
  })
}

export function useProjectMembers(slug: string) {
  return useQuery<MemberResponse[]>({
    queryKey: ['projects', slug, 'members'],
    queryFn: () => apiClient.get(`/projects/${slug}/members`),
  })
}

export function useCreateProject() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (data: { slug: string; description?: string }) =>
      apiClient.post('/projects', data),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['projects'] }),
  })
}

export function useUpdateProject(slug: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (data: { description: string }) =>
      apiClient.put(`/projects/${slug}`, data),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['projects', slug] }),
  })
}

export function useDeleteProject(slug: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => apiClient.delete(`/projects/${slug}`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['projects'] }),
  })
}

export function useAddMember(slug: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (data: { username: string; role: string }) =>
      apiClient.post(`/projects/${slug}/members`, data),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: ['projects', slug, 'members'] }),
  })
}

export function useUpdateMember(slug: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ username, role }: { username: string; role: string }) =>
      apiClient.put(`/projects/${slug}/members/${username}`, { role }),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: ['projects', slug, 'members'] }),
  })
}

export function useRemoveMember(slug: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (username: string) =>
      apiClient.delete(`/projects/${slug}/members/${username}`),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: ['projects', slug, 'members'] }),
  })
}
```

### Pattern 2: Project context store (new Zustand store)

```typescript
// Source: web/src/store/authStore.ts (established pattern)
// web/src/store/projectStore.ts
import { create } from 'zustand'
import { persist, createJSONStorage } from 'zustand/middleware'

interface ProjectState {
  activeProjectSlug: string | null
  setActiveProject: (slug: string | null) => void
}

export const useProjectStore = create<ProjectState>()(
  persist(
    (set) => ({
      activeProjectSlug: null,
      setActiveProject: (slug) => set({ activeProjectSlug: slug }),
    }),
    {
      name: 'flowy-project',
      storage: createJSONStorage(() => sessionStorage),
    }
  )
)
```

### Pattern 3: Slug auto-generation from name

```typescript
// Derived from backend slugPattern: ^[a-z0-9]([a-z0-9-]{0,62}[a-z0-9])?$
function generateSlug(name: string): string {
  return name
    .toLowerCase()
    .trim()
    .replace(/\s+/g, '-')       // spaces to hyphens
    .replace(/[^a-z0-9-]/g, '') // strip non-alphanumeric
    .replace(/^-+|-+$/g, '')    // trim leading/trailing hyphens
    .substring(0, 64)
}
```

Zod schema for slug validation must mirror the backend pattern:
```typescript
const slugSchema = z.string()
  .min(1, 'Slug is required')
  .max(64)
  .regex(/^[a-z0-9]([a-z0-9-]{0,62}[a-z0-9])?$/, 'Lowercase alphanumeric and hyphens only, no leading/trailing hyphens')
```

### Pattern 4: Admin role detection for project-level admin

**IMPORTANT:** Project-level admin and global admin are DIFFERENT.

- `authStore.role === 'admin'` = global admin (can create projects, see all users)
- Project admin role = `MemberResponse.role === 'admin'` for the current user's row in members list

For the project detail page, the "Edit Description" and "Delete Project" buttons need the current user's project role. Derive it from the members list:

```typescript
const currentUsername = useAuthStore((s) => s.username)
const { data: members } = useProjectMembers(slug)
const currentMember = members?.find((m) => m.username === currentUsername)
const isProjectAdmin = currentMember?.role === 'admin'
```

### Pattern 5: Sidebar wiring with active project

The sidebar needs to show project-scoped nav only when an active project is selected:

```typescript
// In Sidebar.tsx
const activeSlug = useProjectStore((s) => s.activeProjectSlug)

// Project-scoped nav items (only shown when a project is selected)
const projectNavItems = activeSlug ? [
  { to: `/projects/${activeSlug}/registry`, label: 'Registry', icon: Package },
  { to: `/projects/${activeSlug}/runs`, label: 'Runs', icon: Play },
  { to: `/projects/${activeSlug}/members`, label: 'Members', icon: Users },
] : []
```

The project dropdown in the sidebar needs to fetch all projects and let the user select one. When a card is clicked on ProjectsPage, it calls `setActiveProject(slug)` then `navigate('/projects/' + slug)`. When the dropdown changes, same action.

### Pattern 6: Inline role Select (no dialog)

```typescript
// Role column cell in member table — immediate API call on change
<Select
  value={member.role}
  onValueChange={async (newRole) => {
    if (member.username === currentUsername) return // self-protection
    try {
      await updateMember.mutateAsync({ username: member.username, role: newRole })
      toast.success('Role updated.')
    } catch {
      toast.error('Something went wrong. Please try again.')
    }
  }}
  disabled={member.username === currentUsername}
>
  <SelectTrigger className="w-28">
    <SelectValue />
  </SelectTrigger>
  <SelectContent>
    <SelectItem value="admin">admin</SelectItem>
    <SelectItem value="executor">executor</SelectItem>
    <SelectItem value="viewer">viewer</SelectItem>
  </SelectContent>
</Select>
```

### Anti-Patterns to Avoid
- **Fetching users list in AddMemberDialog from `/users` as global admin**: The add member dialog needs the list of users not yet in the project. Fetch users from `/users` (global admin) filtered by those not in the members list. BUT `/users` is admin-only. Non-admin project admins cannot call `/users`. If the logged-in user is a project admin but not global admin, this approach fails. **Resolution:** The "Add Member" flow uses a text input for username (not a dropdown) for non-global-admins, OR the backend is checked — since `handleAddMember` takes `username` as a string, a plain text Input + lookup is fine. Given CONTEXT.md says "user dropdown (all active users not yet in project)" — this needs project admins who are also global admins, or the component can degrade to text input. The safest approach: use text Input for username, not a Select dropdown — avoids needing the full user list.
- **Using `name` field in `CreateProjectRequest`**: The backend `CreateProjectRequest` has only `slug` and `description` — there is NO `name` field. The "Name" field on the creation form is purely frontend and drives slug auto-generation. The slug IS the display name in this system.
- **Assuming project nav links work without active project**: `/registry`, `/runs`, `/members` must be project-scoped paths. Render them only when `activeProjectSlug` is set; otherwise show placeholder or nothing.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Form validation | Custom validators | zod schema + react-hook-form | Established pattern in CreateUserDialog |
| Optimistic loading | Manual loading state | React Query `isLoading` flag | Already used in UsersPage skeleton rows |
| Active project persistence | localStorage manual code | Zustand persist middleware (sessionStorage) | Same as authStore pattern |
| Toast notifications | Custom notification component | sonner `toast.success/error` | Already configured in App.tsx |
| Confirmation dialogs | Custom modal | AlertDialog from shadcn/ui | Used in DeactivateDialog (Phase 27) |
| Route protection | Manual redirect logic | ProtectedRoute component | Already in App.tsx |

**Key insight:** Every UX pattern in this phase has a direct precedent in Phase 26 or 27. Copy structures, adapt for project/member domain.

## Common Pitfalls

### Pitfall 1: CreateProjectRequest has no `name` field
**What goes wrong:** Developer adds `name` to the API call, backend returns 400 or ignores it.
**Why it happens:** CONTEXT.md says "Fields: Name (required), Slug", implying two separate API fields.
**How to avoid:** Read `internal/api/types.go` — `CreateProjectRequest` has only `slug` + `description`. The "Name" input in the form auto-populates the `slug` field. There is no separate name concept in the backend.
**Warning signs:** API returns 400 or project is created without visible name.

### Pitfall 2: Member update/remove endpoints use `username` not `user_id`
**What goes wrong:** Developer passes user ID in the URL path, gets 404.
**Why it happens:** Pattern mismatch — User management endpoints use `{id}` (UUID), but member endpoints use `{username}` (string).
**How to avoid:** Check `server.go` route registration: `PUT /members/{username}`, `DELETE /members/{username}`. Use `member.username` in the URL, not `member.user_id`.
**Warning signs:** 404 responses when updating/removing members.

### Pitfall 3: Project admin vs global admin confusion
**What goes wrong:** "Edit Description" and "Delete Project" buttons shown/hidden based on `authStore.role === 'admin'` (global), but should use project-level role.
**Why it happens:** Phase 27 patterns only needed global admin check. Phase 28 introduces a second authorization layer.
**How to avoid:** Derive `isProjectAdmin` from the current user's entry in the members list (see Pattern 4 above). Global admins may not be project admins if they were never added.
**Warning signs:** Non-project-admins can see edit/delete buttons; project admins who aren't global admins can't.

### Pitfall 4: Sidebar nav links showing before project selected
**What goes wrong:** User clicks Registry/Runs/Members in sidebar before selecting a project, navigates to a broken URL like `/projects/undefined/registry`.
**Why it happens:** `activeProjectSlug` is null initially; string interpolation produces "undefined".
**How to avoid:** Guard project nav items: render only when `activeProjectSlug` is not null. Show placeholder text or disabled state otherwise.
**Warning signs:** `/projects/undefined/registry` appears in browser URL bar.

### Pitfall 5: AlertDialog async confirm pattern (Phase 27 decision)
**What goes wrong:** Confirmation dialog closes prematurely during async delete operation.
**Why it happens:** Shadcn AlertDialog default behavior closes on action button click before async completes.
**How to avoid:** Use `e.preventDefault()` on the action button's onClick to block default close, then call `onOpenChange(false)` manually after the `await` completes. This is the Phase 27 pattern already established.
**Warning signs:** Dialog flickers closed then reopens, or closes before API responds.

### Pitfall 6: Add Member user dropdown requires admin-only `/users` endpoint
**What goes wrong:** Non-global-admin project admins can't populate the "users not yet in project" dropdown.
**Why it happens:** `GET /users` has `roleRequired("admin")` middleware — only global admins can list all users.
**How to avoid:** Implement AddMemberDialog with a plain text Input for username instead of a Select dropdown. The `AddMemberRequest` takes `username` as a string — text entry works perfectly. Only show the user-selection dropdown when `authStore.role === 'admin'`.
**Warning signs:** 403 error when AddMemberDialog opens for non-admin project admins.

## Code Examples

### Route additions to App.tsx
```typescript
// Source: web/src/App.tsx (established pattern)
import { ProjectsPage } from '@/features/projects/ProjectsPage'
import { CreateProjectPage } from '@/features/projects/CreateProjectPage'
import { ProjectDetailPage } from '@/features/projects/ProjectDetailPage'

// Inside <Route element={<AppShell />}>:
<Route path="projects" element={<ProjectsPage />} />
<Route path="projects/new" element={<CreateProjectPage />} />
<Route path="projects/:slug" element={<ProjectDetailPage />} />
```

Note: The root `<Route index>` already redirects to `/projects` (confirmed in current App.tsx).

### Reading slug param in ProjectDetailPage
```typescript
import { useParams } from 'react-router'

export function ProjectDetailPage() {
  const { slug } = useParams<{ slug: string }>()
  // slug is guaranteed non-null inside this route
}
```

### Card click sets active project
```typescript
import { useNavigate } from 'react-router'
import { useProjectStore } from '@/store/projectStore'

const navigate = useNavigate()
const setActiveProject = useProjectStore((s) => s.setActiveProject)

function handleCardClick(slug: string) {
  setActiveProject(slug)
  navigate(`/projects/${slug}`)
}
```

### Self-protection in member row (mirrors UsersPage pattern)
```typescript
// Source: web/src/features/users/UsersPage.tsx line 118
const currentUsername = useAuthStore((s) => s.username)

// In member row:
{member.username === currentUsername ? null : (
  <DropdownMenu>...</DropdownMenu>
)}
// And for inline role Select:
disabled={member.username === currentUsername}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Static sidebar nav items | Dynamic project-scoped nav from projectStore | Phase 28 | Sidebar must be refactored |
| No project context | Zustand projectStore with sessionStorage persist | Phase 28 | New store file required |
| Static `/registry` path | `/projects/{slug}/registry` pattern | Phase 28 | Future phases build on this |

**Deprecated/outdated:**
- Static `projectNavItems` array in Sidebar.tsx with hardcoded paths `/registry`, `/runs`, `/members`: Replace with dynamic paths based on `activeProjectSlug`.
- "Select project" placeholder div in Sidebar.tsx: Replace with functional project dropdown.

## Open Questions

1. **Add Member dialog: user dropdown vs text input for non-admin project admins**
   - What we know: `GET /users` requires global admin; `AddMemberRequest` accepts `username` string
   - What's unclear: Whether all project admins in practice will also be global admins
   - Recommendation: Implement as text Input for username (works for all users). If `authStore.role === 'admin'`, could optionally show a dropdown from the users list. Text input is simpler and universally correct.

2. **Edit description UX: inline edit vs dialog**
   - What we know: CONTEXT.md marks this as Claude's discretion
   - What's unclear: Preference between inline editing (click to edit, save/cancel buttons) vs dialog
   - Recommendation: Use a simple Dialog with a textarea (mirrors CreateUserDialog pattern) — consistent with established dialog-for-mutations pattern in the codebase.

3. **Member count in project cards**
   - What we know: `ProjectResponse` has no `member_count` field; `GET /projects/{slug}/members` returns full list
   - What's unclear: Whether member count should be fetched per-card (N+1) or if backend needs new field
   - Recommendation: Omit member count from cards for now or add "(view members)" text. N+1 fetches on card grid would be expensive. Backend `ProjectResponse` would need to add `member_count` for this to work well. Since CONTEXT.md specifies it, consider either adding a `member_count` field to the response, or just fetching members for each project in parallel queries — but this creates N requests. Simplest: remove member count from cards, show it only on the detail page.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Vitest (vitest.config.ts in web/) |
| Config file | `web/vitest.config.ts` |
| Quick run command | `cd web && pnpm test --run` |
| Full suite command | `cd web && pnpm test --run` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PROJ-01 | ProjectsPage renders project cards from API | unit | `cd web && pnpm test --run ProjectsPage` | ❌ Wave 0 |
| PROJ-01 | ProjectsPage shows empty state when no projects | unit | `cd web && pnpm test --run ProjectsPage` | ❌ Wave 0 |
| PROJ-01 | ProjectsPage shows loading skeletons while fetching | unit | `cd web && pnpm test --run ProjectsPage` | ❌ Wave 0 |
| PROJ-02 | CreateProjectPage renders form fields | unit | `cd web && pnpm test --run CreateProjectPage` | ❌ Wave 0 |
| PROJ-02 | CreateProjectPage auto-generates slug from name | unit | `cd web && pnpm test --run CreateProjectPage` | ❌ Wave 0 |
| PROJ-02 | CreateProjectPage submits with correct API shape | unit | `cd web && pnpm test --run CreateProjectPage` | ❌ Wave 0 |
| PROJ-03 | ProjectDetailPage shows Add Member button for project admins | unit | `cd web && pnpm test --run ProjectDetailPage` | ❌ Wave 0 |
| PROJ-03 | ProjectDetailPage hides three-dot menu for current user row | unit | `cd web && pnpm test --run ProjectDetailPage` | ❌ Wave 0 |
| PROJ-03 | AddMemberDialog submits correct username and role | unit | `cd web && pnpm test --run AddMemberDialog` | ❌ Wave 0 |
| PROJ-04 | ProjectDetailPage renders project slug and description | unit | `cd web && pnpm test --run ProjectDetailPage` | ❌ Wave 0 |
| PROJ-04 | ProjectDetailPage renders member table with roles | unit | `cd web && pnpm test --run ProjectDetailPage` | ❌ Wave 0 |
| PROJ-04 | useProjects hook fetches from GET /projects | unit | `cd web && pnpm test --run useProjects` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `cd web && pnpm test --run`
- **Per wave merge:** `cd web && pnpm test --run`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `web/src/features/projects/__tests__/useProjects.test.ts` — covers PROJ-01 data fetching
- [ ] `web/src/features/projects/__tests__/ProjectsPage.test.tsx` — covers PROJ-01 render
- [ ] `web/src/features/projects/__tests__/CreateProjectPage.test.tsx` — covers PROJ-02
- [ ] `web/src/features/projects/__tests__/ProjectDetailPage.test.tsx` — covers PROJ-03, PROJ-04
- [ ] `web/src/features/projects/__tests__/AddMemberDialog.test.tsx` — covers PROJ-03 add member

## Sources

### Primary (HIGH confidence)
- `internal/api/types.go` — All request/response types verified: `CreateProjectRequest`, `UpdateProjectRequest`, `ProjectResponse`, `AddMemberRequest`, `UpdateMemberRequest`, `MemberResponse`
- `internal/server/handlers_projects.go` — Project CRUD endpoint behavior confirmed
- `internal/server/handlers_members.go` — Member endpoints use `{username}` path param (not `{id}`)
- `internal/server/server.go` lines 130-180 — Route registration confirmed, all 8 project/member endpoints verified
- `web/src/features/users/useUsers.ts` — Definitive React Query hook pattern
- `web/src/features/users/UsersPage.tsx` — Table + self-protection + skeleton + dialog pattern
- `web/src/features/users/CreateUserDialog.tsx` — Dialog + react-hook-form + zod pattern
- `web/src/store/authStore.ts` — Zustand + persist pattern
- `web/src/components/layout/Sidebar.tsx` — Current sidebar structure to be extended
- `web/src/App.tsx` — Route structure to be extended
- `web/vitest.config.ts` — Test configuration confirmed (vitest + jsdom + threads pool)

### Secondary (MEDIUM confidence)
- Existing test files in `web/src/features/users/__tests__/` — Confirm vi.mock pattern for hooks and child components

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all packages are already installed; verified from package.json imports in existing files
- Architecture: HIGH — all patterns directly verified from existing Phase 26/27 source files
- Pitfalls: HIGH — derived from reading actual backend handler code and confirmed API type definitions
- API endpoints: HIGH — read directly from server.go route registration and handler implementations

**Research date:** 2026-04-06
**Valid until:** Stable — backend is complete; only frontend work remains; no external dependencies
