import { describe, it, expect, beforeEach } from 'vitest'
import { useAuthStore } from '@/store/authStore'

describe('authStore', () => {
  beforeEach(() => {
    // Reset store state between tests
    useAuthStore.setState({
      token: null,
      username: null,
      role: null,
    })
    sessionStorage.clear()
  })

  it('setAuth stores token, username, and role', () => {
    useAuthStore.getState().setAuth('jwt-token', 'alice', 'admin')

    const state = useAuthStore.getState()
    expect(state.token).toBe('jwt-token')
    expect(state.username).toBe('alice')
    expect(state.role).toBe('admin')
  })

  it('clearAuth resets token, username, and role to null', () => {
    useAuthStore.getState().setAuth('jwt-token', 'alice', 'admin')
    useAuthStore.getState().clearAuth()

    const state = useAuthStore.getState()
    expect(state.token).toBeNull()
    expect(state.username).toBeNull()
    expect(state.role).toBeNull()
  })

  it('persists to sessionStorage under key flowy-auth', () => {
    useAuthStore.getState().setAuth('jwt-token', 'alice', 'admin')

    const stored = sessionStorage.getItem('flowy-auth')
    expect(stored).not.toBeNull()

    const parsed = JSON.parse(stored!)
    expect(parsed.state.token).toBe('jwt-token')
    expect(parsed.state.username).toBe('alice')
    expect(parsed.state.role).toBe('admin')
  })

  it('hydrates from existing sessionStorage value', () => {
    // Pre-populate sessionStorage
    sessionStorage.setItem(
      'flowy-auth',
      JSON.stringify({
        state: { token: 'restored-token', username: 'bob', role: 'viewer' },
        version: 0,
      })
    )

    // Force store to rehydrate
    useAuthStore.persist.rehydrate()

    const state = useAuthStore.getState()
    expect(state.token).toBe('restored-token')
    expect(state.username).toBe('bob')
    expect(state.role).toBe('viewer')
  })
})
