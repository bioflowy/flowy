import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { useAuthStore } from '@/store/authStore'

// Mock apiClient
vi.mock('@/lib/apiClient', () => ({
  apiClient: {
    post: vi.fn(),
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

// Mock react-router navigate
const mockNavigate = vi.fn()
vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router')
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  }
})

import { apiClient, ApiError } from '@/lib/apiClient'
import { LoginPage } from '@/features/auth/LoginPage'

// Helper: create a valid JWT token with given claims
function makeJwt(claims: { username: string; role: string; sub?: number; exp?: number }): string {
  const header = btoa(JSON.stringify({ alg: 'HS256', typ: 'JWT' }))
  const payload = btoa(
    JSON.stringify({
      sub: claims.sub ?? 1,
      username: claims.username,
      role: claims.role,
      exp: claims.exp ?? Math.floor(Date.now() / 1000) + 3600,
    })
  )
  return `${header}.${payload}.fake-signature`
}

describe('LoginPage', () => {
  const user = userEvent.setup()

  beforeEach(() => {
    vi.clearAllMocks()
    useAuthStore.setState({ token: null, username: null, role: null })
    sessionStorage.clear()
  })

  function renderLoginPage(initialEntries = ['/login']) {
    return render(
      <MemoryRouter initialEntries={initialEntries}>
        <LoginPage />
      </MemoryRouter>
    )
  }

  it('renders username input, password input, and "Sign in" button', () => {
    renderLoginPage()

    expect(screen.getByLabelText(/username/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/password/i)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /sign in/i })).toBeInTheDocument()
  })

  it('calls apiClient.post with correct credentials on submit', async () => {
    const token = makeJwt({ username: 'admin', role: 'admin' })
    vi.mocked(apiClient.post).mockResolvedValue({ token })

    renderLoginPage()

    await user.type(screen.getByLabelText(/username/i), 'admin')
    await user.type(screen.getByLabelText(/password/i), 'pass')
    await user.click(screen.getByRole('button', { name: /sign in/i }))

    await waitFor(() => {
      expect(apiClient.post).toHaveBeenCalledWith('/auth/login', {
        username: 'admin',
        password: 'pass',
      })
    })
  })

  it('stores JWT claims and navigates away on successful login', async () => {
    const token = makeJwt({ username: 'admin', role: 'admin' })
    vi.mocked(apiClient.post).mockResolvedValue({ token })

    renderLoginPage()

    await user.type(screen.getByLabelText(/username/i), 'admin')
    await user.type(screen.getByLabelText(/password/i), 'pass')
    await user.click(screen.getByRole('button', { name: /sign in/i }))

    await waitFor(() => {
      const state = useAuthStore.getState()
      expect(state.token).toBe(token)
      expect(state.username).toBe('admin')
      expect(state.role).toBe('admin')
    })

    expect(mockNavigate).toHaveBeenCalledWith('/', { replace: true })
  })

  it('shows generic error on 401 with "invalid credentials"', async () => {
    vi.mocked(apiClient.post).mockRejectedValue(new ApiError(401, 'invalid credentials'))

    renderLoginPage()

    await user.type(screen.getByLabelText(/username/i), 'bad')
    await user.type(screen.getByLabelText(/password/i), 'creds')
    await user.click(screen.getByRole('button', { name: /sign in/i }))

    await waitFor(() => {
      expect(
        screen.getByText('Invalid username or password. Please try again.')
      ).toBeInTheDocument()
    })
  })

  it('shows same generic error on 401 with "account disabled" (security posture)', async () => {
    vi.mocked(apiClient.post).mockRejectedValue(new ApiError(401, 'account disabled'))

    renderLoginPage()

    await user.type(screen.getByLabelText(/username/i), 'disabled')
    await user.type(screen.getByLabelText(/password/i), 'user')
    await user.click(screen.getByRole('button', { name: /sign in/i }))

    await waitFor(() => {
      expect(
        screen.getByText('Invalid username or password. Please try again.')
      ).toBeInTheDocument()
    })
  })

  it('shows network error message on non-ApiError', async () => {
    vi.mocked(apiClient.post).mockRejectedValue(new TypeError('Failed to fetch'))

    renderLoginPage()

    await user.type(screen.getByLabelText(/username/i), 'user')
    await user.type(screen.getByLabelText(/password/i), 'pass')
    await user.click(screen.getByRole('button', { name: /sign in/i }))

    await waitFor(() => {
      expect(
        screen.getByText('Unable to connect. Check your connection and try again.')
      ).toBeInTheDocument()
    })
  })

  it('shows loading state while submitting', async () => {
    // Never resolve so we stay in loading state
    vi.mocked(apiClient.post).mockReturnValue(new Promise(() => {}))

    renderLoginPage()

    await user.type(screen.getByLabelText(/username/i), 'admin')
    await user.type(screen.getByLabelText(/password/i), 'pass')
    await user.click(screen.getByRole('button', { name: /sign in/i }))

    await waitFor(() => {
      expect(screen.getByText(/signing in\.\.\./i)).toBeInTheDocument()
      expect(screen.getByRole('button', { name: /signing in/i })).toBeDisabled()
    })
  })

  it('navigates to redirect path from query param after login', async () => {
    const token = makeJwt({ username: 'admin', role: 'admin' })
    vi.mocked(apiClient.post).mockResolvedValue({ token })

    renderLoginPage(['/login?redirect=/projects/foo'])

    await user.type(screen.getByLabelText(/username/i), 'admin')
    await user.type(screen.getByLabelText(/password/i), 'pass')
    await user.click(screen.getByRole('button', { name: /sign in/i }))

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/projects/foo', { replace: true })
    })
  })
})
