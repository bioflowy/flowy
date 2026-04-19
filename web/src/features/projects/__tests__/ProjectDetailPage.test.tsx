import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import React from 'react'

vi.mock('@/features/projects/useProjects', () => ({
  useProjectDetail: vi.fn(),
  useProjectMembers: vi.fn(),
  useUpdateProject: vi.fn(),
  useDeleteProject: vi.fn(),
  useUpdateMember: vi.fn(),
  useRemoveMember: vi.fn(),
  useManagedFiles: vi.fn(),
  useDeleteManagedFile: vi.fn(),
}))

vi.mock('@/store/authStore', () => ({
  useAuthStore: vi.fn(),
}))

vi.mock('@/store/projectStore', () => ({
  useProjectStore: vi.fn(),
}))

const mockNavigate = vi.fn()
vi.mock('react-router', () => ({
  useParams: () => ({ slug: 'test-project' }),
  useNavigate: () => mockNavigate,
}))

// Stub AddMemberDialog to avoid its own dependency chain
vi.mock('@/features/projects/AddMemberDialog', () => ({
  AddMemberDialog: ({ open }: { open: boolean }) =>
    open ? <div data-testid="add-member-dialog">AddMemberDialog</div> : null,
}))

vi.mock('@/features/projects/ImportFileDialog', () => ({
  ImportFileDialog: ({ open }: { open: boolean }) =>
    open ? <div data-testid="import-file-dialog">ImportFileDialog</div> : null,
}))

import {
  useProjectDetail,
  useProjectMembers,
  useUpdateProject,
  useDeleteProject,
  useUpdateMember,
  useRemoveMember,
  useManagedFiles,
  useDeleteManagedFile,
} from '@/features/projects/useProjects'
import { useAuthStore } from '@/store/authStore'
import { useProjectStore } from '@/store/projectStore'
import { ProjectDetailPage } from '../ProjectDetailPage'

const mockProject = {
  id: 'proj-1',
  slug: 'test-project',
  description: 'A test project description',
  is_active: true,
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
}

const mockMembers = [
  {
    user_id: 'user-1',
    username: 'alice',
    role: 'admin',
    created_at: '2024-03-15T00:00:00Z',
    updated_at: '2024-03-15T00:00:00Z',
  },
  {
    user_id: 'user-2',
    username: 'bob',
    role: 'executor',
    created_at: '2024-03-16T00:00:00Z',
    updated_at: '2024-03-16T00:00:00Z',
  },
]

const mockFiles = [
  {
    id: 'file-1',
    path: '/data/reads.fastq.gz',
    file_type: 'fastq',
    label: 'reads.fastq.gz',
    size: 2048,
    created_at: '2024-03-20T00:00:00Z',
  },
]

describe('ProjectDetailPage', () => {
  const user = userEvent.setup()

  beforeEach(() => {
    vi.clearAllMocks()

    vi.mocked(useProjectDetail).mockReturnValue({
      data: mockProject,
      isLoading: false,
    } as ReturnType<typeof useProjectDetail>)

    vi.mocked(useProjectMembers).mockReturnValue({
      data: mockMembers,
      isLoading: false,
    } as ReturnType<typeof useProjectMembers>)

    vi.mocked(useUpdateProject).mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
    } as unknown as ReturnType<typeof useUpdateProject>)

    vi.mocked(useDeleteProject).mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
    } as unknown as ReturnType<typeof useDeleteProject>)

    vi.mocked(useUpdateMember).mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
    } as unknown as ReturnType<typeof useUpdateMember>)

    vi.mocked(useRemoveMember).mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
    } as unknown as ReturnType<typeof useRemoveMember>)

    vi.mocked(useManagedFiles).mockReturnValue({
      data: mockFiles,
      isLoading: false,
    } as unknown as ReturnType<typeof useManagedFiles>)

    vi.mocked(useDeleteManagedFile).mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
    } as unknown as ReturnType<typeof useDeleteManagedFile>)

    vi.mocked(useProjectStore).mockImplementation(
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      (selector: any) => {
        const state = {
          activeProjectSlug: 'test-project',
          setActiveProject: vi.fn(),
        }
        return typeof selector === 'function' ? selector(state) : state
      }
    )
  })

  it('renders project slug heading', () => {
    vi.mocked(useAuthStore).mockReturnValue('alice' as ReturnType<typeof useAuthStore>)

    render(<ProjectDetailPage />)

    expect(screen.getByRole('heading', { name: 'test-project' })).toBeInTheDocument()
  })

  it('renders description', () => {
    vi.mocked(useAuthStore).mockReturnValue('alice' as ReturnType<typeof useAuthStore>)

    render(<ProjectDetailPage />)

    expect(screen.getByText('A test project description')).toBeInTheDocument()
  })

  it('shows "Edit Description" and "Delete Project" when user is project admin', () => {
    // alice is admin in mockMembers
    vi.mocked(useAuthStore).mockReturnValue('alice' as ReturnType<typeof useAuthStore>)

    render(<ProjectDetailPage />)

    expect(screen.getByText('Edit Description')).toBeInTheDocument()
    expect(screen.getByText('Delete Project')).toBeInTheDocument()
  })

  it('hides "Edit Description" and "Delete Project" when user is NOT project admin', () => {
    // bob is executor, not admin
    vi.mocked(useAuthStore).mockReturnValue('bob' as ReturnType<typeof useAuthStore>)

    render(<ProjectDetailPage />)

    expect(screen.queryByText('Edit Description')).not.toBeInTheDocument()
    expect(screen.queryByText('Delete Project')).not.toBeInTheDocument()
  })

  it('renders member table with usernames and roles', () => {
    vi.mocked(useAuthStore).mockReturnValue('alice' as ReturnType<typeof useAuthStore>)

    render(<ProjectDetailPage />)

    expect(screen.getByText('alice')).toBeInTheDocument()
    expect(screen.getByText('bob')).toBeInTheDocument()
  })

  it('does not render three-dot menu for current user (self-protection)', () => {
    // alice is current user — alice's row should have no three-dot action menu
    vi.mocked(useAuthStore).mockReturnValue('alice' as ReturnType<typeof useAuthStore>)

    render(<ProjectDetailPage />)

    // There should be only one action menu button (for bob), not two
    const menuButtons = screen.queryAllByRole('button', { name: 'Open menu' })
    // alice (self) should have no three-dot menu, bob should
    expect(menuButtons).toHaveLength(1)
  })

  it('shows "Add Member" button for project admin', () => {
    vi.mocked(useAuthStore).mockReturnValue('alice' as ReturnType<typeof useAuthStore>)

    render(<ProjectDetailPage />)

    expect(screen.getByText('Add Member')).toBeInTheDocument()
  })

  it('shows loading skeletons during data fetch', () => {
    vi.mocked(useAuthStore).mockReturnValue('alice' as ReturnType<typeof useAuthStore>)
    vi.mocked(useProjectMembers).mockReturnValue({
      data: undefined,
      isLoading: true,
    } as ReturnType<typeof useProjectMembers>)

    render(<ProjectDetailPage />)

    const skeletons = document.querySelectorAll('[data-testid="skeleton-row"]')
    expect(skeletons.length).toBeGreaterThanOrEqual(3)
  })

  it('renders managed files when Files tab is selected', async () => {
    vi.mocked(useAuthStore).mockReturnValue('alice' as ReturnType<typeof useAuthStore>)

    render(<ProjectDetailPage />)

    await user.click(screen.getByRole('button', { name: 'Files' }))

    expect(screen.getByText('/data/reads.fastq.gz')).toBeInTheDocument()
    expect(screen.getByText('fastq')).toBeInTheDocument()
    expect(screen.getByText('2.0 KB')).toBeInTheDocument()
    expect(screen.getByText('Import File')).toBeInTheDocument()
  })
})
