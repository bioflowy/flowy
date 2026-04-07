import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

// Mock useRegistry hooks
vi.mock('@/features/registry/useRegistry', () => ({
  usePushWDL: vi.fn(),
}))

// Mock toast
vi.mock('sonner', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

import { usePushWDL } from '@/features/registry/useRegistry'
import { ApiError } from '@/lib/apiClient'
import { toast } from 'sonner'
import { PushWDLDialog } from '@/features/registry/PushWDLDialog'

function createMockFile(name: string, content: string): File {
  return new File([content], name, { type: 'text/plain' })
}

const defaultMutateAsync = vi.fn()

function setupMocks({ mutateAsync = defaultMutateAsync, isPending = false } = {}) {
  vi.mocked(usePushWDL).mockReturnValue({
    mutateAsync,
    isPending,
  } as unknown as ReturnType<typeof usePushWDL>)
}

describe('PushWDLDialog', () => {
  const user = userEvent.setup()

  beforeEach(() => {
    vi.clearAllMocks()
    defaultMutateAsync.mockResolvedValue({
      name: 'my-workflow',
      digest: 'sha256:abc123def456ghi789',
      kind: 'workflow',
      version_id: 'v1',
    })
    setupMocks()
  })

  it('renders dialog with entry name input and file picker', () => {
    render(
      <PushWDLDialog open={true} onOpenChange={vi.fn()} slug="my-project" />
    )

    expect(screen.getByText('Push WDL to Registry')).toBeInTheDocument()
    expect(screen.getByLabelText('Entry name')).toBeInTheDocument()
    const fileInput = screen.getByLabelText('WDL file')
    expect(fileInput).toBeInTheDocument()
    expect(fileInput).toHaveAttribute('accept', '.wdl')
  })

  it('submit button disabled when fields are empty', () => {
    render(
      <PushWDLDialog open={true} onOpenChange={vi.fn()} slug="my-project" />
    )

    const submitBtn = screen.getByRole('button', { name: 'Push WDL' })
    expect(submitBtn).toBeDisabled()
  })

  it('shows validation error for non-.wdl file', async () => {
    render(
      <PushWDLDialog open={true} onOpenChange={vi.fn()} slug="my-project" />
    )

    await user.type(screen.getByLabelText('Entry name'), 'my-workflow')

    const fileInput = screen.getByLabelText('WDL file')
    const badFile = createMockFile('test.txt', 'not a wdl file')
    // Use fireEvent.change to simulate file selection — userEvent.upload does not
    // trigger onChange through Radix Slot in jsdom (pointer-events blocked by dialog overlay)
    Object.defineProperty(fileInput, 'files', {
      value: { 0: badFile, length: 1, item: () => badFile },
      configurable: true,
    })
    fireEvent.change(fileInput)

    await waitFor(() => {
      expect(
        screen.getByText('Only .wdl files are accepted. Please select a valid WDL file.')
      ).toBeInTheDocument()
    })
  })

  it('calls mutateAsync on valid submission', async () => {
    render(
      <PushWDLDialog open={true} onOpenChange={vi.fn()} slug="my-project" />
    )

    await user.type(screen.getByLabelText('Entry name'), 'my-workflow')

    const fileInput = screen.getByLabelText('WDL file')
    const wdlFile = createMockFile('test.wdl', 'version 1.2\nworkflow hello {}')
    await user.upload(fileInput, wdlFile)

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Push WDL' })).not.toBeDisabled()
    })

    await user.click(screen.getByRole('button', { name: 'Push WDL' }))

    await waitFor(() => {
      expect(defaultMutateAsync).toHaveBeenCalledWith({
        name: 'my-workflow',
        content: 'version 1.2\nworkflow hello {}',
      })
    })
  })

  it('shows success toast after push', async () => {
    render(
      <PushWDLDialog open={true} onOpenChange={vi.fn()} slug="my-project" />
    )

    await user.type(screen.getByLabelText('Entry name'), 'my-workflow')

    const fileInput = screen.getByLabelText('WDL file')
    const wdlFile = createMockFile('test.wdl', 'version 1.2\nworkflow hello {}')
    await user.upload(fileInput, wdlFile)

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Push WDL' })).not.toBeDisabled()
    })

    await user.click(screen.getByRole('button', { name: 'Push WDL' }))

    await waitFor(() => {
      expect(toast.success).toHaveBeenCalledWith(
        expect.stringContaining('my-workflow pushed')
      )
    })
  })

  it('shows error toast for 422', async () => {
    defaultMutateAsync.mockRejectedValue(new ApiError(422, 'parse error at line 1'))

    render(
      <PushWDLDialog open={true} onOpenChange={vi.fn()} slug="my-project" />
    )

    await user.type(screen.getByLabelText('Entry name'), 'my-workflow')

    const fileInput = screen.getByLabelText('WDL file')
    await user.upload(fileInput, createMockFile('test.wdl', 'version 1.2'))

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Push WDL' })).not.toBeDisabled()
    })

    await user.click(screen.getByRole('button', { name: 'Push WDL' }))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith('Invalid WDL: parse error at line 1')
    })
  })

  it('shows error toast for 409', async () => {
    defaultMutateAsync.mockRejectedValue(new ApiError(409, 'conflict'))

    render(
      <PushWDLDialog open={true} onOpenChange={vi.fn()} slug="my-project" />
    )

    await user.type(screen.getByLabelText('Entry name'), 'my-workflow')

    const fileInput = screen.getByLabelText('WDL file')
    await user.upload(fileInput, createMockFile('test.wdl', 'version 1.2'))

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Push WDL' })).not.toBeDisabled()
    })

    await user.click(screen.getByRole('button', { name: 'Push WDL' }))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith(
        expect.stringContaining('Name already exists')
      )
    })
  })

  it('resets form on dialog close', async () => {
    const onOpenChange = vi.fn()

    const { rerender } = render(
      <PushWDLDialog open={true} onOpenChange={onOpenChange} slug="my-project" />
    )

    await user.type(screen.getByLabelText('Entry name'), 'some-name')

    // Close dialog
    await user.click(screen.getByRole('button', { name: 'Discard' }))

    // Reopen
    rerender(
      <PushWDLDialog open={true} onOpenChange={onOpenChange} slug="my-project" />
    )

    expect(screen.getByLabelText('Entry name')).toHaveValue('')
  })
})
