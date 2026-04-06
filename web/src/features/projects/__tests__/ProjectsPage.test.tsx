import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import React from 'react'

vi.mock('@/features/projects/useProjects', () => ({
  useProjects: vi.fn(),
}))

vi.mock('@/store/authStore', () => ({
  useAuthStore: vi.fn(),
}))

const mockSetActiveProject = vi.fn()

vi.mock('@/store/projectStore', () => ({
  useProjectStore: vi.fn(),
}))

const mockNavigate = vi.fn()
vi.mock('react-router', () => ({
  useNavigate: () => mockNavigate,
}))

import { useProjects } from '@/features/projects/useProjects'
import { useAuthStore } from '@/store/authStore'
import { useProjectStore } from '@/store/projectStore'
import { ProjectsPage } from '../ProjectsPage'

const mockProjects = [
  {
    id: 'proj-1',
    slug: 'alpha-project',
    description: 'An alpha project description',
    is_active: true,
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z',
  },
  {
    id: 'proj-2',
    slug: 'beta-project',
    description: 'A beta project description',
    is_active: true,
    created_at: '2024-01-02T00:00:00Z',
    updated_at: '2024-01-02T00:00:00Z',
  },
]

describe('ProjectsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    // useProjectStore is called with a selector — implement the selector pattern
    vi.mocked(useProjectStore).mockImplementation(
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      (selector: any) => {
        const state = {
          activeProjectSlug: null,
          setActiveProject: mockSetActiveProject,
        }
        return typeof selector === 'function' ? selector(state) : state
      }
    )
  })

  it('renders "Projects" heading', () => {
    vi.mocked(useProjects).mockReturnValue({ isLoading: false, data: [] } as ReturnType<typeof useProjects>)
    vi.mocked(useAuthStore).mockReturnValue('admin' as ReturnType<typeof useAuthStore>)

    render(<ProjectsPage />)

    expect(screen.getByText('Projects')).toBeInTheDocument()
  })

  it('renders skeleton cards when loading', () => {
    vi.mocked(useProjects).mockReturnValue({ isLoading: true, data: undefined } as ReturnType<typeof useProjects>)
    vi.mocked(useAuthStore).mockReturnValue('admin' as ReturnType<typeof useAuthStore>)

    render(<ProjectsPage />)

    const skeletons = document.querySelectorAll('.animate-pulse')
    expect(skeletons.length).toBeGreaterThanOrEqual(3)
  })

  it('renders project cards with slug and description', () => {
    vi.mocked(useProjects).mockReturnValue({ isLoading: false, data: mockProjects } as ReturnType<typeof useProjects>)
    vi.mocked(useAuthStore).mockReturnValue('admin' as ReturnType<typeof useAuthStore>)

    render(<ProjectsPage />)

    expect(screen.getByText('alpha-project')).toBeInTheDocument()
    expect(screen.getByText('An alpha project description')).toBeInTheDocument()
    expect(screen.getByText('beta-project')).toBeInTheDocument()
    expect(screen.getByText('A beta project description')).toBeInTheDocument()
  })

  it('shows empty state when no projects', () => {
    vi.mocked(useProjects).mockReturnValue({ isLoading: false, data: [] } as ReturnType<typeof useProjects>)
    vi.mocked(useAuthStore).mockReturnValue('viewer' as ReturnType<typeof useAuthStore>)

    render(<ProjectsPage />)

    expect(screen.getByText('No projects yet.')).toBeInTheDocument()
  })

  it('shows "Create Project" button when admin, hides when non-admin', () => {
    vi.mocked(useProjects).mockReturnValue({ isLoading: false, data: mockProjects } as ReturnType<typeof useProjects>)
    vi.mocked(useAuthStore).mockReturnValue('admin' as ReturnType<typeof useAuthStore>)

    const { rerender } = render(<ProjectsPage />)

    // Admin sees button
    expect(screen.getByText('Create Project')).toBeInTheDocument()

    // Non-admin does not
    vi.mocked(useAuthStore).mockReturnValue('viewer' as ReturnType<typeof useAuthStore>)
    rerender(<ProjectsPage />)
    expect(screen.queryByText('Create Project')).not.toBeInTheDocument()
  })

  it('calls setActiveProject and navigates on card click', async () => {
    const user = userEvent.setup()
    vi.mocked(useProjects).mockReturnValue({ isLoading: false, data: mockProjects } as ReturnType<typeof useProjects>)
    vi.mocked(useAuthStore).mockReturnValue('viewer' as ReturnType<typeof useAuthStore>)

    render(<ProjectsPage />)

    const card = screen.getByText('alpha-project').closest('[role="button"]')
    expect(card).toBeTruthy()
    await user.click(card!)

    expect(mockSetActiveProject).toHaveBeenCalledWith('alpha-project')
    expect(mockNavigate).toHaveBeenCalledWith('/projects/alpha-project')
  })
})
