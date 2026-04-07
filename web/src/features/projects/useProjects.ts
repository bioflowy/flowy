import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiClient } from '@/lib/apiClient'
import type { UserResponse } from '@/features/users/useUsers'

export interface ProjectResponse {
  id: string
  slug: string
  description: string
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface MemberResponse {
  user_id: string
  username: string
  role: string
  created_at: string
  updated_at: string
}

export function useProjects() {
  return useQuery<ProjectResponse[]>({
    queryKey: ['projects'],
    queryFn: () => apiClient.get('/projects'),
  })
}

export function useProjectDetail(slug: string) {
  return useQuery<ProjectResponse>({
    queryKey: ['projects', slug],
    queryFn: () => apiClient.get(`/projects/${slug}`),
    enabled: !!slug,
  })
}

export function useProjectMembers(slug: string) {
  return useQuery<MemberResponse[]>({
    queryKey: ['projects', slug, 'members'],
    queryFn: () => apiClient.get(`/projects/${slug}/members`),
    enabled: !!slug,
  })
}

export function useCreateProject() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (data: { slug: string; description: string }) =>
      apiClient.post('/projects', data),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['projects'] }),
  })
}

export function useUpdateProject(slug: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (data: { description: string }) =>
      apiClient.put(`/projects/${slug}`, data),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['projects', slug] }),
  })
}

export function useDeleteProject(slug: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => apiClient.delete(`/projects/${slug}`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['projects'] }),
  })
}

export function useAddMember(slug: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (data: { username: string; role: string }) =>
      apiClient.post(`/projects/${slug}/members`, data),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['projects', slug, 'members'] }),
  })
}

export function useUpdateMember(slug: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ username, role }: { username: string; role: string }) =>
      apiClient.put(`/projects/${slug}/members/${username}`, { role }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['projects', slug, 'members'] }),
  })
}

export function useRemoveMember(slug: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (username: string) =>
      apiClient.delete(`/projects/${slug}/members/${username}`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['projects', slug, 'members'] }),
  })
}

export function useProjectUsers(slug: string) {
  return useQuery<UserResponse[]>({
    queryKey: ['projects', slug, 'users'],
    queryFn: () => apiClient.get(`/projects/${slug}/users`),
    enabled: !!slug,
  })
}
