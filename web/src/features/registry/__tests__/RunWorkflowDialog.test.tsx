import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

// Mock useRegistry hooks
vi.mock('@/features/registry/useRegistry', () => ({
  useRegistryInputs: vi.fn(),
  useRunWorkflow: vi.fn(),
}))

vi.mock('@/features/projects/useProjects', () => ({
  useManagedFiles: vi.fn(),
}))

// Mock toast
vi.mock('sonner', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

// Mock react-router
const mockNavigate = vi.fn()
vi.mock('react-router', () => ({
  useNavigate: () => mockNavigate,
}))

import { useRegistryInputs, useRunWorkflow } from '@/features/registry/useRegistry'
import { useManagedFiles } from '@/features/projects/useProjects'
import { ApiError } from '@/lib/apiClient'
import { toast } from 'sonner'
import { RunWorkflowDialog } from '@/features/registry/RunWorkflowDialog'

const mockInputs = {
  inputs: [
    { name: 'name', type: 'String', optional: false, default: null, file_type: null },
    { name: 'count', type: 'Int', optional: true, default: '5', file_type: null },
    { name: 'threshold', type: 'Float', optional: false, default: null, file_type: null },
    { name: 'verbose', type: 'Boolean', optional: false, default: 'false', file_type: null },
    { name: 'input_file', type: 'File', optional: false, default: null, file_type: 'fastq' },
    { name: 'tags', type: 'Array[String]', optional: true, default: null, file_type: null },
  ],
}

const mockManagedFiles = [
  {
    id: 'file-1',
    path: '/managed/reads.fastq.gz',
    label: 'reads.fastq.gz',
    file_type: 'fastq',
    source_type: 'workflow',
    run_id: 'run-1',
    size: 100,
    created_at: '2026-04-01T00:00:00Z',
  },
  {
    id: 'file-2',
    path: '/managed/report.txt',
    label: 'report.txt',
    file_type: 'report',
    source_type: 'import',
    size: 100,
    created_at: '2026-04-01T00:00:00Z',
  },
]

const mockVersions = [
  {
    id: 'v1',
    digest: 'sha256:abc123def456...',
    tags: ['latest', 'v1.0'],
    created_at: '2026-04-01T00:00:00Z',
  },
  {
    id: 'v2',
    digest: 'sha256:def456abc123...',
    tags: ['v2.0'],
    created_at: '2026-04-02T00:00:00Z',
  },
]

const defaultMutateAsync = vi.fn()

function setupMocks({
  mutateAsync = defaultMutateAsync,
  isPending = false,
  inputsData = mockInputs,
  inputsLoading = false,
  inputsError = false,
} = {}) {
  vi.mocked(useRegistryInputs).mockReturnValue({
    data: inputsData,
    isLoading: inputsLoading,
    isError: inputsError,
  } as unknown as ReturnType<typeof useRegistryInputs>)

  vi.mocked(useRunWorkflow).mockReturnValue({
    mutateAsync,
    isPending,
  } as unknown as ReturnType<typeof useRunWorkflow>)

  vi.mocked(useManagedFiles).mockReturnValue({
    data: mockManagedFiles,
    isLoading: false,
  } as unknown as ReturnType<typeof useManagedFiles>)
}

const defaultProps = {
  open: true,
  onOpenChange: vi.fn(),
  slug: 'my-project',
  name: 'hello-workflow',
  versions: mockVersions,
}

describe('RunWorkflowDialog', () => {
  const user = userEvent.setup()

  beforeEach(() => {
    vi.clearAllMocks()
    defaultMutateAsync.mockResolvedValue({ run_id: 'run-uuid-123' })
    mockNavigate.mockReset()
    setupMocks()
  })

  it('renders version selector with available tags, defaults to latest', () => {
    render(<RunWorkflowDialog {...defaultProps} />)

    // Use heading role to avoid matching the submit button text "Run Workflow"
    expect(screen.getByRole('heading', { name: 'Run Workflow' })).toBeInTheDocument()
    expect(screen.getByText('Configure inputs and submit execution.')).toBeInTheDocument()
    // Check the selected value shows "latest" (may appear in trigger + options)
    const latestItems = screen.getAllByText('latest')
    expect(latestItems.length).toBeGreaterThan(0)
  })

  it('renders correct input control per WDL type', () => {
    render(<RunWorkflowDialog {...defaultProps} />)

    // String input
    const nameInput = screen.getByPlaceholderText('Enter a string value')
    expect(nameInput).toBeInTheDocument()
    expect(nameInput).toHaveAttribute('type', 'text')

    // Int input
    const countInput = screen.getByPlaceholderText('Enter an integer')
    expect(countInput).toBeInTheDocument()
    expect(countInput).toHaveAttribute('type', 'number')
    expect(countInput).toHaveAttribute('step', '1')

    // Float input
    const thresholdInput = screen.getByPlaceholderText('Enter a decimal number')
    expect(thresholdInput).toBeInTheDocument()
    expect(thresholdInput).toHaveAttribute('type', 'number')
    expect(thresholdInput).toHaveAttribute('step', 'any')

    // Boolean checkbox — look for a checkbox role
    const checkboxes = screen.getAllByRole('checkbox')
    expect(checkboxes.length).toBeGreaterThan(0)

    // File input with tooltip icon
    const fileInput = screen.getByPlaceholderText('e.g., s3://bucket/path or /absolute/path')
    expect(fileInput).toBeInTheDocument()

    // Array textarea
    const textarea = screen.getByPlaceholderText('Enter valid JSON')
    expect(textarea).toBeInTheDocument()
  })

  it('pre-fills default values from inputs endpoint', () => {
    render(<RunWorkflowDialog {...defaultProps} />)

    // count field has default value '5'
    const countInput = screen.getByPlaceholderText('Enter an integer')
    expect(countInput).toHaveValue(5)
  })

  it('shows validation error for empty required String field on submit', async () => {
    render(<RunWorkflowDialog {...defaultProps} />)

    // Submit without filling required fields
    await user.click(screen.getByRole('button', { name: 'Run Workflow' }))

    await waitFor(() => {
      expect(screen.getByText('name is required')).toBeInTheDocument()
    })
  })

  it('optional fields left empty are omitted from submission', async () => {
    render(<RunWorkflowDialog {...defaultProps} />)

    // Fill required fields
    await user.type(screen.getByPlaceholderText('Enter a string value'), 'test-name')
    await user.clear(screen.getByPlaceholderText('Enter an integer'))
    await user.type(screen.getByPlaceholderText('Enter a decimal number'), '1.5')
    await user.type(
      screen.getByPlaceholderText('e.g., s3://bucket/path or /absolute/path'),
      '/data/input.txt'
    )

    await user.click(screen.getByRole('button', { name: 'Run Workflow' }))

    await waitFor(() => {
      expect(defaultMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          inputs: expect.not.objectContaining({ count: expect.anything() }),
        })
      )
    })
  })

  it('File input shows tooltip with server-accessible path explanation', async () => {
    render(<RunWorkflowDialog {...defaultProps} />)

    // Assert the Info icon is present before hovering — fails explicitly if missing
    const infoIcon = screen.getByTestId('file-input-tooltip-trigger')
    await user.hover(infoIcon)

    await waitFor(() => {
      // Tooltip content may be rendered multiple times (visible + hidden by Radix)
      const tooltipItems = screen.getAllByText(/Enter a server-accessible path/i)
      expect(tooltipItems.length).toBeGreaterThan(0)
    })
  })

  it('shows managed file picker guidance for File inputs', () => {
    render(<RunWorkflowDialog {...defaultProps} />)

    expect(screen.getByLabelText('Managed file picker for input_file')).toBeInTheDocument()
    expect(
      screen.getByText('Pick from the project file library or enter a manual path. Showing only files tagged "fastq".')
    ).toBeInTheDocument()
  })

  it('renders selected managed file label-first with path context', async () => {
    render(<RunWorkflowDialog {...defaultProps} />)

    await user.type(
      screen.getByPlaceholderText('e.g., s3://bucket/path or /absolute/path'),
      '/managed/reads.fastq.gz'
    )

    expect(screen.getByText('reads.fastq.gz')).toBeInTheDocument()
    expect(screen.getByText('/managed/reads.fastq.gz')).toBeInTheDocument()
  })

  it('calls POST /projects/{slug}/workflows on submit with correct payload', async () => {
    render(<RunWorkflowDialog {...defaultProps} />)

    await user.type(screen.getByPlaceholderText('Enter a string value'), 'hello')
    await user.clear(screen.getByPlaceholderText('Enter an integer'))
    await user.type(screen.getByPlaceholderText('Enter a decimal number'), '2.5')
    await user.type(
      screen.getByPlaceholderText('e.g., s3://bucket/path or /absolute/path'),
      '/data/file.txt'
    )

    await user.click(screen.getByRole('button', { name: 'Run Workflow' }))

    await waitFor(() => {
      expect(defaultMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          registry_ref: 'hello-workflow:latest',
          inputs: expect.objectContaining({
            name: 'hello',
            threshold: 2.5,
            input_file: '/data/file.txt',
          }),
        })
      )
    })
  })

  it('shows success toast and navigates to RunDetailPage', async () => {
    render(<RunWorkflowDialog {...defaultProps} />)

    await user.type(screen.getByPlaceholderText('Enter a string value'), 'hello')
    await user.clear(screen.getByPlaceholderText('Enter an integer'))
    await user.type(screen.getByPlaceholderText('Enter a decimal number'), '2.5')
    await user.type(
      screen.getByPlaceholderText('e.g., s3://bucket/path or /absolute/path'),
      '/data/file.txt'
    )

    await user.click(screen.getByRole('button', { name: 'Run Workflow' }))

    await waitFor(() => {
      expect(toast.success).toHaveBeenCalledWith('Run submitted.')
      expect(mockNavigate).toHaveBeenCalledWith('/projects/my-project/runs/run-uuid-123')
    })
  })

  it('shows mapped error toast for 400', async () => {
    defaultMutateAsync.mockRejectedValue(new ApiError(400, 'bad input'))
    render(<RunWorkflowDialog {...defaultProps} />)

    await user.type(screen.getByPlaceholderText('Enter a string value'), 'hello')
    await user.clear(screen.getByPlaceholderText('Enter an integer'))
    await user.type(screen.getByPlaceholderText('Enter a decimal number'), '2.5')
    await user.type(
      screen.getByPlaceholderText('e.g., s3://bucket/path or /absolute/path'),
      '/data/file.txt'
    )

    await user.click(screen.getByRole('button', { name: 'Run Workflow' }))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith(
        'Invalid inputs. Check the values and try again.'
      )
    })
  })

  it('shows mapped error toast for 404', async () => {
    defaultMutateAsync.mockRejectedValue(new ApiError(404, 'not found'))
    render(<RunWorkflowDialog {...defaultProps} />)

    await user.type(screen.getByPlaceholderText('Enter a string value'), 'hello')
    await user.clear(screen.getByPlaceholderText('Enter an integer'))
    await user.type(screen.getByPlaceholderText('Enter a decimal number'), '2.5')
    await user.type(
      screen.getByPlaceholderText('e.g., s3://bucket/path or /absolute/path'),
      '/data/file.txt'
    )

    await user.click(screen.getByRole('button', { name: 'Run Workflow' }))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith('Workflow version not found.')
    })
  })

  it('shows mapped error toast for 500', async () => {
    defaultMutateAsync.mockRejectedValue(new ApiError(500, 'server error'))
    render(<RunWorkflowDialog {...defaultProps} />)

    await user.type(screen.getByPlaceholderText('Enter a string value'), 'hello')
    await user.clear(screen.getByPlaceholderText('Enter an integer'))
    await user.type(screen.getByPlaceholderText('Enter a decimal number'), '2.5')
    await user.type(
      screen.getByPlaceholderText('e.g., s3://bucket/path or /absolute/path'),
      '/data/file.txt'
    )

    await user.click(screen.getByRole('button', { name: 'Run Workflow' }))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith('Server error. Please try again.')
    })
  })

  it('shows loading skeletons while inputs are loading', () => {
    setupMocks({ inputsData: undefined, inputsLoading: true })
    render(<RunWorkflowDialog {...defaultProps} />)

    const skeletons = document.querySelectorAll('.animate-pulse')
    expect(skeletons.length).toBeGreaterThan(0)
  })

  it('shows no-inputs message for empty inputs array', () => {
    setupMocks({ inputsData: { inputs: [] } })
    render(<RunWorkflowDialog {...defaultProps} />)

    expect(
      screen.getByText(
        'This workflow declares no inputs. Click Run Workflow to execute with defaults.'
      )
    ).toBeInTheDocument()
  })

  it('shows error message when input schema fetch fails', () => {
    setupMocks({ inputsData: undefined, inputsError: true })
    render(<RunWorkflowDialog {...defaultProps} />)

    expect(
      screen.getByText('Failed to load workflow inputs. Please close and try again.')
    ).toBeInTheDocument()

    const submitBtn = screen.getByRole('button', { name: 'Run Workflow' })
    expect(submitBtn).toBeDisabled()
  })

  it('resets form on dialog close', async () => {
    const onOpenChange = vi.fn()
    const { rerender } = render(
      <RunWorkflowDialog {...defaultProps} onOpenChange={onOpenChange} />
    )

    await user.type(screen.getByPlaceholderText('Enter a string value'), 'something')
    expect(screen.getByPlaceholderText('Enter a string value')).toHaveValue('something')

    await user.click(screen.getByRole('button', { name: 'Discard' }))
    expect(onOpenChange).toHaveBeenCalledWith(false)

    // Reopen dialog
    rerender(
      <RunWorkflowDialog {...defaultProps} open={true} onOpenChange={onOpenChange} />
    )

    await waitFor(() => {
      expect(screen.getByPlaceholderText('Enter a string value')).toHaveValue('')
    })
  })
})
