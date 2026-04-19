import { useState } from 'react'
import { Loader2 } from 'lucide-react'
import { toast } from 'sonner'

import { useImportManagedFile } from '@/features/projects/useProjects'
import { ApiError } from '@/lib/apiClient'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { cn } from '@/lib/utils'

type ImportMode = 'browser' | 's3' | 'local'

interface ImportFileDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  slug: string
}

const modeLabels: Record<ImportMode, string> = {
  browser: 'Browser Upload',
  s3: 'S3 URI',
  local: 'Server Path',
}

export function ImportFileDialog({ open, onOpenChange, slug }: ImportFileDialogProps) {
  const importManagedFile = useImportManagedFile(slug)
  const [mode, setMode] = useState<ImportMode>('browser')
  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  const [uri, setURI] = useState('')
  const [fileType, setFileType] = useState('')

  const isPending = importManagedFile.isPending

  function resetForm() {
    setMode('browser')
    setSelectedFile(null)
    setURI('')
    setFileType('')
  }

  function handleOpenChange(nextOpen: boolean) {
    if (!nextOpen) {
      resetForm()
    }
    onOpenChange(nextOpen)
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()

    try {
      if (mode === 'browser') {
        if (!selectedFile) {
          toast.error('Choose a file to upload.')
          return
        }
        await importManagedFile.mutateAsync({ file: selectedFile, fileType })
      } else {
        const trimmed = uri.trim()
        if (!trimmed) {
          toast.error('Enter a path to import.')
          return
        }
        await importManagedFile.mutateAsync({ uri: trimmed, fileType })
      }

      toast.success('File imported.')
      handleOpenChange(false)
    } catch (err) {
      if (err instanceof ApiError && err.status === 400) {
        toast.error(err.message)
      } else {
        toast.error('Import failed. Please try again.')
      }
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Import File</DialogTitle>
          <DialogDescription>
            Add a file to this project&apos;s managed library from the browser, S3, or a server path.
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="grid grid-cols-1 gap-2 sm:grid-cols-3">
            {(['browser', 's3', 'local'] as ImportMode[]).map((option) => (
              <button
                key={option}
                type="button"
                className={cn(
                  'rounded-md border px-3 py-2 text-left text-sm transition-colors',
                  mode === option
                    ? 'border-primary bg-primary/5 text-foreground'
                    : 'border-input text-muted-foreground hover:text-foreground'
                )}
                onClick={() => setMode(option)}
              >
                {modeLabels[option]}
              </button>
            ))}
          </div>

          {mode === 'browser' ? (
            <div className="space-y-2">
              <label className="text-sm font-medium" htmlFor="import-browser-file">
                File
              </label>
              <Input
                id="import-browser-file"
                type="file"
                onChange={(e) => setSelectedFile(e.target.files?.[0] ?? null)}
              />
              <p className="text-xs text-muted-foreground">
                Uploads are stored in the project upload area and registered automatically.
              </p>
            </div>
          ) : (
            <div className="space-y-2">
              <label className="text-sm font-medium" htmlFor="import-uri">
                {mode === 's3' ? 'S3 URI' : 'Server Path'}
              </label>
              <Input
                id="import-uri"
                value={uri}
                onChange={(e) => setURI(e.target.value)}
                placeholder={
                  mode === 's3'
                    ? 's3://bucket/path/to/file.fastq.gz'
                    : '/srv/imports/path/to/file.fastq.gz'
                }
              />
              <p className="text-xs text-muted-foreground">
                {mode === 's3'
                  ? 'The object must be reachable through the configured storage backend.'
                  : 'Local imports must stay inside the configured import_root.'}
              </p>
            </div>
          )}

          <div className="space-y-2">
            <label className="text-sm font-medium" htmlFor="import-file-type">
              File Type
            </label>
            <Input
              id="import-file-type"
              value={fileType}
              onChange={(e) => setFileType(e.target.value)}
              placeholder="e.g. fastq, bam, report"
            />
            <p className="text-xs text-muted-foreground">
              Optional. This semantic type is used by the managed file picker filter.
            </p>
          </div>

          <DialogFooter className="flex flex-col gap-2 sm:flex-row">
            <Button type="button" variant="ghost" onClick={() => handleOpenChange(false)}>
              Discard
            </Button>
            <Button type="submit" disabled={isPending}>
              {isPending ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Importing...
                </>
              ) : (
                'Import File'
              )}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
