import { useState } from 'react'
import { useNavigate, useParams } from 'react-router'
import { Loader2, MoreHorizontal } from 'lucide-react'
import { toast } from 'sonner'

import {
  useDeleteManagedFile,
  useDeleteProject,
  useManagedFiles,
  useProjectDetail,
  useProjectMembers,
  useRemoveMember,
  useUpdateMember,
  useUpdateProject,
} from '@/features/projects/useProjects'
import type { ManagedFileResponse, MemberResponse } from '@/features/projects/useProjects'
import { AddMemberDialog } from '@/features/projects/AddMemberDialog'
import { ImportFileDialog } from '@/features/projects/ImportFileDialog'
import { useAuthStore } from '@/store/authStore'
import { useProjectStore } from '@/store/projectStore'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Separator } from '@/components/ui/separator'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Textarea } from '@/components/ui/textarea'
import { cn } from '@/lib/utils'

function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  })
}

function formatFileSize(size: number): string {
  if (size === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let value = size
  let unitIndex = 0
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024
    unitIndex += 1
  }
  const rounded = value >= 10 || unitIndex === 0 ? value.toFixed(0) : value.toFixed(1)
  return `${rounded} ${units[unitIndex]}`
}

function fileSourceLabel(sourceType: string): string {
  switch (sourceType) {
    case 'workflow':
      return 'Workflow'
    case 'upload':
      return 'Upload'
    case 'import':
      return 'Import'
    default:
      return 'Unknown'
  }
}

function filterManagedFiles(
  files: ManagedFileResponse[],
  search: string,
  fileType: string
): ManagedFileResponse[] {
  const normalizedSearch = search.trim().toLowerCase()
  return files.filter((file) => {
    if (fileType !== 'all' && file.file_type !== fileType) {
      return false
    }
    if (normalizedSearch === '') {
      return true
    }
    return [file.label, file.path, file.file_type]
      .join(' ')
      .toLowerCase()
      .includes(normalizedSearch)
  })
}

type ProjectTab = 'members' | 'files'

export function ProjectDetailPage() {
  const { slug } = useParams<{ slug: string }>()
  const navigate = useNavigate()

  const { data: project } = useProjectDetail(slug!)
  const { data: members, isLoading: membersLoading } = useProjectMembers(slug!)
  const { data: files, isLoading: filesLoading } = useManagedFiles(slug!)

  const currentUsername = useAuthStore((s) => s.username)
  const setActiveProject = useProjectStore((s) => s.setActiveProject)

  const currentMember = members?.find((m) => m.username === currentUsername)
  const isProjectAdmin = currentMember?.role === 'admin'
  const canManageFiles = currentMember?.role === 'admin' || currentMember?.role === 'executor'

  const updateProject = useUpdateProject(slug!)
  const deleteProject = useDeleteProject(slug!)
  const updateMember = useUpdateMember(slug!)
  const removeMemberMutation = useRemoveMember(slug!)
  const deleteManagedFile = useDeleteManagedFile(slug!)

  const [activeTab, setActiveTab] = useState<ProjectTab>('members')
  const [addMemberOpen, setAddMemberOpen] = useState(false)
  const [importOpen, setImportOpen] = useState(false)
  const [removeMember, setRemoveMember] = useState<MemberResponse | null>(null)
  const [removeFile, setRemoveFile] = useState<ManagedFileResponse | null>(null)
  const [fileSearch, setFileSearch] = useState('')
  const [fileTypeFilter, setFileTypeFilter] = useState('all')
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [editDescOpen, setEditDescOpen] = useState(false)
  const [editDesc, setEditDesc] = useState('')
  const [isRemovingMember, setIsRemovingMember] = useState(false)
  const [isRemovingFile, setIsRemovingFile] = useState(false)
  const [isDeletingProject, setIsDeletingProject] = useState(false)
  const [isSavingDesc, setIsSavingDesc] = useState(false)

  const fileTypeOptions = Array.from(
    new Set((files ?? []).map((file) => file.file_type).filter((value) => value !== ''))
  ).sort()
  const filteredFiles = filterManagedFiles(files ?? [], fileSearch, fileTypeFilter)

  async function handleRoleChange(member: MemberResponse, newRole: string) {
    try {
      await updateMember.mutateAsync({ username: member.username, role: newRole })
      toast.success('Role updated.')
    } catch {
      toast.error('Something went wrong. Please try again.')
    }
  }

  async function handleRemoveMember(e: React.MouseEvent) {
    e.preventDefault()
    if (!removeMember) return

    setIsRemovingMember(true)
    try {
      await removeMemberMutation.mutateAsync(removeMember.username)
      toast.success(`${removeMember.username} removed.`)
      setRemoveMember(null)
    } catch {
      toast.error('Something went wrong. Please try again.')
    } finally {
      setIsRemovingMember(false)
    }
  }

  async function handleRemoveFile(e: React.MouseEvent) {
    e.preventDefault()
    if (!removeFile) return

    setIsRemovingFile(true)
    try {
      await deleteManagedFile.mutateAsync(removeFile.id)
      toast.success(`${removeFile.label} removed.`)
      setRemoveFile(null)
    } catch {
      toast.error('Something went wrong. Please try again.')
    } finally {
      setIsRemovingFile(false)
    }
  }

  async function handleDeleteProject(e: React.MouseEvent) {
    e.preventDefault()

    setIsDeletingProject(true)
    try {
      await deleteProject.mutateAsync()
      setActiveProject(null)
      navigate('/projects')
      toast.success(`${slug} deleted.`)
    } catch {
      toast.error('Something went wrong. Please try again.')
    } finally {
      setIsDeletingProject(false)
      setDeleteOpen(false)
    }
  }

  async function handleUpdateDescription(e: React.FormEvent) {
    e.preventDefault()
    setIsSavingDesc(true)
    try {
      await updateProject.mutateAsync({ description: editDesc })
      toast.success('Description updated.')
      setEditDescOpen(false)
    } catch {
      toast.error('Something went wrong. Please try again.')
    } finally {
      setIsSavingDesc(false)
    }
  }

  return (
    <div className="p-6">
      <div>
        <h1 className="text-xl font-semibold">{project?.slug}</h1>
        <p className="mt-1 text-xs text-muted-foreground">slug: {project?.slug}</p>
        <p className="mt-2 text-sm">
          {project?.description ? (
            project.description
          ) : (
            <span className="italic text-muted-foreground">No description.</span>
          )}
        </p>

        {isProjectAdmin && (
          <div className="mt-4 flex gap-2">
            <Button
              variant="outline"
              onClick={() => {
                setEditDesc(project?.description ?? '')
                setEditDescOpen(true)
              }}
            >
              Edit Description
            </Button>
            <Button
              variant="outline"
              className="text-destructive"
              onClick={() => setDeleteOpen(true)}
            >
              Delete Project
            </Button>
          </div>
        )}
      </div>

      <Separator className="my-6" />

      <div className="inline-flex rounded-lg border bg-muted/30 p-1">
        <button
          type="button"
          className={cn(
            'rounded-md px-3 py-1.5 text-sm transition-colors',
            activeTab === 'members'
              ? 'bg-background text-foreground shadow-sm'
              : 'text-muted-foreground hover:text-foreground'
          )}
          onClick={() => setActiveTab('members')}
        >
          Members
        </button>
        <button
          type="button"
          className={cn(
            'rounded-md px-3 py-1.5 text-sm transition-colors',
            activeTab === 'files'
              ? 'bg-background text-foreground shadow-sm'
              : 'text-muted-foreground hover:text-foreground'
          )}
          onClick={() => setActiveTab('files')}
        >
          Files
        </button>
      </div>

      <div className="mt-8">
        {activeTab === 'members' ? (
          <>
            <div className="flex items-center justify-between">
              <h2 className="text-xl font-semibold">Members</h2>
              {isProjectAdmin && (
                <Button variant="default" onClick={() => setAddMemberOpen(true)}>
                  Add Member
                </Button>
              )}
            </div>

            <div className="mt-4">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Username</TableHead>
                    <TableHead>Role</TableHead>
                    <TableHead>Joined</TableHead>
                    <TableHead className="w-[50px]"></TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {membersLoading ? (
                    Array.from({ length: 3 }).map((_, i) => (
                      <TableRow key={i} data-testid="skeleton-row">
                        <TableCell>
                          <div className="h-4 w-24 animate-pulse rounded bg-muted" />
                        </TableCell>
                        <TableCell>
                          <div className="h-4 w-24 animate-pulse rounded bg-muted" />
                        </TableCell>
                        <TableCell>
                          <div className="h-4 w-20 animate-pulse rounded bg-muted" />
                        </TableCell>
                        <TableCell />
                      </TableRow>
                    ))
                  ) : !members || members.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={4} className="text-center text-muted-foreground">
                        No members yet.
                      </TableCell>
                    </TableRow>
                  ) : (
                    members.map((member) => (
                      <TableRow key={member.user_id}>
                        <TableCell>{member.username}</TableCell>
                        <TableCell>
                          <Select
                            value={member.role}
                            onValueChange={(newRole) => handleRoleChange(member, newRole)}
                            disabled={member.username === currentUsername}
                          >
                            <SelectTrigger
                              className="w-32"
                              aria-label={
                                member.username === currentUsername
                                  ? 'You cannot change your own role'
                                  : undefined
                              }
                            >
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value="admin">admin</SelectItem>
                              <SelectItem value="executor">executor</SelectItem>
                              <SelectItem value="viewer">viewer</SelectItem>
                            </SelectContent>
                          </Select>
                        </TableCell>
                        <TableCell>{formatDate(member.created_at)}</TableCell>
                        <TableCell>
                          {member.username === currentUsername ? null : (
                            <DropdownMenu>
                              <DropdownMenuTrigger asChild>
                                <Button
                                  variant="ghost"
                                  size="icon"
                                  className="h-8 w-8"
                                  aria-label="Open menu"
                                >
                                  <MoreHorizontal className="h-4 w-4" />
                                </Button>
                              </DropdownMenuTrigger>
                              <DropdownMenuContent align="end">
                                <DropdownMenuItem
                                  className="text-destructive"
                                  onClick={() => setRemoveMember(member)}
                                >
                                  Remove
                                </DropdownMenuItem>
                              </DropdownMenuContent>
                            </DropdownMenu>
                          )}
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </div>
          </>
        ) : (
          <>
            <div className="flex items-center justify-between gap-3">
              <div>
                <h2 className="text-xl font-semibold">Files</h2>
                <p className="mt-1 text-sm text-muted-foreground">
                  Managed files available for imports and workflow launches.
                </p>
              </div>
              {canManageFiles && (
                <Button onClick={() => setImportOpen(true)}>Import File</Button>
              )}
            </div>

            <div className="mt-4">
              <div className="mb-4 flex flex-col gap-3 sm:flex-row">
                <Input
                  value={fileSearch}
                  onChange={(e) => setFileSearch(e.target.value)}
                  placeholder="Filter by label, path, or type"
                  className="sm:max-w-sm"
                />
                <Select value={fileTypeFilter} onValueChange={setFileTypeFilter}>
                  <SelectTrigger className="sm:w-48">
                    <SelectValue placeholder="All types" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">All types</SelectItem>
                    {fileTypeOptions.map((fileType) => (
                      <SelectItem key={fileType} value={fileType}>
                        {fileType}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>File</TableHead>
                    <TableHead>Type</TableHead>
                    <TableHead>Source</TableHead>
                    <TableHead>Size</TableHead>
                    <TableHead>Created</TableHead>
                    <TableHead className="w-[50px]"></TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {filesLoading ? (
                    Array.from({ length: 3 }).map((_, i) => (
                      <TableRow key={i} data-testid="file-skeleton-row">
                        <TableCell>
                          <div className="h-4 w-64 animate-pulse rounded bg-muted" />
                        </TableCell>
                        <TableCell>
                          <div className="h-4 w-20 animate-pulse rounded bg-muted" />
                        </TableCell>
                        <TableCell>
                          <div className="h-4 w-20 animate-pulse rounded bg-muted" />
                        </TableCell>
                        <TableCell>
                          <div className="h-4 w-16 animate-pulse rounded bg-muted" />
                        </TableCell>
                        <TableCell>
                          <div className="h-4 w-20 animate-pulse rounded bg-muted" />
                        </TableCell>
                        <TableCell />
                      </TableRow>
                    ))
                  ) : !files || files.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={6} className="text-center text-muted-foreground">
                        No files in this project yet.
                      </TableCell>
                    </TableRow>
                  ) : filteredFiles.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={6} className="text-center text-muted-foreground">
                        No files match the current filters.
                      </TableCell>
                    </TableRow>
                  ) : (
                    filteredFiles.map((file) => (
                      <TableRow key={file.id}>
                        <TableCell>
                          <div className="space-y-1">
                            <p className="font-medium">{file.label}</p>
                            <p className="font-mono text-xs text-muted-foreground">{file.path}</p>
                          </div>
                        </TableCell>
                        <TableCell>{file.file_type || '\u2014'}</TableCell>
                        <TableCell>
                          <div className="flex flex-col items-start gap-1">
                            <Badge variant="outline">{fileSourceLabel(file.source_type)}</Badge>
                            {file.run_id ? (
                              <Button
                                type="button"
                                variant="link"
                                className="h-auto p-0 text-xs"
                                onClick={() => navigate(`/projects/${slug}/runs/${file.run_id}`)}
                              >
                                View run
                              </Button>
                            ) : null}
                          </div>
                        </TableCell>
                        <TableCell>{formatFileSize(file.size)}</TableCell>
                        <TableCell>{formatDate(file.created_at)}</TableCell>
                        <TableCell>
                          {canManageFiles ? (
                            <Button
                              variant="ghost"
                              size="sm"
                              className="text-destructive"
                              onClick={() => setRemoveFile(file)}
                            >
                              Delete
                            </Button>
                          ) : null}
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </div>
          </>
        )}
      </div>

      <AlertDialog
        open={!!removeMember}
        onOpenChange={(open) => {
          if (!open) setRemoveMember(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              Remove {removeMember?.username} from {slug}?
            </AlertDialogTitle>
            <AlertDialogDescription>
              This will revoke their access to this project.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel asChild>
              <Button variant="outline">Keep Member</Button>
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={handleRemoveMember}
              disabled={isRemovingMember}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              {isRemovingMember ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Removing...
                </>
              ) : (
                'Remove'
              )}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={!!removeFile}
        onOpenChange={(open) => {
          if (!open) setRemoveFile(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete {removeFile?.label}?</AlertDialogTitle>
            <AlertDialogDescription>
              This removes the library record only. The underlying object is left untouched.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel asChild>
              <Button variant="outline">Keep File</Button>
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={handleRemoveFile}
              disabled={isRemovingFile}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              {isRemovingFile ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Deleting...
                </>
              ) : (
                'Delete'
              )}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete {slug}?</AlertDialogTitle>
            <AlertDialogDescription>
              This action cannot be undone. All project data will be removed.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel asChild>
              <Button variant="outline">Keep Project</Button>
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={handleDeleteProject}
              disabled={isDeletingProject}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              {isDeletingProject ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Deleting...
                </>
              ) : (
                'Delete'
              )}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <Dialog open={editDescOpen} onOpenChange={setEditDescOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Edit Description</DialogTitle>
          </DialogHeader>
          <form onSubmit={handleUpdateDescription} className="space-y-4">
            <div>
              <Textarea
                rows={4}
                placeholder="A brief description of this project."
                value={editDesc}
                onChange={(e) => setEditDesc(e.target.value)}
              />
            </div>
            <DialogFooter className="flex flex-col gap-2 sm:flex-row">
              <Button
                type="button"
                variant="ghost"
                onClick={() => setEditDescOpen(false)}
              >
                Discard Changes
              </Button>
              <Button type="submit" disabled={isSavingDesc}>
                {isSavingDesc ? (
                  <>
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    Saving...
                  </>
                ) : (
                  'Update Description'
                )}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <AddMemberDialog
        open={addMemberOpen}
        onOpenChange={setAddMemberOpen}
        slug={slug!}
        existingMembers={members?.map((m) => m.username) ?? []}
      />
      <ImportFileDialog
        open={importOpen}
        onOpenChange={setImportOpen}
        slug={slug!}
      />
    </div>
  )
}
