import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

// Mock useRegistry hooks
vi.mock('@/features/registry/useRegistry', () => ({
  useRegistryVersions: vi.fn(),
  useRegistryContent: vi.fn(),
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
  useParams: vi.fn(() => ({ slug: 'my-project', name: 'hello-workflow' })),
}))

// Mock child dialogs to keep tests focused
vi.mock('@/features/registry/CreateTagDialog', () => ({
  CreateTagDialog: ({ open }: { open: boolean }) =>
    open ? <div data-testid="create-tag-dialog" /> : null,
}))

vi.mock('@/features/registry/DeleteTagDialog', () => ({
  DeleteTagDialog: ({ open }: { open: boolean }) =>
    open ? <div data-testid="delete-tag-dialog" /> : null,
}))

vi.mock('@/features/registry/RunWorkflowDialog', () => ({
  RunWorkflowDialog: ({ open }: { open: boolean }) =>
    open ? <div data-testid="run-workflow-dialog" /> : null,
}))

import { useRegistryVersions, useRegistryContent } from '@/features/registry/useRegistry'
import { useProjectMembers } from '@/features/projects/useProjects'
import { useAuthStore } from '@/store/authStore'
import { RegistryEntryPage } from '@/features/registry/RegistryEntryPage'

const mockVersions = [
  {
    id: 'v1',
    digest: 'sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890',
    tags: ['latest', 'v1.0'],
    created_at: '2026-04-01T00:00:00Z',
  },
  {
    id: 'v2',
    digest: 'sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
    tags: [],
    created_at: '2026-03-28T00:00:00Z',
  },
]

const mockContent = {
  name: 'hello-workflow',
  digest: mockVersions[0].digest,
  kind: 'workflow',
  content: 'version 1.1\n\ntask hello {\n  command {\n    echo "Hello"\n  }\n}',
}

const adminMembers = [{ username: 'alice', role: 'admin', user_id: 'u1', created_at: '2026-01-01T00:00:00Z' }]
const viewerMembers = [{ username: 'alice', role: 'viewer', user_id: 'u1', created_at: '2026-01-01T00:00:00Z' }]
const executorMembers = [{ username: 'alice', role: 'executor', user_id: 'u1', created_at: '2026-01-01T00:00:00Z' }]

function setupMocks({
  username = 'alice',
  globalRole,
  versions = mockVersions,
  versionsLoading = false,
  content = mockContent,
  contentLoading = false,
  contentError = false,
  members = adminMembers,
}: {
  username?: string
  globalRole?: string
  versions?: typeof mockVersions
  versionsLoading?: boolean
  content?: typeof mockContent | null
  contentLoading?: boolean
  contentError?: boolean
  members?: typeof adminMembers
} = {}) {
  vi.mocked(useAuthStore).mockImplementation((selector) =>
    selector({ username, role: globalRole ?? 'user' } as never)
  )
  vi.mocked(useRegistryVersions).mockReturnValue({
    data: versionsLoading ? undefined : versions,
    isLoading: versionsLoading,
  } as ReturnType<typeof useRegistryVersions>)
  vi.mocked(useRegistryContent).mockReturnValue({
    data: contentLoading || contentError ? undefined : (content ?? undefined),
    isLoading: contentLoading,
    isError: contentError,
  } as ReturnType<typeof useRegistryContent>)
  vi.mocked(useProjectMembers).mockReturnValue({
    data: members,
    isLoading: false,
  } as ReturnType<typeof useProjectMembers>)
}

describe('RegistryEntryPage', () => {
  const user = userEvent.setup()

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders version table with truncated digest, tags, and date', () => {
    setupMocks()
    render(<RegistryEntryPage />)

    // Truncated digest for v1: sha256: + first 16 chars of hex + ...
    // Appears in both the version table and the source subtitle section
    const digestTexts = screen.getAllByText('sha256:abcdef1234567890...')
    expect(digestTexts.length).toBeGreaterThanOrEqual(1)
    // Tag badges for v1
    expect(screen.getByText('latest')).toBeInTheDocument()
    expect(screen.getByText('v1.0')).toBeInTheDocument()
    // v2 has no tags — em dash
    expect(screen.getByText('\u2014')).toBeInTheDocument()
    // Date for v1
    expect(screen.getByText('Apr 1, 2026')).toBeInTheDocument()
  })

  it('first version is selected by default and WDL content loads for it', async () => {
    setupMocks()
    render(<RegistryEntryPage />)

    // The first version row should have the selected highlight class
    const rows = screen.getAllByRole('row')
    // Row index 1 is the first data row (after header)
    expect(rows[1]).toHaveClass('bg-muted/50')
  })

  it('displays WDL source with line numbers', () => {
    setupMocks()
    render(<RegistryEntryPage />)

    // Content text visible
    expect(screen.getByText('version 1.1')).toBeInTheDocument()
    // Line numbers
    expect(screen.getByText('1')).toBeInTheDocument()
    expect(screen.getByText('2')).toBeInTheDocument()
  })

  it('admin sees Create Tag button; viewer does not', () => {
    // Admin
    setupMocks({ members: adminMembers })
    const { unmount } = render(<RegistryEntryPage />)
    expect(screen.getByRole('button', { name: 'Create Tag' })).toBeInTheDocument()
    unmount()

    // Viewer
    setupMocks({ members: viewerMembers })
    render(<RegistryEntryPage />)
    expect(screen.queryByRole('button', { name: 'Create Tag' })).not.toBeInTheDocument()
  })

  it('admin sees tag delete icons; viewer does not', () => {
    // Admin — v1 has tags 'latest' and 'v1.0'
    setupMocks({ members: adminMembers })
    const { unmount } = render(<RegistryEntryPage />)
    const deleteButtons = screen.getAllByRole('button', { name: /delete tag/i })
    expect(deleteButtons.length).toBeGreaterThan(0)
    unmount()

    // Viewer
    setupMocks({ members: viewerMembers })
    render(<RegistryEntryPage />)
    expect(screen.queryByRole('button', { name: /delete tag/i })).not.toBeInTheDocument()
  })

  it('renders loading skeletons for versions', () => {
    setupMocks({ versionsLoading: true })
    render(<RegistryEntryPage />)

    const skeletons = screen.getAllByTestId('skeleton-row')
    expect(skeletons).toHaveLength(3)
    expect(screen.queryByText('sha256:abcdef1234567890...')).not.toBeInTheDocument()
  })

  it('renders source loading skeleton', () => {
    setupMocks({ contentLoading: true })
    render(<RegistryEntryPage />)

    expect(screen.getByTestId('source-skeleton')).toBeInTheDocument()
  })

  it('renders error state for source', () => {
    setupMocks({ contentError: true })
    render(<RegistryEntryPage />)

    expect(screen.getByText('Failed to load source for this version.')).toBeInTheDocument()
  })

  it('opens CreateTagDialog when Create Tag button is clicked', async () => {
    setupMocks({ members: adminMembers })
    render(<RegistryEntryPage />)

    expect(screen.queryByTestId('create-tag-dialog')).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Create Tag' }))
    await waitFor(() => {
      expect(screen.getByTestId('create-tag-dialog')).toBeInTheDocument()
    })
  })

  it('opens DeleteTagDialog when delete tag button is clicked', async () => {
    setupMocks({ members: adminMembers })
    render(<RegistryEntryPage />)

    expect(screen.queryByTestId('delete-tag-dialog')).not.toBeInTheDocument()
    const deleteButtons = screen.getAllByRole('button', { name: /delete tag/i })
    await user.click(deleteButtons[0])
    await waitFor(() => {
      expect(screen.getByTestId('delete-tag-dialog')).toBeInTheDocument()
    })
  })

  it('executor sees Run button; viewer does not', () => {
    // Executor
    setupMocks({ members: executorMembers })
    const { unmount } = render(<RegistryEntryPage />)
    expect(screen.getByRole('button', { name: 'Run' })).toBeInTheDocument()
    unmount()

    // Viewer
    setupMocks({ members: viewerMembers })
    render(<RegistryEntryPage />)
    expect(screen.queryByRole('button', { name: 'Run' })).not.toBeInTheDocument()
  })

  it('admin sees Run button', () => {
    setupMocks({ members: adminMembers })
    render(<RegistryEntryPage />)
    expect(screen.getByRole('button', { name: 'Run' })).toBeInTheDocument()
  })

  it('global admin sees Run button regardless of project role', () => {
    setupMocks({ members: viewerMembers, globalRole: 'admin' })
    render(<RegistryEntryPage />)
    expect(screen.getByRole('button', { name: 'Run' })).toBeInTheDocument()
  })

  it('opens RunWorkflowDialog when Run button is clicked', async () => {
    setupMocks({ members: executorMembers })
    render(<RegistryEntryPage />)

    expect(screen.queryByTestId('run-workflow-dialog')).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Run' }))
    await waitFor(() => {
      expect(screen.getByTestId('run-workflow-dialog')).toBeInTheDocument()
    })
  })
})
