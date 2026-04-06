import { useState } from 'react'
import { useParams, useNavigate } from 'react-router'
import { MoreHorizontal, Loader2 } from 'lucide-react'
import { toast } from 'sonner'

import {
  useProjectDetail,
  useProjectMembers,
  useUpdateProject,
  useDeleteProject,
  useUpdateMember,
  useRemoveMember,
} from '@/features/projects/useProjects'
import type { MemberResponse } from '@/features/projects/useProjects'
import { AddMemberDialog } from '@/features/projects/AddMemberDialog'
import { useAuthStore } from '@/store/authStore'
import { useProjectStore } from '@/store/projectStore'
import { Button } from '@/components/ui/button'
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
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog'
import { Textarea } from '@/components/ui/textarea'

function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  })
}

export function ProjectDetailPage() {
  const { slug } = useParams<{ slug: string }>()
  const navigate = useNavigate()

  const { data: project } = useProjectDetail(slug!)
  const { data: members, isLoading: membersLoading } = useProjectMembers(slug!)

  const currentUsername = useAuthStore((s) => s.username)
  const setActiveProject = useProjectStore((s) => s.setActiveProject)

  // Project-level admin check — derived from members list, NOT global authStore.role
  const currentMember = members?.find((m) => m.username === currentUsername)
  const isProjectAdmin = currentMember?.role === 'admin'

  const updateProject = useUpdateProject(slug!)
  const deleteProject = useDeleteProject(slug!)
  const updateMember = useUpdateMember(slug!)
  const removeMemberMutation = useRemoveMember(slug!)

  const [addMemberOpen, setAddMemberOpen] = useState(false)
  const [removeMember, setRemoveMember] = useState<MemberResponse | null>(null)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [editDescOpen, setEditDescOpen] = useState(false)
  const [editDesc, setEditDesc] = useState('')
  const [isRemovingMember, setIsRemovingMember] = useState(false)
  const [isDeletingProject, setIsDeletingProject] = useState(false)
  const [isSavingDesc, setIsSavingDesc] = useState(false)

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
      {/* Project info header */}
      <div>
        <h1 className="text-xl font-semibold">{project?.slug}</h1>
        <p className="text-xs text-muted-foreground">slug: {project?.slug}</p>
        <p className="text-sm mt-2">
          {project?.description ? (
            project.description
          ) : (
            <span className="text-muted-foreground italic">No description.</span>
          )}
        </p>

        {isProjectAdmin && (
          <div className="flex gap-2 mt-4">
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

      {/* Members section */}
      <div className="mt-8">
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
      </div>

      {/* Remove member AlertDialog */}
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

      {/* Delete project AlertDialog */}
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

      {/* Edit description Dialog */}
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

      {/* Add member dialog */}
      <AddMemberDialog
        open={addMemberOpen}
        onOpenChange={setAddMemberOpen}
        slug={slug!}
        existingMembers={members?.map((m) => m.username) ?? []}
      />
    </div>
  )
}
