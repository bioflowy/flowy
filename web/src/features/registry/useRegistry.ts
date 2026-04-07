import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiClient } from '@/lib/apiClient'

export interface RegistryEntryResponse {
  name: string
  kind: string
  version_count: number
  tags: string[]
  created_at: string
}

export interface RegistryVersionResponse {
  id: string
  digest: string
  tags: string[]
  created_at: string
}

export interface RegistryPullResponse {
  name: string
  digest: string
  kind: string
  content: string
}

export function useRegistryEntries(slug: string) {
  return useQuery<RegistryEntryResponse[]>({
    queryKey: ['projects', slug, 'registry'],
    queryFn: () => apiClient.get(`/projects/${slug}/registry`),
    enabled: !!slug,
  })
}

export function useRegistryVersions(slug: string, name: string) {
  return useQuery<RegistryVersionResponse[]>({
    queryKey: ['projects', slug, 'registry', name, 'versions'],
    queryFn: () => apiClient.get(`/projects/${slug}/registry/${name}/versions`),
    enabled: !!slug && !!name,
  })
}

export function useRegistryContent(slug: string, name: string, digest: string) {
  return useQuery<RegistryPullResponse>({
    queryKey: ['projects', slug, 'registry', name, 'content', digest],
    queryFn: () =>
      apiClient.get(`/projects/${slug}/registry/${name}?digest=${encodeURIComponent(digest)}`),
    enabled: !!slug && !!name && !!digest,
  })
}

export function useDeleteRegistryEntry(slug: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (name: string) => apiClient.delete(`/projects/${slug}/registry/${name}`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['projects', slug, 'registry'] }),
  })
}

export function useUpsertTag(slug: string, name: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ tag, digest }: { tag: string; digest: string }) =>
      apiClient.post(`/projects/${slug}/registry/${name}/tags`, { tag, digest }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', slug, 'registry'] })
      queryClient.invalidateQueries({
        queryKey: ['projects', slug, 'registry', name, 'versions'],
      })
    },
  })
}

export function useDeleteTag(slug: string, name: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (tag: string) =>
      apiClient.delete(`/projects/${slug}/registry/${name}/tags/${tag}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', slug, 'registry'] })
      queryClient.invalidateQueries({
        queryKey: ['projects', slug, 'registry', name, 'versions'],
      })
    },
  })
}
