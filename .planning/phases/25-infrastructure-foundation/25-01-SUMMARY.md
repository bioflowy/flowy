---
phase: 25-infrastructure-foundation
plan: "01"
subsystem: ui
tags: [react, vite, typescript, tailwind, shadcn, react-router, tanstack-query, zustand, lucide-react]

# Dependency graph
requires: []
provides:
  - Vite 6 + React 19 + TypeScript 5.7 + Tailwind v4 + shadcn/ui project scaffold in web/
  - AppShell layout with left sidebar navigation and Outlet-based content area
  - Sidebar component with Registry/Runs/Members/Users/Account navigation using NavLink active state
  - shadcn/ui components: button, separator, tooltip
  - QueryClientProvider with retry logic (no retry on 401/403)
  - BrowserRouter routing wired with Navigate redirect from / to /projects
  - Toaster (sonner) at root level for toast notifications
affects: [25-02, 25-03, 26-auth, 27-projects, 28-registry, 29-runs, 30-files]

# Tech tracking
tech-stack:
  added:
    - vite@6.4.1 (build tool)
    - react@19.2.4 + react-dom@19.2.4 (UI framework)
    - typescript@5.7.3 (type system)
    - tailwindcss@4.2.2 + @tailwindcss/vite@4.2.2 (CSS-first styling)
    - react-router@7.13.2 (client-side routing, library mode with BrowserRouter)
    - "@tanstack/react-query"@5.95.2 (server state management)
    - zustand@5.0.12 (client state)
    - lucide-react@0.475.0 (icons)
    - sonner@1.7.4 (toast notifications)
    - "@radix-ui/react-separator, @radix-ui/react-tooltip, @radix-ui/react-slot" (shadcn/ui primitives)
    - class-variance-authority@0.7.1, clsx@2.1.1, tailwind-merge@2.6.1 (styling utilities)
  patterns:
    - shadcn/ui components placed in web/src/components/ui/ (CLI-managed)
    - Custom layout components in web/src/components/layout/
    - Feature-scoped components in web/src/features/ (convention established for future phases)
    - Path alias "@" maps to web/src/ (consistent across tsconfig and vite.config)
    - Tailwind v4 CSS-first config: "@import tailwindcss" in index.css + OKLCH CSS custom properties
    - cn() utility wraps clsx + twMerge for conditional class names

key-files:
  created:
    - web/package.json
    - web/pnpm-lock.yaml
    - web/vite.config.ts
    - web/tsconfig.json
    - web/tsconfig.app.json
    - web/tsconfig.node.json
    - web/index.html
    - web/src/main.tsx
    - web/src/App.tsx
    - web/src/index.css
    - web/src/vite-env.d.ts
    - web/src/lib/utils.ts
    - web/src/components/layout/AppShell.tsx
    - web/src/components/layout/Sidebar.tsx
    - web/src/components/ui/button.tsx
    - web/src/components/ui/separator.tsx
    - web/src/components/ui/tooltip.tsx
    - web/components.json
  modified:
    - .gitignore (added web/node_modules/, web/dist/, web/.pnpmfile.cjs)

key-decisions:
  - "pnpm create vite@6 is interactive-only; project scaffolded manually with exact file contents"
  - "esbuild optional build scripts blocked by pnpm sandbox; Vite found compatible esbuild via npm global cache — build still succeeds"
  - "AppShell and Sidebar created in Task 1 commit because App.tsx depends on both; Task 2 is verification-only"
  - "CSS variables use OKLCH color space matching Tailwind v4 shadcn/ui slate preset per UI-SPEC"

patterns-established:
  - "Layout pattern: flex h-screen overflow-hidden root, w-60 sidebar with bg-sidebar border-r, flex-1 overflow-y-auto p-6 content"
  - "Nav pattern: NavLink with isActive callback selects between bg-primary/text-primary-foreground (active) and text-muted-foreground/hover:bg-accent (inactive)"
  - "Query pattern: QueryClient with no retry on 401/403 (auth errors surface immediately)"

requirements-completed: [INFRA-03]

# Metrics
duration: 4min
completed: 2026-03-24
---

# Phase 25 Plan 01: Vite + React 19 frontend scaffold with AppShell sidebar layout

**Vite 6 + React 19 + TypeScript + Tailwind v4 + shadcn/ui SPA scaffold in web/ with sidebar navigation shell (Registry, Runs, Members, Users, Account) and QueryClientProvider/BrowserRouter root**

## Performance

- **Duration:** ~4 min
- **Started:** 2026-03-24T02:54:34Z
- **Completed:** 2026-03-24T02:58:12Z
- **Tasks:** 2
- **Files modified:** 19

## Accomplishments

- Complete Vite 6 + React 19 + TypeScript 5.7 frontend workspace in web/ builds to web/dist/ with `pnpm build`
- AppShell layout (flex h-screen, sidebar + Outlet content area) and Sidebar navigation with NavLink active state styling
- Full dependency stack installed: react-router v7, @tanstack/react-query v5, zustand v5, lucide-react, sonner, shadcn/ui primitives

## Task Commits

Each task was committed atomically:

1. **Task 1: Scaffold Vite + React + TypeScript project** - `3cfdbc8` (feat)
2. **Task 2: Build AppShell and Sidebar** - (included in `3cfdbc8` — both tasks produced the same files; App.tsx required layout components to compile)

## Files Created/Modified

- `web/package.json` - Project manifest with all dependencies
- `web/vite.config.ts` - Vite config: Tailwind v4 plugin, @ alias, /api proxy, base: './'
- `web/tsconfig.json`, `web/tsconfig.app.json`, `web/tsconfig.node.json` - TypeScript project references config
- `web/index.html` - HTML entry point
- `web/src/main.tsx` - React entry point using createRoot
- `web/src/App.tsx` - Root component: QueryClientProvider + BrowserRouter + Routes + Toaster
- `web/src/index.css` - Tailwind v4 CSS-first import + OKLCH shadcn/ui CSS custom properties
- `web/src/lib/utils.ts` - cn() utility (clsx + twMerge)
- `web/src/components/layout/AppShell.tsx` - Root layout: flex h-screen, Sidebar + Outlet
- `web/src/components/layout/Sidebar.tsx` - Left nav: flowy title, project selector placeholder, Registry/Runs/Members/Users/Account NavLinks
- `web/src/components/ui/button.tsx` - shadcn/ui Button component (CVA variants)
- `web/src/components/ui/separator.tsx` - shadcn/ui Separator (Radix primitive)
- `web/src/components/ui/tooltip.tsx` - shadcn/ui Tooltip (Radix primitive)
- `web/components.json` - shadcn/ui config (slate base, CSS variables)
- `web/src/vite-env.d.ts` - Vite client types
- `web/pnpm-lock.yaml` - Locked dependency manifest
- `.gitignore` - Added web/node_modules/, web/dist/, web/.pnpmfile.cjs

## Decisions Made

- pnpm create vite@6 is interactive-only; scaffolded project manually with exact file contents matching the template.
- esbuild build scripts blocked by pnpm sandbox policy; build succeeded anyway because Vite resolves esbuild via the system npm cache.
- AppShell and Sidebar were created in the same commit as the scaffold since App.tsx requires both to compile — Task 2 had no additional files to commit separately.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] pnpm create vite@6 used interactive prompts that cannot run non-interactively**
- **Found during:** Task 1 (scaffold step)
- **Issue:** `pnpm create vite@6 web -- --template react-ts` launched an interactive CLI selector that blocked in the non-interactive environment
- **Fix:** Created all project files manually (package.json, tsconfig files, index.html, src/main.tsx, src/App.tsx) with exact content matching the Vite react-ts template
- **Files modified:** All files listed in Task 1
- **Verification:** `pnpm build` exits 0, produces web/dist/index.html
- **Committed in:** 3cfdbc8

---

**Total deviations:** 1 auto-fixed (1 blocking — non-interactive environment)
**Impact on plan:** Deviation was a tooling constraint, not a design change. Scaffolded files are equivalent to `pnpm create vite@6 --template react-ts` output.

## Issues Encountered

- pnpm not on PATH initially; installed globally via `npm install -g pnpm` (one-time setup).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- web/ workspace ready for Plan 02 (flowy-ui Go binary with go:embed)
- web/dist/ produced by `pnpm build` will be embedded via `//go:embed all:web/dist` in Plan 03
- Sidebar navigation items (Registry, Runs, Members, Users) are stubs — feature screens added in Phases 27-30
- No mobile breakpoints — out of scope per REQUIREMENTS.md

---
*Phase: 25-infrastructure-foundation*
*Completed: 2026-03-24*
