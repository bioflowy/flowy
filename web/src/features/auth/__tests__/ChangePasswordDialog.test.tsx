import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

// Mock apiClient
vi.mock('@/lib/apiClient', () => ({
  apiClient: {
    put: vi.fn(),
  },
  ApiError: class ApiError extends Error {
    status: number
    constructor(status: number, message: string) {
      super(message)
      this.name = 'ApiError'
      this.status = status
    }
  },
}))

// Mock sonner
vi.mock('sonner', () => ({
  toast: {
    success: vi.fn(),
  },
}))

import { apiClient, ApiError } from '@/lib/apiClient'
import { toast } from 'sonner'
import { ChangePasswordDialog } from '@/features/auth/ChangePasswordDialog'

describe('ChangePasswordDialog', () => {
  const user = userEvent.setup()
  const mockOnOpenChange = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
  })

  function renderDialog() {
    return render(
      <ChangePasswordDialog open={true} onOpenChange={mockOnOpenChange} />
    )
  }

  it('renders current password, new password, confirm new password fields and "Update password" button', () => {
    renderDialog()

    expect(screen.getByLabelText(/current password/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/^new password$/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/confirm new password/i)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /update password/i })).toBeInTheDocument()
  })

  it('shows "Passwords do not match." when new password and confirm don\'t match', async () => {
    renderDialog()

    await user.type(screen.getByLabelText(/current password/i), 'oldpass')
    await user.type(screen.getByLabelText(/^new password$/i), 'newpass123')
    await user.type(screen.getByLabelText(/confirm new password/i), 'different')
    await user.click(screen.getByRole('button', { name: /update password/i }))

    await waitFor(() => {
      expect(screen.getByText('Passwords do not match.')).toBeInTheDocument()
    })
  })

  it('on valid submit, calls apiClient.put with correct payload', async () => {
    vi.mocked(apiClient.put).mockResolvedValue(undefined)

    renderDialog()

    await user.type(screen.getByLabelText(/current password/i), 'old')
    await user.type(screen.getByLabelText(/^new password$/i), 'newpass12')
    await user.type(screen.getByLabelText(/confirm new password/i), 'newpass12')
    await user.click(screen.getByRole('button', { name: /update password/i }))

    await waitFor(() => {
      expect(apiClient.put).toHaveBeenCalledWith('/users/me/password', {
        current_password: 'old',
        new_password: 'newpass12',
      })
    })
  })

  it('on 401 error, shows "Current password is incorrect."', async () => {
    vi.mocked(apiClient.put).mockRejectedValue(
      new ApiError(401, 'current password is incorrect')
    )

    renderDialog()

    await user.type(screen.getByLabelText(/current password/i), 'wrong')
    await user.type(screen.getByLabelText(/^new password$/i), 'newpass12')
    await user.type(screen.getByLabelText(/confirm new password/i), 'newpass12')
    await user.click(screen.getByRole('button', { name: /update password/i }))

    await waitFor(() => {
      expect(screen.getByText('Current password is incorrect.')).toBeInTheDocument()
    })
  })

  it('on success, clears form fields (form.reset)', async () => {
    vi.mocked(apiClient.put).mockResolvedValue(undefined)

    renderDialog()

    await user.type(screen.getByLabelText(/current password/i), 'old')
    await user.type(screen.getByLabelText(/^new password$/i), 'newpass12')
    await user.type(screen.getByLabelText(/confirm new password/i), 'newpass12')
    await user.click(screen.getByRole('button', { name: /update password/i }))

    await waitFor(() => {
      expect(toast.success).toHaveBeenCalledWith('Password updated successfully.')
    })

    // After success, fields should be cleared
    await waitFor(() => {
      expect(screen.getByLabelText(/current password/i)).toHaveValue('')
      expect(screen.getByLabelText(/^new password$/i)).toHaveValue('')
      expect(screen.getByLabelText(/confirm new password/i)).toHaveValue('')
    })
  })

  it('while submitting, button shows "Updating..." and is disabled', async () => {
    // Never resolve to keep submitting state
    vi.mocked(apiClient.put).mockReturnValue(new Promise(() => {}))

    renderDialog()

    await user.type(screen.getByLabelText(/current password/i), 'old')
    await user.type(screen.getByLabelText(/^new password$/i), 'newpass12')
    await user.type(screen.getByLabelText(/confirm new password/i), 'newpass12')
    await user.click(screen.getByRole('button', { name: /update password/i }))

    await waitFor(() => {
      expect(screen.getByText(/updating\.\.\./i)).toBeInTheDocument()
      expect(screen.getByRole('button', { name: /updating/i })).toBeDisabled()
    })
  })
})
