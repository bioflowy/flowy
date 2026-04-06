import { describe, it, expect, beforeEach, vi } from 'vitest'
import { renderHook, act } from '@testing-library/react'

// Mock react-router navigate
const mockNavigate = vi.fn()
vi.mock('react-router', () => ({
  useNavigate: () => mockNavigate,
}))

// Mock authStore
const mockClearAuth = vi.fn()
vi.mock('@/store/authStore', () => ({
  useAuthStore: (selector: (s: Record<string, unknown>) => unknown) =>
    selector({ clearAuth: mockClearAuth }),
}))

import { useLogout } from '@/features/auth/useLogout'

describe('useLogout', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('calls clearAuth() on the auth store', () => {
    const { result } = renderHook(() => useLogout())
    act(() => {
      result.current()
    })
    expect(mockClearAuth).toHaveBeenCalledOnce()
  })

  it('navigates to /login with replace: true', () => {
    const { result } = renderHook(() => useLogout())
    act(() => {
      result.current()
    })
    expect(mockNavigate).toHaveBeenCalledWith('/login', { replace: true })
  })
})
