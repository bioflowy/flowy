import { useForm } from 'react-hook-form'
import { z } from 'zod'
import { zodResolver } from '@hookform/resolvers/zod'
import { Loader2 } from 'lucide-react'
import { toast } from 'sonner'

import { useUpsertTag } from '@/features/registry/useRegistry'
import type { RegistryVersionResponse } from '@/features/registry/useRegistry'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Dialog,
  DialogContent,
  DialogDescription,
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

const tagSchema = z.object({
  tag: z
    .string()
    .min(1, 'Tag name is required')
    .max(128)
    .refine((v) => !v.includes('@') && !v.includes(':'), {
      message: "Tag must not contain '@' or ':'",
    }),
  digest: z.string().min(1, 'Select a version'),
})

type TagFormValues = z.infer<typeof tagSchema>

function truncateDigest(digest: string): string {
  if (digest.startsWith('sha256:')) return `sha256:${digest.slice(7, 23)}...`
  return digest
}

interface CreateTagDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  slug: string
  name: string
  versions: RegistryVersionResponse[]
}

export function CreateTagDialog({ open, onOpenChange, slug, name, versions }: CreateTagDialogProps) {
  const upsertTag = useUpsertTag(slug, name)

  const form = useForm<TagFormValues>({
    resolver: zodResolver(tagSchema),
    defaultValues: {
      tag: '',
      digest: '',
    },
  })

  const isPending = form.formState.isSubmitting || upsertTag.isPending

  function handleOpenChange(nextOpen: boolean) {
    if (!nextOpen) {
      form.reset()
    }
    onOpenChange(nextOpen)
  }

  async function onSubmit(data: TagFormValues) {
    try {
      await upsertTag.mutateAsync({ tag: data.tag, digest: data.digest })
      toast.success(`Tag "${data.tag}" saved.`)
      form.reset()
      onOpenChange(false)
    } catch {
      toast.error('Something went wrong. Please try again.')
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Create or Move Tag</DialogTitle>
          <DialogDescription>
            Assign a tag to a specific version. If the tag already exists on another version, it
            will be moved.
          </DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
            <FormField
              control={form.control}
              name="tag"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Tag name</FormLabel>
                  <FormControl>
                    <Input placeholder="e.g. latest, v1.0.0" {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="digest"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Version</FormLabel>
                  <Select onValueChange={field.onChange} value={field.value}>
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue placeholder="Select a version" />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      {versions.map((v) => (
                        <SelectItem key={v.id} value={v.digest}>
                          {truncateDigest(v.digest)}
                          {v.tags.length > 0 ? ` [${v.tags.join(', ')}]` : ''}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />

            <DialogFooter className="flex flex-col gap-2 sm:flex-row">
              <Button
                type="button"
                variant="ghost"
                onClick={() => {
                  form.reset()
                  onOpenChange(false)
                }}
              >
                Discard
              </Button>
              <Button type="submit" disabled={isPending}>
                {isPending ? (
                  <>
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    Saving...
                  </>
                ) : (
                  'Save Tag'
                )}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
