import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen } from '@testing-library/react'

// Mock useRuns hooks
vi.mock('@/features/runs/useRuns', () => ({
  useRuns: vi.fn(),
  useRunOutputs: vi.fn(),
}))

// Mock react-router
vi.mock('react-router', () => ({
  useParams: vi.fn(() => ({ slug: 'test-project', id: 'test-run-id-1234-5678-abcd' })),
  useNavigate: vi.fn(() => vi.fn()),
}))

// Mock authStore
vi.mock('@/store/authStore', () => ({
  useAuthStore: {
    getState: vi.fn(() => ({ token: 'test-token' })),
  },
}))

// Mock toast
vi.mock('sonner', () => ({
  toast: {
    error: vi.fn(),
  },
}))

import { useRuns, useRunOutputs } from '@/features/runs/useRuns'
import { RunDetailPage } from '@/features/runs/RunDetailPage'

const mockCompletedRun = {
  id: 'test-run-id-1234-5678-abcd',
  state: 'COMPLETED',
  registry_ref: 'my-workflow:latest',
  wdl_path: undefined,
  created_at: '2026-04-01T10:00:00Z',
  updated_at: '2026-04-01T10:05:00Z',
}

const mockRunningRun = {
  ...mockCompletedRun,
  state: 'RUNNING',
}

const mockFailedRun = {
  ...mockCompletedRun,
  state: 'FAILED',
}

const mockOutputs = {
  run_id: 'test-run-id-1234-5678-abcd',
  outputs: {
    output_file: '/data/output.txt',
    output_string: 'hello',
    bam_file: '/data/result.bam',
  },
}

function setupMocks({
  run = mockCompletedRun,
  outputs = mockOutputs,
  outputsLoading = false,
}: {
  run?: typeof mockCompletedRun
  outputs?: typeof mockOutputs | null
  outputsLoading?: boolean
} = {}) {
  vi.mocked(useRuns).mockReturnValue({
    data: [run],
    isLoading: false,
    isError: false,
  } as ReturnType<typeof useRuns>)
  vi.mocked(useRunOutputs).mockReturnValue({
    data: outputsLoading ? undefined : (outputs ?? undefined),
    isLoading: outputsLoading,
    isError: false,
  } as ReturnType<typeof useRunOutputs>)
}

describe('RunDetailPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders metadata for a completed run', () => {
    setupMocks()
    render(<RunDetailPage />)

    // Heading: first 8 chars of run ID
    expect(screen.getByText('test-run')).toBeInTheDocument()
    // Full ID below heading
    expect(screen.getByText('test-run-id-1234-5678-abcd')).toBeInTheDocument()
    // Status badge
    expect(screen.getByText('COMPLETED')).toBeInTheDocument()
    // Registry ref
    expect(screen.getByText(/my-workflow:latest/)).toBeInTheDocument()
  })

  it('renders outputs table for a completed run', () => {
    setupMocks()
    render(<RunDetailPage />)

    // Output keys
    expect(screen.getByText('output_file')).toBeInTheDocument()
    expect(screen.getByText('output_string')).toBeInTheDocument()

    // File type badge
    expect(screen.getAllByText('File').length).toBeGreaterThan(0)

    // Filename in value column (last path segment)
    expect(screen.getByText('output.txt')).toBeInTheDocument()

    // Primitive value
    expect(screen.getByText('hello')).toBeInTheDocument()
  })

  it('renders not-completed message for a RUNNING run', () => {
    setupMocks({ run: mockRunningRun })
    render(<RunDetailPage />)

    expect(screen.getByText(/currently in progress/)).toBeInTheDocument()
  })

  it('renders not-completed message for a FAILED run', () => {
    setupMocks({ run: mockFailedRun })
    render(<RunDetailPage />)

    expect(screen.getByText(/This run failed/)).toBeInTheDocument()
  })

  it('shows Download button for file-type outputs', () => {
    setupMocks()
    render(<RunDetailPage />)

    const downloadButtons = screen.getAllByRole('button', { name: /download/i })
    expect(downloadButtons.length).toBeGreaterThan(0)
  })

  it('shows Preview button for text file outputs (.txt)', () => {
    setupMocks()
    render(<RunDetailPage />)

    // output_file is /data/output.txt — previewable
    const previewButtons = screen.getAllByRole('button', { name: /preview/i })
    expect(previewButtons.length).toBeGreaterThan(0)
  })

  it('does not show Preview button for non-text file outputs (.bam)', () => {
    // Only mock a run with bam file output
    vi.mocked(useRuns).mockReturnValue({
      data: [mockCompletedRun],
      isLoading: false,
      isError: false,
    } as ReturnType<typeof useRuns>)
    vi.mocked(useRunOutputs).mockReturnValue({
      data: {
        run_id: 'test-run-id-1234-5678-abcd',
        outputs: { bam_file: '/data/result.bam' },
      },
      isLoading: false,
      isError: false,
    } as ReturnType<typeof useRunOutputs>)

    render(<RunDetailPage />)

    // bam_file should have Download but no Preview
    expect(screen.getByRole('button', { name: /download/i })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /preview/i })).not.toBeInTheDocument()
  })
})
