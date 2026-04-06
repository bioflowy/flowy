import { useNavigate } from 'react-router'
import { useAuthStore } from '@/store/authStore'

export function useLogout() {
  const clearAuth = useAuthStore((s) => s.clearAuth)
  const navigate = useNavigate()
  return () => {
    clearAuth()
    navigate('/login', { replace: true })
  }
}
