import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

// Mock useUsers hooks
vi.mock('@/features/users/useUsers', () => ({
  useUsers: vi.fn(),
  useReactivateUser: vi.fn(),
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

// Mock child dialogs
vi.mock('@/features/users/CreateUserDialog', () => ({
  CreateUserDialog: ({ open }: { open: boolean }) =>
    open ? <div data-testid="create-user-dialog" /> : null,
}))

vi.mock('@/features/users/DeactivateDialog', () => ({
  DeactivateDialog: ({ open }: { open: boolean }) =>
    open ? <div data-testid="deactivate-dialog" /> : null,
}))

vi.mock('@/features/users/ResetPasswordDialog', () => ({
  ResetPasswordDialog: ({ open }: { open: boolean }) =>
    open ? <div data-testid="reset-password-dialog" /> : null,
}))

import { useUsers, useReactivateUser } from '@/features/users/useUsers'
import { useAuthStore } from '@/store/authStore'
import { UsersPage } from '@/features/users/UsersPage'

const mockUsers = [
  {
    id: 'user-1',
    username: 'alice',
    display_name: 'Alice',
    email: 'alice@example.com',
    global_role: 'admin',
    is_active: true,
    created_at: '2026-03-15T00:00:00Z',
    updated_at: '2026-03-15T00:00:00Z',
  },
  {
    id: 'user-2',
    username: 'bob',
    display_name: 'Bob',
    email: 'bob@example.com',
    global_role: 'viewer',
    is_active: false,
    created_at: '2026-03-16T00:00:00Z',
    updated_at: '2026-03-16T00:00:00Z',
  },
]

function setupMocks({ username = 'carol', users = mockUsers, isLoading = false } = {}) {
  vi.mocked(useAuthStore).mockReturnValue(username)
  vi.mocked(useUsers).mockReturnValue({
    data: isLoading ? undefined : users,
    isLoading,
  } as ReturnType<typeof useUsers>)
  vi.mocked(useReactivateUser).mockReturnValue({
    mutateAsync: vi.fn().mockResolvedValue(undefined),
  } as unknown as ReturnType<typeof useReactivateUser>)
}

describe('UsersPage', () => {
  const user = userEvent.setup()

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders table with user data (username, role badge, status badge, created date)', () => {
    setupMocks()
    render(<UsersPage />)

    // alice row
    expect(screen.getByText('alice')).toBeInTheDocument()
    expect(screen.getByText('admin')).toBeInTheDocument()
    // bob row
    expect(screen.getByText('bob')).toBeInTheDocument()
    expect(screen.getByText('viewer')).toBeInTheDocument()
    // Status badges
    expect(screen.getByText('Active')).toBeInTheDocument()
    expect(screen.getByText('Inactive')).toBeInTheDocument()
    // Created dates
    expect(screen.getByText('Mar 15, 2026')).toBeInTheDocument()
    expect(screen.getByText('Mar 16, 2026')).toBeInTheDocument()
  })

  it('shows loading skeleton rows while data is fetching', () => {
    setupMocks({ isLoading: true, users: [] })
    render(<UsersPage />)

    // Should show skeleton rows, not real data
    expect(screen.queryByText('alice')).not.toBeInTheDocument()
    // Skeleton rows should be present
    const skeletons = screen.getAllByTestId('skeleton-row')
    expect(skeletons.length).toBeGreaterThan(0)
  })

  it('shows "No users found." when API returns empty array', () => {
    setupMocks({ users: [] })
    render(<UsersPage />)

    expect(screen.getByText('No users found.')).toBeInTheDocument()
  })

  it('hides action dropdown for the row matching authStore.username (self-row)', async () => {
    // Current user is alice
    setupMocks({ username: 'alice' })
    render(<UsersPage />)

    // alice's row should NOT have the dropdown trigger
    const rows = screen.getAllByRole('row')
    // Find alice row — it should not contain a MoreHorizontal button
    // All rows with dropdown buttons
    const dropdownTriggers = screen.queryAllByRole('button', { name: /open menu/i })
    // Only bob's row should have a dropdown (not alice who is the current user)
    expect(dropdownTriggers).toHaveLength(1)
  })

  it('shows "Deactivate" and "Reset Password" in dropdown for active users', async () => {
    // Current user is carol (not in list), so both alice and bob have dropdowns
    setupMocks({ username: 'carol' })
    render(<UsersPage />)

    // Open alice's dropdown (active user)
    const dropdownTriggers = screen.getAllByRole('button', { name: /open menu/i })
    await user.click(dropdownTriggers[0])

    await waitFor(() => {
      expect(screen.getByText('Deactivate')).toBeInTheDocument()
      expect(screen.getByText('Reset Password')).toBeInTheDocument()
    })
  })

  it('shows "Reactivate" and "Reset Password" in dropdown for inactive users', async () => {
    setupMocks({ username: 'carol' })
    render(<UsersPage />)

    // Open bob's dropdown (inactive user - second trigger)
    const dropdownTriggers = screen.getAllByRole('button', { name: /open menu/i })
    await user.click(dropdownTriggers[1])

    await waitFor(() => {
      expect(screen.getByText('Reactivate')).toBeInTheDocument()
      expect(screen.getByText('Reset Password')).toBeInTheDocument()
    })
  })
})
