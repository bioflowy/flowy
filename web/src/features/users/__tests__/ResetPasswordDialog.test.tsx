import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'

// Mock useResetPassword
vi.mock('@/features/users/useUsers', () => ({
  useResetPassword: vi.fn(),
}))

// Mock toast
vi.mock('sonner', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

import { useResetPassword } from '@/features/users/useUsers'
import { toast } from 'sonner'
import { ResetPasswordDialog } from '@/features/users/ResetPasswordDialog'

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
}

const testUser = { id: 'user-1', username: 'testuser' }

function renderDialog(user = testUser, onOpenChange = vi.fn()) {
  const Wrapper = createWrapper()
  return render(
    <Wrapper>
      <ResetPasswordDialog open={true} onOpenChange={onOpenChange} user={user} />
    </Wrapper>
  )
}

describe('ResetPasswordDialog', () => {
  const user = userEvent.setup()

  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(useResetPassword).mockReturnValue({
      mutateAsync: vi.fn().mockResolvedValue(undefined),
      isPending: false,
    } as unknown as ReturnType<typeof useResetPassword>)
  })

  it('renders description "Resetting password for testuser"', () => {
    renderDialog()

    expect(screen.getByText('Resetting password for testuser')).toBeInTheDocument()
  })

  it('shows "Passwords do not match." when passwords differ', async () => {
    renderDialog()

    await user.type(screen.getByLabelText(/^new password$/i), 'password123')
    await user.type(screen.getByLabelText(/^confirm password$/i), 'different1')
    await user.click(screen.getByRole('button', { name: /reset password/i }))

    await waitFor(() => {
      expect(screen.getByText('Passwords do not match.')).toBeInTheDocument()
    })
  })

  it('shows "Password must be at least 8 characters." for short password', async () => {
    renderDialog()

    await user.type(screen.getByLabelText(/^new password$/i), 'short')
    await user.type(screen.getByLabelText(/^confirm password$/i), 'short')
    await user.click(screen.getByRole('button', { name: /reset password/i }))

    await waitFor(() => {
      expect(screen.getByText('Password must be at least 8 characters.')).toBeInTheDocument()
    })
  })

  it('calls mutateAsync with correct payload, shows "Password reset for testuser." toast', async () => {
    const mockMutateAsync = vi.fn().mockResolvedValue(undefined)
    vi.mocked(useResetPassword).mockReturnValue({
      mutateAsync: mockMutateAsync,
      isPending: false,
    } as unknown as ReturnType<typeof useResetPassword>)

    renderDialog()

    await user.type(screen.getByLabelText(/^new password$/i), 'newpassword123')
    await user.type(screen.getByLabelText(/^confirm password$/i), 'newpassword123')
    await user.click(screen.getByRole('button', { name: /reset password/i }))

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith({
        userId: 'user-1',
        newPassword: 'newpassword123',
      })
      expect(toast.success).toHaveBeenCalledWith('Password reset for testuser.')
    })
  })
})
