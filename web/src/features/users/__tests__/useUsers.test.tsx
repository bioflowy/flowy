import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'

vi.mock('@/lib/apiClient', () => ({
  apiClient: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
    patch: vi.fn(),
  },
}))

import { apiClient } from '@/lib/apiClient'
import {
  useUsers,
  useCreateUser,
  useDeactivateUser,
  useReactivateUser,
  useResetPassword,
} from '../useUsers'

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

const mockUsers = [
  {
    id: 'user-1',
    username: 'alice',
    display_name: 'Alice',
    email: 'alice@example.com',
    global_role: 'user',
    is_active: true,
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z',
  },
]

describe('useUsers', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('fetches users from GET /users', async () => {
    vi.mocked(apiClient.get).mockResolvedValue(mockUsers)

    const { result } = renderHook(() => useUsers(), { wrapper: createWrapper() })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(apiClient.get).toHaveBeenCalledWith('/users')
    expect(result.current.data).toEqual(mockUsers)
  })
})

describe('useCreateUser', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('posts to /users and invalidates users query', async () => {
    const newUser = {
      id: 'user-2',
      username: 'bob',
      display_name: '',
      email: '',
      global_role: 'user',
      is_active: true,
      created_at: '2024-01-01T00:00:00Z',
      updated_at: '2024-01-01T00:00:00Z',
    }
    vi.mocked(apiClient.post).mockResolvedValue(newUser)

    const { result } = renderHook(() => useCreateUser(), { wrapper: createWrapper() })

    result.current.mutate({
      username: 'bob',
      password: 'password123',
      global_role: 'user',
    })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(apiClient.post).toHaveBeenCalledWith('/users', {
      username: 'bob',
      password: 'password123',
      global_role: 'user',
    })
  })
})

describe('useDeactivateUser', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('calls DELETE /users/{id} and invalidates users query', async () => {
    vi.mocked(apiClient.delete).mockResolvedValue(undefined)

    const { result } = renderHook(() => useDeactivateUser(), { wrapper: createWrapper() })

    result.current.mutate('user-1')

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(apiClient.delete).toHaveBeenCalledWith('/users/user-1')
  })
})

describe('useReactivateUser', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('calls PATCH /users/{id}/reactivate and invalidates users query', async () => {
    vi.mocked(apiClient.patch).mockResolvedValue(mockUsers[0])

    const { result } = renderHook(() => useReactivateUser(), { wrapper: createWrapper() })

    result.current.mutate('user-1')

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(apiClient.patch).toHaveBeenCalledWith('/users/user-1/reactivate')
  })
})

describe('useResetPassword', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('calls PUT /users/{id}/password with new_password and invalidates users query', async () => {
    vi.mocked(apiClient.put).mockResolvedValue(undefined)

    const { result } = renderHook(() => useResetPassword(), { wrapper: createWrapper() })

    result.current.mutate({ userId: 'user-1', newPassword: 'newpassword123' })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(apiClient.put).toHaveBeenCalledWith('/users/user-1/password', {
      new_password: 'newpassword123',
    })
  })
})
