import { useState } from 'react'
import { useParams, useNavigate } from 'react-router'
import { MoreHorizontal, Loader2 } from 'lucide-react'
import { toast } from 'sonner'

import { useRegistryEntries, useDeleteRegistryEntry } from '@/features/registry/useRegistry'
import { useProjectMembers } from '@/features/projects/useProjects'
import { useAuthStore } from '@/store/authStore'
import { Badge } from '@/components/ui/badge'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Button } from '@/components/ui/button'
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

function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  })
}

export function RegistryPage() {
  const { slug } = useParams<{ slug: string }>()
  const navigate = useNavigate()

  const { data: entries, isLoading, isError } = useRegistryEntries(slug!)
  const { data: members } = useProjectMembers(slug!)
  const currentUsername = useAuthStore((s) => s.username)
  const deleteEntry = useDeleteRegistryEntry(slug!)

  const isProjectAdmin =
    members?.find((m) => m.username === currentUsername)?.role === 'admin'

  const [deleteEntryName, setDeleteEntryName] = useState<string | null>(null)
  const [isDeletingEntry, setIsDeletingEntry] = useState(false)

  async function handleDeleteEntry(e: React.MouseEvent) {
    e.preventDefault()
    if (!deleteEntryName) return

    setIsDeletingEntry(true)
    try {
      await deleteEntry.mutateAsync(deleteEntryName)
      toast.success(`${deleteEntryName} deleted.`)
      setDeleteEntryName(null)
    } catch {
      toast.error('Something went wrong. Please try again.')
    } finally {
      setIsDeletingEntry(false)
    }
  }

  return (
    <div className="p-6">
      {/* Page header */}
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">Registry</h1>
      </div>

      {/* Table */}
      <div className="mt-8">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Kind</TableHead>
              <TableHead>Versions</TableHead>
              <TableHead>Tags</TableHead>
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
                    <div className="h-4 w-12 animate-pulse rounded bg-muted" />
                  </TableCell>
                  <TableCell>
                    <div className="h-4 w-32 animate-pulse rounded bg-muted" />
                  </TableCell>
                  <TableCell>
                    <div className="h-4 w-20 animate-pulse rounded bg-muted" />
                  </TableCell>
                  <TableCell />
                </TableRow>
              ))
            ) : isError ? (
              <TableRow>
                <TableCell colSpan={6} className="text-center text-muted-foreground">
                  Failed to load registry. Please refresh the page.
                </TableCell>
              </TableRow>
            ) : !entries || entries.length === 0 ? (
              <TableRow>
                <TableCell colSpan={6} className="text-center text-muted-foreground">
                  No registry entries.
                  <p className="text-xs text-muted-foreground mt-1">
                    Push a WDL workflow or task using the flowy CLI to see it here.
                  </p>
                </TableCell>
              </TableRow>
            ) : (
              entries.map((entry) => (
                <TableRow
                  key={entry.name}
                  className="cursor-pointer hover:bg-muted/50"
                  onClick={() => navigate(`/projects/${slug}/registry/${entry.name}`)}
                >
                  <TableCell>{entry.name}</TableCell>
                  <TableCell>
                    <Badge
                      variant="outline"
                      className={
                        entry.kind === 'workflow'
                          ? 'border-purple-300 text-purple-700 bg-purple-50'
                          : 'border-blue-300 text-blue-700 bg-blue-50'
                      }
                    >
                      {entry.kind}
                    </Badge>
                  </TableCell>
                  <TableCell>{entry.version_count}</TableCell>
                  <TableCell>
                    {entry.tags.length > 0 ? entry.tags.join(', ') : '\u2014'}
                  </TableCell>
                  <TableCell>{formatDate(entry.created_at)}</TableCell>
                  <TableCell>
                    {isProjectAdmin && (
                      <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-8 w-8"
                            aria-label="Open menu"
                            onClick={(e) => e.stopPropagation()}
                          >
                            <MoreHorizontal className="h-4 w-4" />
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end">
                          <DropdownMenuItem
                            className="text-destructive"
                            onClick={(e) => {
                              e.stopPropagation()
                              setDeleteEntryName(entry.name)
                            }}
                          >
                            Delete
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

      {/* Delete registry entry AlertDialog */}
      <AlertDialog
        open={!!deleteEntryName}
        onOpenChange={(open) => {
          if (!open) setDeleteEntryName(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete registry entry</AlertDialogTitle>
            <AlertDialogDescription>
              This will permanently delete &quot;{deleteEntryName}&quot; and all its versions. This
              action cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel asChild>
              <Button variant="outline">Keep Entry</Button>
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={handleDeleteEntry}
              disabled={isDeletingEntry}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              {isDeletingEntry ? (
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
    </div>
  )
}
