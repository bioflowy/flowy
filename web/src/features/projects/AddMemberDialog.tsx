import { useForm } from 'react-hook-form'
import { z } from 'zod'
import { zodResolver } from '@hookform/resolvers/zod'
import { Loader2 } from 'lucide-react'
import { toast } from 'sonner'

import { useAddMember } from '@/features/projects/useProjects'
import { useUsers } from '@/features/users/useUsers'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

const addMemberSchema = z.object({
  username: z.string().min(1, 'Username is required.'),
  role: z.enum(['admin', 'executor', 'viewer']),
})

type AddMemberFormValues = z.infer<typeof addMemberSchema>

interface AddMemberDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  slug: string
  existingMembers: string[]
}

export function AddMemberDialog({ open, onOpenChange, slug, existingMembers }: AddMemberDialogProps) {
  const addMember = useAddMember(slug)
  const { data: users } = useUsers()

  const form = useForm<AddMemberFormValues>({
    resolver: zodResolver(addMemberSchema),
    defaultValues: {
      username: '',
      role: 'viewer',
    },
  })

  const { isSubmitting } = form.formState
  const isPending = isSubmitting || addMember.isPending

  // Filter out users who are already members
  const availableUsers = users?.filter((u) => !existingMembers.includes(u.username)) ?? []

  async function onSubmit(values: AddMemberFormValues) {
    try {
      await addMember.mutateAsync({ username: values.username, role: values.role })
      toast.success('Member added.')
      form.reset()
      onOpenChange(false)
    } catch (error) {
      const message = error instanceof Error ? error.message.toLowerCase() : ''
      if (message.includes('not found')) {
        form.setError('root', { message: 'User not found.' })
      } else if (message.includes('already')) {
        form.setError('root', { message: 'User is already a member of this project.' })
      } else {
        form.setError('root', { message: 'Something went wrong. Please try again.' })
      }
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Add Member</DialogTitle>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
            <FormField
              control={form.control}
              name="username"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Username</FormLabel>
                  <Select onValueChange={field.onChange} value={field.value}>
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue placeholder="Select a user" />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      {availableUsers.map((user) => (
                        <SelectItem key={user.id} value={user.username}>
                          {user.username}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="role"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Role</FormLabel>
                  <Select onValueChange={field.onChange} defaultValue={field.value}>
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue placeholder="Select a role" />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      <SelectItem value="admin">admin</SelectItem>
                      <SelectItem value="executor">executor</SelectItem>
                      <SelectItem value="viewer">viewer</SelectItem>
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />

            {form.formState.errors.root && (
              <p className="text-sm text-destructive">{form.formState.errors.root.message}</p>
            )}

            <DialogFooter className="flex flex-col gap-2 sm:flex-row">
              <Button
                type="button"
                variant="ghost"
                onClick={() => onOpenChange(false)}
              >
                Never Mind
              </Button>
              <Button type="submit" disabled={isPending}>
                {isPending ? (
                  <>
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    Adding...
                  </>
                ) : (
                  'Add Member'
                )}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
