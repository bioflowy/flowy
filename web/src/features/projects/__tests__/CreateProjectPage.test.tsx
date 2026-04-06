import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import React from 'react'

vi.mock('@/features/projects/useProjects', () => ({
  useCreateProject: vi.fn(),
}))

const mockSetActiveProject = vi.fn()

vi.mock('@/store/projectStore', () => ({
  useProjectStore: vi.fn(),
}))

const mockNavigate = vi.fn()
vi.mock('react-router', () => ({
  useNavigate: () => mockNavigate,
}))

import { useCreateProject } from '@/features/projects/useProjects'
import { useProjectStore } from '@/store/projectStore'
import { CreateProjectPage } from '../CreateProjectPage'

describe('CreateProjectPage', () => {
  const mockMutateAsync = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(useCreateProject).mockReturnValue({
      mutateAsync: mockMutateAsync,
      isPending: false,
    } as ReturnType<typeof useCreateProject>)
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

  it('renders Name, Slug, Description fields', () => {
    render(<CreateProjectPage />)

    expect(screen.getByLabelText(/name/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/slug/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/description/i)).toBeInTheDocument()
  })

  it('auto-generates slug from name input', async () => {
    const user = userEvent.setup()
    render(<CreateProjectPage />)

    const nameInput = screen.getByLabelText(/name/i)
    await user.type(nameInput, 'My New Project')

    const slugInput = screen.getByLabelText(/slug/i) as HTMLInputElement
    expect(slugInput.value).toBe('my-new-project')
  })

  it('slug validation rejects invalid slugs', async () => {
    const user = userEvent.setup()
    render(<CreateProjectPage />)

    // Manually set the name to something so slug gets generated, then clear slug and type invalid
    const nameInput = screen.getByLabelText(/name/i)
    await user.type(nameInput, 'test')

    const slugInput = screen.getByLabelText(/slug/i)
    await user.clear(slugInput)
    await user.type(slugInput, 'INVALID SLUG!')

    const submitButton = screen.getByRole('button', { name: /^create$/i })
    await user.click(submitButton)

    await waitFor(() => {
      expect(screen.getByText(/lowercase alphanumeric/i)).toBeInTheDocument()
    })
  })
})
