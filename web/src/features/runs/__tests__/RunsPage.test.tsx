import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'

// Mock useRuns hook (kept for RunListItem type usage)
vi.mock('@/features/runs/useRuns', () => ({
  useRuns: vi.fn(),
  TERMINAL_STATES: new Set(['COMPLETED', 'FAILED', 'ABORTED']),
}))

// Mock usePollingRuns hook
vi.mock('@/features/runs/usePollingRuns', () => ({
  usePollingRuns: vi.fn(),
}))

// Mock react-router
const mockNavigate = vi.fn()
vi.mock('react-router', () => ({
  useParams: vi.fn(() => ({ slug: 'test-project' })),
  useNavigate: vi.fn(() => mockNavigate),
}))

import { usePollingRuns } from '@/features/runs/usePollingRuns'
import { RunsPage } from '@/features/runs/RunsPage'

const mockRuns = [
  {
    id: 'abcdef12-3456-7890-abcd-ef1234567890',
    state: 'COMPLETED',
    registry_ref: 'my-workflow:latest',
    wdl_path: undefined,
    created_at: '2026-04-01T10:00:00Z',
    updated_at: '2026-04-01T10:05:00Z',
  },
  {
    id: 'fedcba09-8765-4321-fedc-ba0987654321',
    state: 'FAILED',
    registry_ref: undefined,
    wdl_path: '/path/to/workflow.wdl',
    created_at: '2026-04-02T12:00:00Z',
    updated_at: '2026-04-02T12:10:00Z',
  },
]

const mockRunWithRunning = [
  {
    id: 'abcdef12-3456-7890-abcd-ef1234567890',
    state: 'RUNNING',
    registry_ref: 'my-workflow:latest',
    wdl_path: undefined,
    created_at: '2026-04-01T10:00:00Z',
    updated_at: '2026-04-01T10:05:00Z',
  },
]

function setupMocks({
  runs = mockRuns,
  isLoading = false,
  isError = false,
}: {
  runs?: typeof mockRuns
  isLoading?: boolean
  isError?: boolean
} = {}) {
  vi.mocked(usePollingRuns).mockReturnValue({
    data: isLoading ? undefined : runs,
    isLoading,
    isError,
  } as ReturnType<typeof usePollingRuns>)
}

describe('RunsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockNavigate.mockReset()
  })

  it('renders loading skeleton rows when data is loading', () => {
    setupMocks({ isLoading: true })
    render(<RunsPage />)

    const skeletons = screen.getAllByTestId('skeleton-row')
    expect(skeletons).toHaveLength(5)
    expect(screen.queryByText('abcdef12')).not.toBeInTheDocument()
  })

  it('renders run rows with status badges', () => {
    setupMocks()
    render(<RunsPage />)

    // Truncated IDs (first 8 chars)
    expect(screen.getByText('abcdef12')).toBeInTheDocument()
    expect(screen.getByText('fedcba09')).toBeInTheDocument()

    // Status badges
    expect(screen.getByText('COMPLETED')).toBeInTheDocument()
    expect(screen.getByText('FAILED')).toBeInTheDocument()

    // Source column
    expect(screen.getByText('my-workflow:latest')).toBeInTheDocument()
    expect(screen.getByText('/path/to/workflow.wdl')).toBeInTheDocument()
  })

  it('renders empty state when no runs exist', () => {
    setupMocks({ runs: [] })
    render(<RunsPage />)

    expect(screen.getByText('No runs yet.')).toBeInTheDocument()
    expect(screen.getByText(/Submit a workflow using the flowy CLI/)).toBeInTheDocument()
  })

  it('navigates to run detail on row click', () => {
    setupMocks()
    render(<RunsPage />)

    const row = screen.getByText('abcdef12').closest('tr')!
    fireEvent.click(row)

    expect(mockNavigate).toHaveBeenCalledWith(
      '/projects/test-project/runs/abcdef12-3456-7890-abcd-ef1234567890'
    )
  })

  it('renders error state when query errors', () => {
    setupMocks({ isError: true, runs: [] })
    render(<RunsPage />)

    expect(screen.getByText(/Failed to load runs/)).toBeInTheDocument()
  })

  it('shows pulsing dot for RUNNING run', () => {
    setupMocks({ runs: mockRunWithRunning })
    render(<RunsPage />)
    expect(screen.getByLabelText('polling active')).toBeInTheDocument()
  })

  it('does not show pulsing dot for COMPLETED run', () => {
    setupMocks()  // default mockRuns has COMPLETED and FAILED
    render(<RunsPage />)
    expect(screen.queryByLabelText('polling active')).not.toBeInTheDocument()
  })
})
