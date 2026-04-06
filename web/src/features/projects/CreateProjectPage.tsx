import { useState } from 'react'
import { useNavigate } from 'react-router'
import { useForm } from 'react-hook-form'
import { z } from 'zod'
import { zodResolver } from '@hookform/resolvers/zod'
import { Loader2 } from 'lucide-react'

import { ApiError } from '@/lib/apiClient'
import { useCreateProject } from '@/features/projects/useProjects'
import { useProjectStore } from '@/store/projectStore'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'

const createProjectSchema = z.object({
  name: z.string().min(1, 'Name is required.'),
  slug: z
    .string()
    .min(1, 'Slug is required.')
    .max(64)
    .regex(
      /^[a-z0-9]([a-z0-9-]{0,62}[a-z0-9])?$/,
      'Lowercase alphanumeric and hyphens only, no leading/trailing hyphens.'
    ),
  description: z.string().optional(),
})

type CreateProjectFormValues = z.infer<typeof createProjectSchema>

function generateSlug(name: string): string {
  return name
    .toLowerCase()
    .trim()
    .replace(/\s+/g, '-')
    .replace(/[^a-z0-9-]/g, '')
    .replace(/^-+|-+$/g, '')
    .substring(0, 64)
}

export function CreateProjectPage() {
  const navigate = useNavigate()
  const setActiveProject = useProjectStore((s) => s.setActiveProject)
  const createProject = useCreateProject()
  const [slugManuallyEdited, setSlugManuallyEdited] = useState(false)

  const form = useForm<CreateProjectFormValues>({
    resolver: zodResolver(createProjectSchema),
    defaultValues: {
      name: '',
      slug: '',
      description: '',
    },
  })

  const { isSubmitting } = form.formState
  const isPending = isSubmitting || createProject.isPending

  async function onSubmit(values: CreateProjectFormValues) {
    try {
      await createProject.mutateAsync({
        slug: values.slug,
        description: values.description || '',
      })
      setActiveProject(values.slug)
      navigate(`/projects/${values.slug}`)
    } catch (error) {
      form.setError('root', {
        message:
          error instanceof ApiError ? error.message : 'Something went wrong. Please try again.',
      })
    }
  }

  return (
    <div className="p-6">
      <h1 className="text-xl font-semibold">Create Project</h1>

      <Form {...form}>
        <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4 max-w-md mt-6">
          <FormField
            control={form.control}
            name="name"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Name</FormLabel>
                <FormControl>
                  <Input
                    type="text"
                    placeholder="My Project"
                    {...field}
                    onChange={(e) => {
                      field.onChange(e)
                      if (!slugManuallyEdited) {
                        form.setValue('slug', generateSlug(e.target.value))
                      }
                    }}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="slug"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Slug</FormLabel>
                <FormControl>
                  <Input
                    type="text"
                    placeholder="my-project"
                    {...field}
                    onChange={(e) => {
                      setSlugManuallyEdited(true)
                      field.onChange(e)
                    }}
                  />
                </FormControl>
                <p className="text-xs text-muted-foreground">
                  Lowercase letters, numbers, and hyphens only.
                </p>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="description"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Description</FormLabel>
                <FormControl>
                  <Textarea
                    rows={3}
                    placeholder="A brief description of this project."
                    {...field}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          {form.formState.errors.root && (
            <p className="text-sm text-destructive">{form.formState.errors.root.message}</p>
          )}

          <div className="flex gap-2">
            <Button
              type="button"
              variant="ghost"
              onClick={() => navigate('/projects')}
            >
              Cancel
            </Button>
            <Button type="submit" variant="default" disabled={isPending}>
              {isPending ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Creating...
                </>
              ) : (
                'Create'
              )}
            </Button>
          </div>
        </form>
      </Form>
    </div>
  )
}
