import { useQuery } from '@tanstack/react-query'
import { apiClient } from '@/lib/apiClient'

export interface RunListItem {
  id: string
  state: string
  registry_ref?: string
  wdl_path?: string
  created_at: string
  updated_at: string
}

export interface OutputsResponse {
  run_id: string
  outputs: Record<string, unknown>
}

export const TERMINAL_STATES = new Set(['COMPLETED', 'FAILED', 'ABORTED'])

export function useRuns(slug: string) {
  return useQuery<RunListItem[]>({
    queryKey: ['projects', slug, 'runs'],
    queryFn: () => apiClient.get(`/projects/${slug}/runs`),
    enabled: !!slug,
  })
}

export function useRunOutputs(slug: string, id: string) {
  return useQuery<OutputsResponse>({
    queryKey: ['projects', slug, 'runs', id, 'outputs'],
    queryFn: () => apiClient.get(`/projects/${slug}/runs/${id}/outputs`),
    enabled: !!slug && !!id,
  })
}
