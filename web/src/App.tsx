import { BrowserRouter, Routes, Route, Navigate } from 'react-router'
import {
  QueryClient,
  QueryClientProvider,
  QueryCache,
  MutationCache,
} from '@tanstack/react-query'
import { Toaster } from 'sonner'
import { ApiError } from '@/lib/apiClient'
import { useAuthStore } from '@/store/authStore'
import { AppShell } from '@/components/layout/AppShell'
import { ProtectedRoute } from '@/components/layout/ProtectedRoute'
import { LoginPage } from '@/features/auth/LoginPage'
import { AccountPage } from '@/features/auth/AccountPage'
import { UsersPage } from '@/features/users/UsersPage'

const handleAuthError = (error: unknown) => {
  if (error instanceof ApiError && error.status === 401) {
    useAuthStore.getState().clearAuth()
  }
}

const queryClient = new QueryClient({
  queryCache: new QueryCache({ onError: handleAuthError }),
  mutationCache: new MutationCache({ onError: handleAuthError }),
  defaultOptions: {
    queries: {
      retry: (failureCount, error) => {
        if (error instanceof ApiError && (error.status === 401 || error.status === 403)) return false
        return failureCount < 2
      },
    },
  },
})

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route element={<ProtectedRoute />}>
            <Route element={<AppShell />}>
              <Route index element={<Navigate to="/projects" replace />} />
              <Route path="account" element={<AccountPage />} />
              <Route path="users" element={<UsersPage />} />
              <Route path="*" element={<div>Select a page from the sidebar.</div>} />
            </Route>
          </Route>
        </Routes>
      </BrowserRouter>
      <Toaster />
    </QueryClientProvider>
  )
}
