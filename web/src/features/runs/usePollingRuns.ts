import { useQuery } from '@tanstack/react-query'
import { apiClient } from '@/lib/apiClient'
import type { RunListItem } from '@/features/runs/useRuns'
import { TERMINAL_STATES } from '@/features/runs/useRuns'

export function usePollingRuns(slug: string) {
  return useQuery<RunListItem[]>({
    queryKey: ['projects', slug, 'runs'],
    queryFn: () => apiClient.get(`/projects/${slug}/runs`),
    enabled: !!slug,
    refetchInterval: (query) => {
      const data = query.state.data
      if (!data || data.length === 0) return false
      return data.some((r) => !TERMINAL_STATES.has(r.state)) ? 3000 : false
    },
    refetchIntervalInBackground: false,
  })
}
