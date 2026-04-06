import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiClient } from '@/lib/apiClient'

export interface UserResponse {
  id: string
  username: string
  display_name: string
  email: string
  global_role: string
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface CreateUserRequest {
  username: string
  password: string
  global_role: string
  display_name?: string
  email?: string
}

export function useUsers() {
  return useQuery<UserResponse[]>({
    queryKey: ['users'],
    queryFn: () => apiClient.get('/users'),
  })
}

export function useCreateUser() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (data: CreateUserRequest) => apiClient.post('/users', data),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['users'] }),
  })
}

export function useDeactivateUser() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (userId: string) => apiClient.delete(`/users/${userId}`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['users'] }),
  })
}

export function useReactivateUser() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (userId: string) => apiClient.patch(`/users/${userId}/reactivate`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['users'] }),
  })
}

export function useResetPassword() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ userId, newPassword }: { userId: string; newPassword: string }) =>
      apiClient.put(`/users/${userId}/password`, { new_password: newPassword }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['users'] }),
  })
}
