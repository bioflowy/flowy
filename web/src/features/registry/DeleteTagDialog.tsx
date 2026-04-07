import { useState } from 'react'
import { Loader2 } from 'lucide-react'
import { toast } from 'sonner'

import { useDeleteTag } from '@/features/registry/useRegistry'
import { Button } from '@/components/ui/button'
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

interface DeleteTagDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  slug: string
  entryName: string
  tagName: string | null
}

export function DeleteTagDialog({ open, onOpenChange, slug, entryName, tagName }: DeleteTagDialogProps) {
  const deleteTag = useDeleteTag(slug, entryName)
  const [isDeleting, setIsDeleting] = useState(false)

  async function handleDelete(e: React.MouseEvent) {
    e.preventDefault()
    if (!tagName) return

    setIsDeleting(true)
    try {
      await deleteTag.mutateAsync(tagName)
      toast.success(`Tag "${tagName}" deleted.`)
      onOpenChange(false)
    } catch {
      toast.error('Something went wrong. Please try again.')
    } finally {
      setIsDeleting(false)
    }
  }

  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete tag</AlertDialogTitle>
          <AlertDialogDescription>
            This will permanently remove the tag &quot;{tagName}&quot; from this registry entry.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel asChild>
            <Button variant="outline">Keep Tag</Button>
          </AlertDialogCancel>
          <AlertDialogAction
            onClick={handleDelete}
            disabled={isDeleting}
            className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
          >
            {isDeleting ? (
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
  )
}
