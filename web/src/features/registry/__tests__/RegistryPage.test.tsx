import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

// Mock useRegistry hooks
vi.mock('@/features/registry/useRegistry', () => ({
  useRegistryEntries: vi.fn(),
  useDeleteRegistryEntry: vi.fn(),
}))

// Mock useProjectMembers
vi.mock('@/features/projects/useProjects', () => ({
  useProjectMembers: vi.fn(),
}))

// Mock useAuthStore
vi.mock('@/store/authStore', () => ({
  useAuthStore: vi.fn(),
}))

// Mock toast
vi.mock('sonner', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

// Mock react-router
vi.mock('react-router', () => ({
  useParams: vi.fn(() => ({ slug: 'my-project' })),
  useNavigate: vi.fn(() => mockNavigate),
}))

const mockNavigate = vi.fn()

import { useRegistryEntries, useDeleteRegistryEntry } from '@/features/registry/useRegistry'
import { useProjectMembers } from '@/features/projects/useProjects'
import { useAuthStore } from '@/store/authStore'
import { RegistryPage } from '@/features/registry/RegistryPage'

const mockEntries = [
  {
    name: 'my-tool',
    kind: 'tool',
    version_count: 3,
    tags: ['latest', 'v1.0'],
    created_at: '2026-03-15T00:00:00Z',
  },
  {
    name: 'my-workflow',
    kind: 'workflow',
    version_count: 1,
    tags: [],
    created_at: '2026-03-20T00:00:00Z',
  },
]

const adminMembers = [{ username: 'alice', role: 'admin', user_id: 'u1', created_at: '2026-01-01T00:00:00Z' }]
const viewerMembers = [{ username: 'alice', role: 'viewer', user_id: 'u1', created_at: '2026-01-01T00:00:00Z' }]

function setupMocks({
  username = 'alice',
  entries = mockEntries,
  isLoading = false,
  isError = false,
  members = adminMembers,
} = {}) {
  vi.mocked(useAuthStore).mockImplementation((selector) => selector({ username } as never))
  vi.mocked(useRegistryEntries).mockReturnValue({
    data: isLoading ? undefined : entries,
    isLoading,
    isError,
  } as ReturnType<typeof useRegistryEntries>)
  vi.mocked(useProjectMembers).mockReturnValue({
    data: members,
    isLoading: false,
  } as ReturnType<typeof useProjectMembers>)
  vi.mocked(useDeleteRegistryEntry).mockReturnValue({
    mutateAsync: vi.fn().mockResolvedValue(undefined),
  } as unknown as ReturnType<typeof useDeleteRegistryEntry>)
}

describe('RegistryPage', () => {
  const user = userEvent.setup()

  beforeEach(() => {
    vi.clearAllMocks()
    mockNavigate.mockReset()
  })

  it('renders entry table with name, kind badge, version count, tags, created date', () => {
    setupMocks()
    render(<RegistryPage />)

    // Names
    expect(screen.getByText('my-tool')).toBeInTheDocument()
    expect(screen.getByText('my-workflow')).toBeInTheDocument()

    // Kind badges
    expect(screen.getByText('tool')).toBeInTheDocument()
    expect(screen.getByText('workflow')).toBeInTheDocument()

    // Version counts
    expect(screen.getByText('3')).toBeInTheDocument()
    expect(screen.getByText('1')).toBeInTheDocument()

    // Tags
    expect(screen.getByText('latest, v1.0')).toBeInTheDocument()
    // my-workflow has no tags — em dash
    expect(screen.getByText('\u2014')).toBeInTheDocument()

    // Created dates
    expect(screen.getByText('Mar 15, 2026')).toBeInTheDocument()
    expect(screen.getByText('Mar 20, 2026')).toBeInTheDocument()
  })

  it('renders loading skeleton rows', () => {
    setupMocks({ isLoading: true, entries: [] })
    render(<RegistryPage />)

    expect(screen.queryByText('my-tool')).not.toBeInTheDocument()
    const skeletons = screen.getAllByTestId('skeleton-row')
    expect(skeletons).toHaveLength(4)
  })

  it('renders empty state when no entries', () => {
    setupMocks({ entries: [] })
    render(<RegistryPage />)

    expect(screen.getByText('No registry entries.')).toBeInTheDocument()
    expect(screen.getByText(/Push a WDL workflow or task/)).toBeInTheDocument()
  })

  it('navigates to entry detail on row click', async () => {
    setupMocks()
    render(<RegistryPage />)

    const row = screen.getByText('my-tool').closest('tr')!
    await user.click(row)

    expect(mockNavigate).toHaveBeenCalledWith('/projects/my-project/registry/my-tool')
  })

  it('admin sees delete dropdown; viewer does not', async () => {
    // Admin case
    setupMocks({ members: adminMembers })
    const { unmount } = render(<RegistryPage />)

    const triggers = screen.getAllByRole('button', { name: /open menu/i })
    expect(triggers.length).toBeGreaterThan(0)
    unmount()

    // Viewer case
    setupMocks({ members: viewerMembers })
    render(<RegistryPage />)
    expect(screen.queryByRole('button', { name: /open menu/i })).not.toBeInTheDocument()
  })

  it('renders error state', () => {
    setupMocks({ isError: true, entries: [] })
    render(<RegistryPage />)

    expect(screen.getByText(/Failed to load registry/)).toBeInTheDocument()
  })
})
