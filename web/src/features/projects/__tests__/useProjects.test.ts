import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'

vi.mock('@/lib/apiClient', () => ({
  apiClient: {
    get: vi.fn(),
    post: vi.fn(),
    postForm: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
    patch: vi.fn(),
  },
}))

import { apiClient } from '@/lib/apiClient'
import {
  useProjects,
  useProjectDetail,
  useProjectMembers,
  useCreateProject,
  useUpdateProject,
  useDeleteProject,
  useAddMember,
  useUpdateMember,
  useRemoveMember,
  useImportManagedFile,
} from '../useProjects'

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })
  return ({ children }: { children: React.ReactNode }) => (
    React.createElement(QueryClientProvider, { client: queryClient }, children)
  )
}

const mockProject = {
  id: 'proj-1',
  slug: 'my-project',
  description: 'A test project',
  is_active: true,
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
}

const mockMember = {
  user_id: 'user-1',
  username: 'alice',
  role: 'admin',
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
}

describe('useProjects', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('fetches projects from GET /projects', async () => {
    vi.mocked(apiClient.get).mockResolvedValue([mockProject])

    const { result } = renderHook(() => useProjects(), { wrapper: createWrapper() })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(apiClient.get).toHaveBeenCalledWith('/projects')
    expect(result.current.data).toEqual([mockProject])
  })
})

describe('useProjectDetail', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('fetches project detail from GET /projects/{slug}', async () => {
    vi.mocked(apiClient.get).mockResolvedValue(mockProject)

    const { result } = renderHook(() => useProjectDetail('my-project'), { wrapper: createWrapper() })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(apiClient.get).toHaveBeenCalledWith('/projects/my-project')
    expect(result.current.data).toEqual(mockProject)
  })
})

describe('useProjectMembers', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('fetches members from GET /projects/{slug}/members', async () => {
    vi.mocked(apiClient.get).mockResolvedValue([mockMember])

    const { result } = renderHook(() => useProjectMembers('my-project'), { wrapper: createWrapper() })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(apiClient.get).toHaveBeenCalledWith('/projects/my-project/members')
    expect(result.current.data).toEqual([mockMember])
  })
})

describe('useCreateProject', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('posts to /projects and invalidates projects query', async () => {
    vi.mocked(apiClient.post).mockResolvedValue(mockProject)

    const { result } = renderHook(() => useCreateProject(), { wrapper: createWrapper() })

    result.current.mutate({ slug: 'my-project', description: 'A test project' })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(apiClient.post).toHaveBeenCalledWith('/projects', {
      slug: 'my-project',
      description: 'A test project',
    })
  })
})

describe('useUpdateProject', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('puts to /projects/{slug} and invalidates project query', async () => {
    vi.mocked(apiClient.put).mockResolvedValue(mockProject)

    const { result } = renderHook(() => useUpdateProject('my-project'), { wrapper: createWrapper() })

    result.current.mutate({ description: 'Updated description' })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(apiClient.put).toHaveBeenCalledWith('/projects/my-project', {
      description: 'Updated description',
    })
  })
})

describe('useDeleteProject', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('deletes /projects/{slug} and invalidates projects query', async () => {
    vi.mocked(apiClient.delete).mockResolvedValue(undefined)

    const { result } = renderHook(() => useDeleteProject('my-project'), { wrapper: createWrapper() })

    result.current.mutate()

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(apiClient.delete).toHaveBeenCalledWith('/projects/my-project')
  })
})

describe('useAddMember', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('posts to /projects/{slug}/members and invalidates members query', async () => {
    vi.mocked(apiClient.post).mockResolvedValue(mockMember)

    const { result } = renderHook(() => useAddMember('my-project'), { wrapper: createWrapper() })

    result.current.mutate({ username: 'alice', role: 'admin' })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(apiClient.post).toHaveBeenCalledWith('/projects/my-project/members', {
      username: 'alice',
      role: 'admin',
    })
  })
})

describe('useUpdateMember', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('puts to /projects/{slug}/members/{username} with role', async () => {
    vi.mocked(apiClient.put).mockResolvedValue(mockMember)

    const { result } = renderHook(() => useUpdateMember('my-project'), { wrapper: createWrapper() })

    result.current.mutate({ username: 'alice', role: 'viewer' })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(apiClient.put).toHaveBeenCalledWith('/projects/my-project/members/alice', {
      role: 'viewer',
    })
  })
})

describe('useRemoveMember', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('deletes /projects/{slug}/members/{username}', async () => {
    vi.mocked(apiClient.delete).mockResolvedValue(undefined)

    const { result } = renderHook(() => useRemoveMember('my-project'), { wrapper: createWrapper() })

    result.current.mutate('alice')

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(apiClient.delete).toHaveBeenCalledWith('/projects/my-project/members/alice')
  })
})

describe('useImportManagedFile', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('posts multipart upload with optional file_type', async () => {
    vi.mocked(apiClient.postForm).mockResolvedValue({
      file: {
        id: 'file-1',
        path: '/tmp/reads.fastq.gz',
        file_type: 'fastq',
        label: 'reads.fastq.gz',
        size: 42,
        created_at: '2024-01-01T00:00:00Z',
      },
    })

    const { result } = renderHook(() => useImportManagedFile('my-project'), { wrapper: createWrapper() })
    const file = new File(['acgt'], 'reads.fastq.gz')

    result.current.mutate({ file, fileType: 'fastq' })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(apiClient.postForm).toHaveBeenCalledWith(
      '/projects/my-project/files',
      expect.any(FormData)
    )
    const formData = vi.mocked(apiClient.postForm).mock.calls[0][1]
    expect(formData.get('file')).toBe(file)
    expect(formData.get('file_type')).toBe('fastq')
  })

  it('posts JSON import with optional file_type', async () => {
    vi.mocked(apiClient.post).mockResolvedValue({
      file: {
        id: 'file-1',
        path: 's3://bucket/reads.fastq.gz',
        file_type: 'fastq',
        label: 'reads.fastq.gz',
        size: 42,
        created_at: '2024-01-01T00:00:00Z',
      },
    })

    const { result } = renderHook(() => useImportManagedFile('my-project'), { wrapper: createWrapper() })

    result.current.mutate({ uri: 's3://bucket/reads.fastq.gz', fileType: 'fastq' })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(apiClient.post).toHaveBeenCalledWith('/projects/my-project/files', {
      uri: 's3://bucket/reads.fastq.gz',
      file_type: 'fastq',
    })
  })
})
