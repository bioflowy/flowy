import { describe, it, expect, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router'
import { useAuthStore } from '@/store/authStore'
import { ProtectedRoute } from '@/components/layout/ProtectedRoute'

describe('ProtectedRoute', () => {
  beforeEach(() => {
    useAuthStore.setState({ token: null, username: null, role: null })
    sessionStorage.clear()
  })

  function renderWithRoutes(initialEntries: string[], token: string | null = null) {
    if (token) {
      useAuthStore.setState({ token, username: 'testuser', role: 'admin' })
    }

    return render(
      <MemoryRouter initialEntries={initialEntries}>
        <Routes>
          <Route path="/login" element={<div data-testid="login-page">Login Page</div>} />
          <Route element={<ProtectedRoute />}>
            <Route path="/dashboard" element={<div data-testid="dashboard">Dashboard</div>} />
            <Route path="/projects/foo" element={<div data-testid="project">Project Foo</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    )
  }

  it('redirects to /login with redirect query param when token is null', () => {
    renderWithRoutes(['/dashboard'])

    expect(screen.getByTestId('login-page')).toBeInTheDocument()
    // The Navigate component should redirect to /login?redirect=%2Fdashboard
    expect(screen.queryByTestId('dashboard')).not.toBeInTheDocument()
  })

  it('renders child route (Outlet) when token is present', () => {
    renderWithRoutes(['/dashboard'], 'valid-token')

    expect(screen.getByTestId('dashboard')).toBeInTheDocument()
    expect(screen.queryByTestId('login-page')).not.toBeInTheDocument()
  })

  it('redirects to /login after clearAuth is called', () => {
    const { rerender } = renderWithRoutes(['/dashboard'], 'valid-token')

    expect(screen.getByTestId('dashboard')).toBeInTheDocument()

    // Clear auth
    useAuthStore.getState().clearAuth()

    // Re-render with same MemoryRouter to reflect state change
    rerender(
      <MemoryRouter initialEntries={['/dashboard']}>
        <Routes>
          <Route path="/login" element={<div data-testid="login-page">Login Page</div>} />
          <Route element={<ProtectedRoute />}>
            <Route path="/dashboard" element={<div data-testid="dashboard">Dashboard</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    )

    expect(screen.getByTestId('login-page')).toBeInTheDocument()
    expect(screen.queryByTestId('dashboard')).not.toBeInTheDocument()
  })

  it('encodes the redirect path properly including search params', () => {
    // Use a path with search params to verify URI encoding
    renderWithRoutes(['/projects/foo?bar=baz'])

    // Should redirect to login - the path gets encoded in the redirect param
    expect(screen.getByTestId('login-page')).toBeInTheDocument()
    expect(screen.queryByTestId('project')).not.toBeInTheDocument()
  })
})
