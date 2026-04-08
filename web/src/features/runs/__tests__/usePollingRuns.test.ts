import { describe, it, expect, vi, beforeEach } from 'vitest'

// Mock @tanstack/react-query
const mockUseQuery = vi.fn()
vi.mock('@tanstack/react-query', () => ({
  useQuery: (...args: unknown[]) => {
    mockUseQuery(...args)
    return { data: undefined, isLoading: true, isError: false }
  },
}))

// Mock apiClient
vi.mock('@/lib/apiClient', () => ({
  apiClient: { get: vi.fn() },
}))

import { usePollingRuns } from '@/features/runs/usePollingRuns'
import { TERMINAL_STATES } from '@/features/runs/useRuns'

describe('TERMINAL_STATES', () => {
  it('contains exactly COMPLETED, FAILED, ABORTED', () => {
    expect(TERMINAL_STATES.has('COMPLETED')).toBe(true)
    expect(TERMINAL_STATES.has('FAILED')).toBe(true)
    expect(TERMINAL_STATES.has('ABORTED')).toBe(true)
    expect(TERMINAL_STATES.size).toBe(3)
  })
})

describe('usePollingRuns', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  function getRefetchInterval(): (query: { state: { data: unknown } }) => number | false {
    usePollingRuns('test-slug')
    const options = mockUseQuery.mock.calls[0][0]
    return options.refetchInterval
  }

  it('returns 3000 when any run is RUNNING', () => {
    const fn = getRefetchInterval()
    const result = fn({
      state: { data: [{ state: 'COMPLETED' }, { state: 'RUNNING' }] },
    })
    expect(result).toBe(3000)
  })

  it('returns 3000 when any run is PENDING', () => {
    const fn = getRefetchInterval()
    const result = fn({
      state: { data: [{ state: 'PENDING' }] },
    })
    expect(result).toBe(3000)
  })

  it('returns false when all runs are COMPLETED', () => {
    const fn = getRefetchInterval()
    const result = fn({
      state: { data: [{ state: 'COMPLETED' }, { state: 'COMPLETED' }] },
    })
    expect(result).toBe(false)
  })

  it('returns false when all runs are terminal (mixed COMPLETED and FAILED)', () => {
    const fn = getRefetchInterval()
    const result = fn({
      state: { data: [{ state: 'COMPLETED' }, { state: 'FAILED' }, { state: 'ABORTED' }] },
    })
    expect(result).toBe(false)
  })

  it('returns false when data is undefined', () => {
    const fn = getRefetchInterval()
    const result = fn({ state: { data: undefined } })
    expect(result).toBe(false)
  })

  it('returns false when data is empty array', () => {
    const fn = getRefetchInterval()
    const result = fn({ state: { data: [] } })
    expect(result).toBe(false)
  })

  it('passes refetchIntervalInBackground: false', () => {
    usePollingRuns('test-slug')
    const options = mockUseQuery.mock.calls[0][0]
    expect(options.refetchIntervalInBackground).toBe(false)
  })
})
