import { useState, useEffect, useMemo } from 'react'
import { useForm } from 'react-hook-form'
import { z } from 'zod'
import { zodResolver } from '@hookform/resolvers/zod'
import { Loader2, Info } from 'lucide-react'
import { toast } from 'sonner'
import { useNavigate } from 'react-router'

import {
  useRegistryInputs,
  useRunWorkflow,
} from '@/features/registry/useRegistry'
import { useManagedFiles } from '@/features/projects/useProjects'
import type { RegistryVersionResponse } from '@/features/registry/useRegistry'
import { ApiError } from '@/lib/apiClient'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Checkbox } from '@/components/ui/checkbox'
import { Separator } from '@/components/ui/separator'
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
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'

interface RunWorkflowDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  slug: string
  name: string
  versions: RegistryVersionResponse[]
}

function truncateDigest(digest: string): string {
  if (digest.startsWith('sha256:')) return `sha256:${digest.slice(7, 23)}...`
  return digest
}

function getDefaultTag(versions: RegistryVersionResponse[]): string {
  if (versions.length === 0) return ''
  for (const v of versions) {
    if (v.tags.includes('latest')) return 'latest'
  }
  // Fall back to first tag, or digest if version is tagless
  return versions[0].tags[0] ?? versions[0].digest
}

function getDigestForTag(versions: RegistryVersionResponse[], tagOrDigest: string): string {
  for (const v of versions) {
    if (v.tags.includes(tagOrDigest)) return v.digest
    // Tagless versions are selected by their full digest
    if (v.digest === tagOrDigest) return v.digest
  }
  return ''
}

function isComplexType(type: string): boolean {
  return type.startsWith('Array') || type.startsWith('Map') || type.startsWith('Struct')
}

function filterManagedFiles(
  files: { id: string; path: string; label: string; file_type: string; source_type: string; run_id?: string }[],
  requiredType: string | null
) {
  if (!requiredType) return files
  return files.filter((file) => file.file_type === requiredType)
}

export function RunWorkflowDialog({
  open,
  onOpenChange,
  slug,
  name,
  versions,
}: RunWorkflowDialogProps) {
  const navigate = useNavigate()
  const runWorkflow = useRunWorkflow(slug)
  const { data: managedFiles, isLoading: managedFilesLoading } = useManagedFiles(slug)

  const defaultTag = useMemo(() => getDefaultTag(versions), [versions])
  const [selectedTag, setSelectedTag] = useState(defaultTag)

  // Reset selectedTag when defaultTag changes (e.g., versions prop changes)
  useEffect(() => {
    setSelectedTag(defaultTag)
  }, [defaultTag])

  const {
    data: inputsData,
    isLoading: inputsLoading,
    isError: inputsError,
  } = useRegistryInputs(slug, name, selectedTag)

  // Build dynamic zod schema from inputs
  const schema = useMemo(() => {
    if (!inputsData?.inputs) return z.object({})

    const shape: Record<string, z.ZodTypeAny> = {}
    for (const input of inputsData.inputs) {
      let fieldSchema: z.ZodTypeAny

      if (input.type === 'Boolean') {
        fieldSchema = z.boolean()
      } else if (input.type === 'Int') {
        fieldSchema = z.coerce.number().int(`${input.name} must be an integer`)
      } else if (input.type === 'Float') {
        fieldSchema = z.coerce.number({ message: `${input.name} must be a number` })
      } else if (isComplexType(input.type)) {
        fieldSchema = z.string().min(1).refine(
          (v) => {
            try {
              JSON.parse(v)
              return true
            } catch {
              return false
            }
          },
          `${input.name} must be valid JSON`
        )
      } else {
        // String, File, and other types
        fieldSchema = z.string().min(1, `${input.name} is required`)
      }

      if (input.optional) {
        if (input.type === 'Boolean') {
          fieldSchema = fieldSchema.optional()
        } else if (input.type === 'Int' || input.type === 'Float') {
          // Preprocess: if the raw value is an empty string, treat as undefined so
          // optional() takes effect. This avoids z.coerce.number("") === 0 false positive.
          const numSchema = input.type === 'Int'
            ? z.coerce.number().int(`${input.name} must be an integer`)
            : z.coerce.number({ message: `${input.name} must be a number` })
          fieldSchema = z.preprocess(
            (v) => (v === '' || v === undefined ? undefined : v),
            numSchema.optional()
          )
        } else {
          fieldSchema = fieldSchema.optional().or(z.literal(''))
        }
      }

      shape[input.name] = fieldSchema
    }

    return z.object(shape)
  }, [inputsData?.inputs])

  // Build default values from inputs
  const computedDefaults = useMemo(() => {
    if (!inputsData?.inputs) return {}
    const defaults: Record<string, unknown> = {}
    for (const input of inputsData.inputs) {
      if (input.type === 'Boolean') {
        defaults[input.name] = input.default === 'true'
      } else {
        defaults[input.name] = input.default ?? ''
      }
    }
    return defaults
  }, [inputsData?.inputs])

  const form = useForm<Record<string, unknown>>({
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    resolver: zodResolver(schema) as any,
    defaultValues: computedDefaults,
  })

  // Reset form when inputs data changes (version/tag change)
  useEffect(() => {
    form.reset(computedDefaults)
  }, [computedDefaults]) // eslint-disable-line react-hooks/exhaustive-deps

  const isPending = form.formState.isSubmitting || runWorkflow.isPending

  function handleOpenChange(nextOpen: boolean) {
    if (!nextOpen) {
      form.reset()
      setSelectedTag(defaultTag)
    }
    onOpenChange(nextOpen)
  }

  async function onSubmit(data: Record<string, unknown>) {
    const inputs: Record<string, unknown> = {}

    if (inputsData?.inputs) {
      for (const input of inputsData.inputs) {
        const value = data[input.name]
        // Skip empty optional fields
        if (input.optional && (value === '' || value === undefined || value === null)) {
          continue
        }
        if (isComplexType(input.type) && typeof value === 'string' && value !== '') {
          inputs[input.name] = JSON.parse(value)
        } else {
          inputs[input.name] = value
        }
      }
    }

    try {
      const result = await runWorkflow.mutateAsync({
        registry_ref: `${name}:${selectedTag}`,
        inputs,
      })
      toast.success('Run submitted.')
      form.reset()
      onOpenChange(false)
      navigate(`/projects/${slug}/runs/${result.run_id}`)
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.status === 400) {
          toast.error('Invalid inputs. Check the values and try again.')
        } else if (err.status === 404) {
          toast.error('Workflow version not found.')
        } else {
          toast.error('Server error. Please try again.')
        }
      } else {
        toast.error('Server error. Please try again.')
      }
    }
  }

  const selectedDigest = getDigestForTag(versions, selectedTag)

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Run Workflow</DialogTitle>
          <DialogDescription>Configure inputs and submit execution.</DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
            {/* Version selector */}
            <div className="space-y-2">
              <label className="text-sm font-medium">Version</label>
              <Select value={selectedTag} onValueChange={setSelectedTag}>
                <SelectTrigger>
                  <SelectValue placeholder="Select a version" />
                </SelectTrigger>
                <SelectContent>
                  {versions.map((v) => {
                    if (v.tags.length > 0) {
                      return v.tags.map((tag) => (
                        <SelectItem key={tag} value={tag}>
                          {tag}
                        </SelectItem>
                      ))
                    }
                    // Tagless version — use digest as both key and value
                    const shortDigest = truncateDigest(v.digest)
                    return (
                      <SelectItem key={v.digest} value={v.digest}>
                        {shortDigest}
                      </SelectItem>
                    )
                  })}
                </SelectContent>
              </Select>
              {selectedDigest && (
                <p className="text-xs text-muted-foreground">{truncateDigest(selectedDigest)}</p>
              )}
            </div>

            <Separator />

            {/* Dynamic input form fields */}
            {inputsLoading && (
              <div className="space-y-4">
                {Array.from({ length: 3 }).map((_, i) => (
                  <div key={i}>
                    <div className="h-3 w-24 animate-pulse rounded bg-muted" />
                    <div className="h-4 w-full animate-pulse rounded bg-muted mt-2" />
                  </div>
                ))}
              </div>
            )}

            {inputsError && (
              <p className="text-muted-foreground text-sm">
                Failed to load workflow inputs. Please close and try again.
              </p>
            )}

            {!inputsLoading && !inputsError && inputsData?.inputs?.length === 0 && (
              <p className="text-muted-foreground text-sm">
                This workflow declares no inputs. Click Run Workflow to execute with defaults.
              </p>
            )}

            {!inputsLoading && !inputsError && inputsData?.inputs && inputsData.inputs.length > 0 && (
              <div className="space-y-4">
                {inputsData.inputs.map((input) => (
                  <FormField
                    key={input.name}
                    control={form.control}
                    name={input.name}
                    render={({ field }) => (
                      <FormItem>
                        {input.type === 'Boolean' ? (
                          <div className="flex flex-row items-start space-x-3 space-y-0">
                            <FormControl>
                              <Checkbox
                                checked={field.value as boolean}
                                onCheckedChange={field.onChange}
                              />
                            </FormControl>
                            <FormLabel>
                              {input.name}
                              {input.optional && (
                                <span className="text-muted-foreground"> (optional)</span>
                              )}
                            </FormLabel>
                          </div>
                        ) : (
                          <>
                            <FormLabel>
                              {input.name}
                              {input.optional && (
                                <span className="text-muted-foreground"> (optional)</span>
                              )}
                              {input.type === 'File' && (
                                <TooltipProvider>
                                  <Tooltip>
                                    <TooltipTrigger asChild>
                                      <Info data-testid="file-input-tooltip-trigger" className="ml-1 inline h-4 w-4 text-muted-foreground cursor-help" />
                                    </TooltipTrigger>
                                    <TooltipContent>
                                      <p>
                                        Enter a server-accessible path. Browser file paths are not
                                        supported. Use s3://, /absolute/path, or another path the
                                        server can resolve.
                                      </p>
                                    </TooltipContent>
                                  </Tooltip>
                                </TooltipProvider>
                              )}
                            </FormLabel>
                            <FormControl>
                              {isComplexType(input.type) ? (
                                <Textarea
                                  placeholder="Enter valid JSON"
                                  {...field}
                                  value={field.value as string}
                                />
                              ) : input.type === 'Int' ? (
                                <Input
                                  type="number"
                                  step="1"
                                  placeholder="Enter an integer"
                                  {...field}
                                  value={field.value as string | number}
                                />
                              ) : input.type === 'Float' ? (
                                <Input
                                  type="number"
                                  step="any"
                                  placeholder="Enter a decimal number"
                                  {...field}
                                  value={field.value as string | number}
                                />
                              ) : input.type === 'File' ? (
                                <div className="space-y-2">
                                  {(() => {
                                    const availableFiles = filterManagedFiles(
                                      managedFiles ?? [],
                                      input.file_type
                                    )
                                    const selectedManagedFile = availableFiles.find(
                                      (file) => file.path === field.value
                                    )

                                    return (
                                  <Select
                                    value={
                                      typeof field.value === 'string' &&
                                      availableFiles.some((file) => file.path === field.value)
                                        ? field.value
                                        : ''
                                    }
                                    onValueChange={(value) => field.onChange(value)}
                                  >
                                    <SelectTrigger aria-label={`Managed file picker for ${input.name}`}>
                                      {selectedManagedFile ? (
                                        <div className="flex min-w-0 flex-col items-start text-left">
                                          <span className="w-full truncate">
                                            {selectedManagedFile.label}
                                          </span>
                                          <span className="w-full truncate text-xs text-muted-foreground">
                                            {selectedManagedFile.path}
                                          </span>
                                        </div>
                                      ) : (
                                        <SelectValue
                                          placeholder={
                                            managedFilesLoading
                                              ? 'Loading managed files...'
                                              : 'Select a managed file'
                                          }
                                        />
                                      )}
                                    </SelectTrigger>
                                    <SelectContent>
                                      {availableFiles.length === 0 ? (
                                        <SelectItem value="__none__" disabled>
                                          No matching managed files
                                        </SelectItem>
                                      ) : (
                                        availableFiles.map((file) => (
                                          <SelectItem key={file.id} value={file.path}>
                                            <div className="flex min-w-0 flex-col items-start">
                                              <span className="w-full truncate">{file.label}</span>
                                              <span className="w-full truncate text-xs text-muted-foreground">
                                                {file.path}
                                              </span>
                                            </div>
                                          </SelectItem>
                                        ))
                                      )}
                                    </SelectContent>
                                  </Select>
                                    )
                                  })()}
                                  <Input
                                    type="text"
                                    placeholder="e.g., s3://bucket/path or /absolute/path"
                                    {...field}
                                    value={field.value as string}
                                  />
                                  <p className="text-xs text-muted-foreground">
                                    Pick from the project file library or enter a manual path.
                                    {input.file_type
                                      ? ` Showing only files tagged "${input.file_type}".`
                                      : ''}
                                  </p>
                                </div>
                              ) : (
                                <Input
                                  type="text"
                                  placeholder="Enter a string value"
                                  {...field}
                                  value={field.value as string}
                                />
                              )}
                            </FormControl>
                            {isComplexType(input.type) && (
                              <p className="text-xs text-muted-foreground">
                                Use JSON format, e.g. [&quot;a&quot;, &quot;b&quot;]
                              </p>
                            )}
                            {input.default !== null && (
                              <p className="text-xs text-muted-foreground">
                                (default: {input.default})
                              </p>
                            )}
                          </>
                        )}
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                ))}
              </div>
            )}

            <DialogFooter className="flex flex-col gap-2 sm:flex-row">
              <Button
                type="button"
                variant="ghost"
                onClick={() => handleOpenChange(false)}
              >
                Discard
              </Button>
              <Button
                type="submit"
                disabled={isPending || inputsLoading || !!inputsError}
              >
                {isPending ? (
                  <>
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    Submitting...
                  </>
                ) : (
                  'Run Workflow'
                )}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
