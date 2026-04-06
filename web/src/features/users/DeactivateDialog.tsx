import { useState } from 'react'
import { Loader2 } from 'lucide-react'
import { toast } from 'sonner'

import { useDeactivateUser } from '@/features/users/useUsers'
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

interface DeactivateDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  user: { id: string; username: string } | null
}

export function DeactivateDialog({ open, onOpenChange, user }: DeactivateDialogProps) {
  const deactivate = useDeactivateUser()
  const [isLoading, setIsLoading] = useState(false)

  async function handleConfirm(e: React.MouseEvent) {
    e.preventDefault()
    if (!user) return

    setIsLoading(true)
    try {
      await deactivate.mutateAsync(user.id)
      toast.success(`${user.username} deactivated.`)
      onOpenChange(false)
    } catch {
      toast.error('Something went wrong. Please try again.')
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Deactivate {user?.username}?</AlertDialogTitle>
          <AlertDialogDescription>
            They will no longer be able to log in.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction
            onClick={handleConfirm}
            disabled={isLoading}
            className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
          >
            {isLoading ? (
              <>
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                Deactivating...
              </>
            ) : (
              'Deactivate'
            )}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
