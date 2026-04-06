import { useState } from 'react'
import { MoreHorizontal } from 'lucide-react'
import { toast } from 'sonner'

import { useAuthStore } from '@/store/authStore'
import { useUsers, useReactivateUser } from '@/features/users/useUsers'
import type { UserResponse } from '@/features/users/useUsers'
import { CreateUserDialog } from '@/features/users/CreateUserDialog'
import { DeactivateDialog } from '@/features/users/DeactivateDialog'
import { ResetPasswordDialog } from '@/features/users/ResetPasswordDialog'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'

function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  })
}

export function UsersPage() {
  const [createOpen, setCreateOpen] = useState(false)
  const [deactivateUser, setDeactivateUser] = useState<{ id: string; username: string } | null>(null)
  const [resetUser, setResetUser] = useState<{ id: string; username: string } | null>(null)

  const currentUsername = useAuthStore((s) => s.username)
  const { data: users, isLoading } = useUsers()
  const reactivate = useReactivateUser()

  async function handleReactivate(user: UserResponse) {
    try {
      await reactivate.mutateAsync(user.id)
      toast.success(`${user.username} reactivated.`)
    } catch {
      toast.error('Something went wrong. Please try again.')
    }
  }

  return (
    <div className="p-6">
      {/* Page header */}
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">Users</h1>
        <Button variant="default" onClick={() => setCreateOpen(true)}>
          Create User
        </Button>
      </div>

      {/* Table */}
      <div className="mt-8">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Username</TableHead>
              <TableHead>Role</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Created</TableHead>
              <TableHead className="w-[50px]"></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              // Loading skeleton rows
              Array.from({ length: 4 }).map((_, i) => (
                <TableRow key={i} data-testid="skeleton-row">
                  <TableCell>
                    <div className="h-4 w-24 animate-pulse rounded bg-muted" />
                  </TableCell>
                  <TableCell>
                    <div className="h-4 w-16 animate-pulse rounded bg-muted" />
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
            ) : !users || users.length === 0 ? (
              <TableRow>
                <TableCell colSpan={5} className="text-center text-muted-foreground">
                  No users found.
                </TableCell>
              </TableRow>
            ) : (
              users.map((user) => (
                <TableRow key={user.id}>
                  <TableCell>{user.username}</TableCell>
                  <TableCell>
                    <Badge variant="outline">{user.global_role}</Badge>
                  </TableCell>
                  <TableCell>
                    {user.is_active ? (
                      <Badge variant="default">Active</Badge>
                    ) : (
                      <Badge variant="secondary">Inactive</Badge>
                    )}
                  </TableCell>
                  <TableCell>{formatDate(user.created_at)}</TableCell>
                  <TableCell>
                    {user.username === currentUsername ? null : (
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
                          {user.is_active ? (
                            <DropdownMenuItem
                              className="text-destructive"
                              onClick={() => setDeactivateUser({ id: user.id, username: user.username })}
                            >
                              Deactivate
                            </DropdownMenuItem>
                          ) : (
                            <DropdownMenuItem onClick={() => handleReactivate(user)}>
                              Reactivate
                            </DropdownMenuItem>
                          )}
                          <DropdownMenuItem
                            onClick={() => setResetUser({ id: user.id, username: user.username })}
                          >
                            Reset Password
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

      {/* Dialogs */}
      <CreateUserDialog open={createOpen} onOpenChange={setCreateOpen} />
      <DeactivateDialog
        open={!!deactivateUser}
        onOpenChange={(open) => {
          if (!open) setDeactivateUser(null)
        }}
        user={deactivateUser}
      />
      <ResetPasswordDialog
        open={!!resetUser}
        onOpenChange={(open) => {
          if (!open) setResetUser(null)
        }}
        user={resetUser}
      />
    </div>
  )
}
