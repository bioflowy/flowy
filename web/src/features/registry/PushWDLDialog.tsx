import { useRef } from 'react'
import { useForm } from 'react-hook-form'
import { z } from 'zod'
import { zodResolver } from '@hookform/resolvers/zod'
import { Loader2 } from 'lucide-react'
import { toast } from 'sonner'

import { usePushWDL } from '@/features/registry/useRegistry'
import { ApiError } from '@/lib/apiClient'
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

const pushWDLSchema = z.object({
  name: z
    .string()
    .min(1, 'Entry name is required')
    .max(64)
    .regex(/^[a-z0-9-]+$/, 'Only lowercase letters, numbers, and hyphens allowed'),
  file: z
    .any()
    .refine((fl: unknown) => fl instanceof FileList && fl.length > 0, 'Select a file')
    .refine(
      (fl: unknown) => fl instanceof FileList && fl.length > 0 && fl[0].name.endsWith('.wdl'),
      'Only .wdl files are accepted. Please select a valid WDL file.'
    ),
})

type PushWDLFormValues = z.infer<typeof pushWDLSchema>

interface PushWDLDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  slug: string
}

function readFileAsText(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = (e) => resolve(e.target?.result as string)
    reader.onerror = () => reject(new Error('Failed to read file'))
    reader.readAsText(file)
  })
}

export function PushWDLDialog({ open, onOpenChange, slug }: PushWDLDialogProps) {
  const pushWDL = usePushWDL(slug)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const form = useForm<PushWDLFormValues>({
    resolver: zodResolver(pushWDLSchema),
    defaultValues: { name: '', file: undefined },
    mode: 'onChange',
  })

  const isPending = form.formState.isSubmitting || pushWDL.isPending

  function handleOpenChange(nextOpen: boolean) {
    if (!nextOpen) {
      form.reset()
      if (fileInputRef.current) {
        fileInputRef.current.value = ''
      }
    }
    onOpenChange(nextOpen)
  }

  async function onSubmit(data: PushWDLFormValues) {
    const file = (data.file as FileList)[0]
    try {
      const content = await readFileAsText(file)
      const result = await pushWDL.mutateAsync({ name: data.name, content })
      const shortDigest = result.digest.startsWith('sha256:')
        ? result.digest.slice(7, 23) + '...'
        : result.digest
      toast.success(`${result.name} pushed. Digest: ${shortDigest}`)
      form.reset()
      if (fileInputRef.current) {
        fileInputRef.current.value = ''
      }
      onOpenChange(false)
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.status === 422) {
          toast.error(`Invalid WDL: ${err.message}`)
        } else if (err.status === 409) {
          toast.error('Name already exists with a different kind. Choose a different entry name.')
        } else {
          toast.error('Something went wrong. Please try again.')
        }
      } else {
        toast.error('Something went wrong. Please try again.')
      }
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Push WDL to Registry</DialogTitle>
          <DialogDescription>
            Upload a .wdl file to add a new registry entry to this project.
          </DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
            <FormField
              control={form.control}
              name="name"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Entry name</FormLabel>
                  <FormControl>
                    <Input
                      autoFocus
                      placeholder="e.g. align-reads, variant-call"
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="file"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>WDL file</FormLabel>
                  <FormControl>
                    <Input
                      type="file"
                      accept=".wdl"
                      ref={fileInputRef}
                      onChange={(e) => {
                        const files = e.target.files
                        field.onChange(files)
                        // Manually validate file extension for immediate feedback
                        if (files && files.length > 0) {
                          if (!files[0].name.endsWith('.wdl')) {
                            form.setError('file', {
                              type: 'manual',
                              message:
                                'Only .wdl files are accepted. Please select a valid WDL file.',
                            })
                          } else {
                            form.clearErrors('file')
                          }
                        } else {
                          form.setError('file', { type: 'manual', message: 'Select a file' })
                        }
                      }}
                    />
                  </FormControl>
                  <FormMessage />
                  <p className="text-sm text-muted-foreground">Only .wdl files are accepted.</p>
                </FormItem>
              )}
            />

            <DialogFooter className="flex flex-col gap-2 sm:flex-row">
              <Button
                type="button"
                variant="ghost"
                disabled={isPending}
                onClick={() => handleOpenChange(false)}
              >
                Discard
              </Button>
              <Button
                type="submit"
                disabled={isPending || !form.formState.isValid}
              >
                {isPending ? (
                  <>
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    Pushing...
                  </>
                ) : (
                  'Push WDL'
                )}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
