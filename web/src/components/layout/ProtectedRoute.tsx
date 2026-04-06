import { Navigate, Outlet, useLocation } from 'react-router'
import { useAuthStore } from '@/store/authStore'

export function ProtectedRoute() {
  const token = useAuthStore((s) => s.token)
  const location = useLocation()

  if (!token) {
    const redirectParam = encodeURIComponent(location.pathname + location.search)
    return <Navigate to={`/login?redirect=${redirectParam}`} replace />
  }

  return <Outlet />
}
