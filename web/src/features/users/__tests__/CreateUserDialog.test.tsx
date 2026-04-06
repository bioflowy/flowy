import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'

// Mock useCreateUser
vi.mock('@/features/users/useUsers', () => ({
  useCreateUser: vi.fn(),
}))

// Mock toast
vi.mock('sonner', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

import { useCreateUser } from '@/features/users/useUsers'
import { toast } from 'sonner'
import { CreateUserDialog } from '@/features/users/CreateUserDialog'

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

function renderDialog(onOpenChange = vi.fn()) {
  const Wrapper = createWrapper()
  return render(
    <Wrapper>
      <CreateUserDialog open={true} onOpenChange={onOpenChange} />
    </Wrapper>
  )
}

describe('CreateUserDialog', () => {
  const user = userEvent.setup()

  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(useCreateUser).mockReturnValue({
      mutateAsync: vi.fn().mockResolvedValue({}),
      isPending: false,
    } as unknown as ReturnType<typeof useCreateUser>)
  })

  it('renders form with Username, Password, Role (defaulting to viewer), Display Name, Email fields', () => {
    renderDialog()

    expect(screen.getByLabelText(/^username$/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/^password$/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/^role$/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/^display name$/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/^email$/i)).toBeInTheDocument()
    // Role select should default to viewer (there may be multiple "viewer" text nodes due to select rendering)
    expect(screen.getAllByText('viewer').length).toBeGreaterThan(0)
  })

  it('shows validation error for password less than 8 characters', async () => {
    renderDialog()

    await user.type(screen.getByLabelText(/^username$/i), 'testuser')
    await user.type(screen.getByLabelText(/^password$/i), 'abc')
    await user.click(screen.getByRole('button', { name: /^create$/i }))

    await waitFor(() => {
      expect(screen.getByText('Password must be at least 8 characters')).toBeInTheDocument()
    })
  })

  it('calls mutateAsync with correct payload on valid submit, shows "User created." toast', async () => {
    const mockMutateAsync = vi.fn().mockResolvedValue({})
    vi.mocked(useCreateUser).mockReturnValue({
      mutateAsync: mockMutateAsync,
      isPending: false,
    } as unknown as ReturnType<typeof useCreateUser>)

    renderDialog()

    await user.type(screen.getByLabelText(/^username$/i), 'newuser')
    await user.type(screen.getByLabelText(/^password$/i), 'password123')
    await user.click(screen.getByRole('button', { name: /^create$/i }))

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith({
        username: 'newuser',
        password: 'password123',
        global_role: 'viewer',
        display_name: '',
        email: '',
      })
      expect(toast.success).toHaveBeenCalledWith('User created.')
    })
  })

  it('shows "Creating..." with spinner during submission', async () => {
    // Mock isPending: true so the button immediately shows "Creating..."
    vi.mocked(useCreateUser).mockReturnValue({
      mutateAsync: vi.fn().mockReturnValue(new Promise(() => {})),
      isPending: true,
    } as unknown as ReturnType<typeof useCreateUser>)

    renderDialog()

    // When isPending is true, the button should show "Creating..."
    await waitFor(() => {
      expect(screen.getByText(/creating\.\.\./i)).toBeInTheDocument()
    })
  })
})
