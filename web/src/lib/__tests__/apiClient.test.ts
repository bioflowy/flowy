import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { useAuthStore } from '@/store/authStore'

// Must import after store setup
let apiClient: typeof import('@/lib/apiClient').apiClient
let ApiError: typeof import('@/lib/apiClient').ApiError

describe('apiClient', () => {
  const mockFetch = vi.fn()

  beforeEach(async () => {
    vi.stubGlobal('fetch', mockFetch)
    mockFetch.mockReset()
    useAuthStore.setState({ token: null, username: null, role: null })

    // Dynamic import to get fresh module
    const mod = await import('@/lib/apiClient')
    apiClient = mod.apiClient
    ApiError = mod.ApiError
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('sends GET request to /api{path} with Content-Type header', async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ data: 'test' }),
    })

    await apiClient.get('/users/me')

    expect(mockFetch).toHaveBeenCalledWith('/api/users/me', {
      method: 'GET',
      headers: expect.objectContaining({
        'Content-Type': 'application/json',
      }),
    })
  })

  it('includes Authorization header when token exists', async () => {
    useAuthStore.setState({ token: 'abc123', username: 'alice', role: 'admin' })

    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve({}),
    })

    await apiClient.get('/users/me')

    expect(mockFetch).toHaveBeenCalledWith('/api/users/me', {
      method: 'GET',
      headers: expect.objectContaining({
        Authorization: 'Bearer abc123',
      }),
    })
  })

  it('does NOT include Authorization header when no token', async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve({}),
    })

    await apiClient.get('/users/me')

    const headers = mockFetch.mock.calls[0][1].headers
    expect(headers).not.toHaveProperty('Authorization')
  })

  it('sends POST with JSON-stringified body', async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ token: 'new-token' }),
    })

    await apiClient.post('/auth/login', {
      username: 'alice',
      password: 'secret',
    })

    expect(mockFetch).toHaveBeenCalledWith('/api/auth/login', {
      method: 'POST',
      headers: expect.objectContaining({
        'Content-Type': 'application/json',
      }),
      body: JSON.stringify({ username: 'alice', password: 'secret' }),
    })
  })

  it('throws ApiError with status and message from response .error field on non-ok response', async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 401,
      statusText: 'Unauthorized',
      json: () => Promise.resolve({ error: 'invalid credentials' }),
    })

    await expect(apiClient.post('/auth/login', {})).rejects.toThrow(
      expect.objectContaining({
        name: 'ApiError',
        status: 401,
        message: 'invalid credentials',
      })
    )
  })

  it('throws ApiError with statusText when response body is not JSON', async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 500,
      statusText: 'Internal Server Error',
      json: () => Promise.reject(new Error('not json')),
    })

    await expect(apiClient.get('/broken')).rejects.toThrow(
      expect.objectContaining({
        name: 'ApiError',
        status: 500,
        message: 'Internal Server Error',
      })
    )
  })
})
