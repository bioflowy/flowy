import { useState } from 'react'
import { useParams } from 'react-router'
import { Loader2 } from 'lucide-react'
import { toast } from 'sonner'

import { useRuns, useRunOutputs } from '@/features/runs/useRuns'
import { useAuthStore } from '@/store/authStore'
import { formatDuration } from '@/features/runs/RunsPage'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'

function statusBadgeClassName(state: string): string {
  switch (state) {
    case 'COMPLETED':
      return 'border-green-300 text-green-700 bg-green-50'
    case 'RUNNING':
      return 'border-yellow-300 text-yellow-700 bg-yellow-50'
    case 'FAILED':
      return 'border-red-300 text-red-700 bg-red-50'
    default:
      return 'border-gray-300 text-gray-500 bg-gray-50'
  }
}

function isFilePath(value: unknown): boolean {
  return (
    typeof value === 'string' &&
    (value.startsWith('/') || value.startsWith('s3://') || value.startsWith('file://'))
  )
}

const PREVIEWABLE_EXTENSIONS = new Set(['.txt', '.log', '.wdl', '.json', '.yaml', '.yml', '.tsv', '.csv'])

function isPreviewable(path: string): boolean {
  const ext = path.slice(path.lastIndexOf('.'))
  return PREVIEWABLE_EXTENSIONS.has(ext)
}

function inferType(value: unknown): string {
  if (Array.isArray(value)) return 'array'
  if (value === null || value === undefined) return 'null'
  return typeof value as string
}

async function fetchPreview(slug: string, runId: string, path: string): Promise<string> {
  const token = useAuthStore.getState().token
  const res = await fetch(
    `/api/projects/${slug}/runs/${runId}/files?path=${encodeURIComponent(path)}&preview=true`,
    { headers: { Authorization: `Bearer ${token}` } }
  )
  if (!res.ok) throw new Error('preview failed')
  return res.text()
}

export function RunDetailPage() {
  const { slug, id } = useParams<{ slug: string; id: string }>()

  const { data: runs } = useRuns(slug ?? '')
  const run = runs?.find((r) => r.id === id)

  const { data: outputsData, isLoading: outputsLoading } = useRunOutputs(
    slug ?? '',
    run?.state === 'COMPLETED' ? (id ?? '') : ''
  )

  const [previewOpen, setPreviewOpen] = useState<Record<string, boolean>>({})
  const [previewContent, setPreviewContent] = useState<Record<string, string | null | 'loading' | 'error'>>({})
  const [downloading, setDownloading] = useState<Record<string, boolean>>({})

  async function handleTogglePreview(key: string, path: string) {
    if (previewOpen[key]) {
      setPreviewOpen((prev) => ({ ...prev, [key]: false }))
      return
    }
    setPreviewOpen((prev) => ({ ...prev, [key]: true }))
    if (previewContent[key] !== undefined) return

    setPreviewContent((prev) => ({ ...prev, [key]: 'loading' }))
    try {
      const text = await fetchPreview(slug ?? '', id ?? '', path)
      setPreviewContent((prev) => ({ ...prev, [key]: text }))
    } catch {
      setPreviewContent((prev) => ({ ...prev, [key]: 'error' }))
    }
  }

  async function handleDownload(path: string) {
    setDownloading((prev) => ({ ...prev, [path]: true }))
    try {
      const token = useAuthStore.getState().token
      const url = `/api/projects/${slug}/runs/${id}/files?path=${encodeURIComponent(path)}`
      const res = await fetch(url, { headers: { Authorization: `Bearer ${token}` } })
      if (!res.ok) throw new Error('download failed')
      const blob = await res.blob()
      const a = document.createElement('a')
      a.href = URL.createObjectURL(blob)
      a.download = path.split('/').pop() ?? 'file'
      a.click()
      URL.revokeObjectURL(a.href)
    } catch {
      toast.error('Download failed. Please try again.')
    } finally {
      setDownloading((prev) => ({ ...prev, [path]: false }))
    }
  }

  if (!run) {
    return (
      <div className="p-6">
        <div className="h-4 w-32 animate-pulse rounded bg-muted" />
      </div>
    )
  }

  const source = run.registry_ref || run.wdl_path

  const outputs = outputsData?.outputs ?? {}
  const outputEntries = Object.entries(outputs)

  function renderOutputsSection() {
    if (run!.state !== 'COMPLETED') {
      let suffix = ''
      if (run!.state === 'RUNNING') suffix = ' This run is currently in progress.'
      else if (run!.state === 'FAILED') suffix = ' This run failed — check CLI logs for details.'
      else if (run!.state === 'PENDING') suffix = ' This run is queued.'

      return (
        <p className="text-muted-foreground text-sm mt-4">
          Outputs are available once the run has completed.{suffix}
        </p>
      )
    }

    if (outputsLoading) {
      return <div className="h-[200px] animate-pulse rounded bg-muted mt-4" />
    }

    return (
      <div className="mt-4">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Output Name</TableHead>
              <TableHead>Type</TableHead>
              <TableHead>Value / Action</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {outputEntries.length === 0 ? (
              <TableRow>
                <TableCell colSpan={3} className="text-center text-muted-foreground">
                  No outputs recorded for this run.
                </TableCell>
              </TableRow>
            ) : (
              outputEntries.flatMap(([key, value]) => {
                const rows = []
                if (isFilePath(value)) {
                  const path = String(value)
                  const filename = path.split('/').pop() ?? path
                  const previewable = isPreviewable(path)
                  const isOpen = previewOpen[key]
                  const content = previewContent[key]

                  rows.push(
                    <TableRow key={key}>
                      <TableCell>{key}</TableCell>
                      <TableCell>
                        <Badge
                          variant="outline"
                          className="border-slate-300 text-slate-600 bg-slate-50"
                        >
                          File
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <span className="font-mono text-sm">{filename}</span>
                        <Button
                          size="sm"
                          variant="default"
                          className="ml-2"
                          disabled={!!downloading[path]}
                          onClick={() => handleDownload(path)}
                        >
                          {downloading[path] ? (
                            <>
                              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                              Downloading...
                            </>
                          ) : (
                            'Download'
                          )}
                        </Button>
                        {previewable && (
                          <Button
                            size="sm"
                            variant="link"
                            onClick={() => handleTogglePreview(key, path)}
                          >
                            {isOpen ? 'Hide Preview' : 'Preview'}
                          </Button>
                        )}
                      </TableCell>
                    </TableRow>
                  )

                  if (isOpen) {
                    rows.push(
                      <TableRow key={`${key}-preview`}>
                        <TableCell colSpan={3}>
                          {content === 'loading' ? (
                            <div className="h-[80px] animate-pulse rounded bg-muted" />
                          ) : content === 'error' ? (
                            <p className="text-sm text-muted-foreground">
                              Failed to load preview. Try downloading the file instead.
                            </p>
                          ) : content !== null && content !== undefined ? (
                            <>
                              <pre className="bg-muted/40 rounded p-4 text-xs font-mono overflow-x-auto whitespace-pre-wrap">
                                {content}
                              </pre>
                              {content.length >= 102400 && (
                                <p className="text-xs text-muted-foreground mt-2 border-t pt-2">
                                  Preview limited to 100 KB. Download the file to view the full content.
                                </p>
                              )}
                            </>
                          ) : null}
                        </TableCell>
                      </TableRow>
                    )
                  }
                } else {
                  rows.push(
                    <TableRow key={key}>
                      <TableCell>{key}</TableCell>
                      <TableCell>
                        <span className="text-xs text-muted-foreground">{inferType(value)}</span>
                      </TableCell>
                      <TableCell>
                        <code className="text-sm font-mono">
                          {Array.isArray(value) ? value.join(', ') : String(value)}
                        </code>
                      </TableCell>
                    </TableRow>
                  )
                }

                return rows
              })
            )}
          </TableBody>
        </Table>
      </div>
    )
  }

  return (
    <div className="p-6">
      {/* Metadata header */}
      <TooltipProvider>
        <Tooltip>
          <TooltipTrigger asChild>
            <h1 className="text-xl font-semibold">{run.id.slice(0, 8)}</h1>
          </TooltipTrigger>
          <TooltipContent>{run.id}</TooltipContent>
        </Tooltip>
      </TooltipProvider>
      <p className="font-mono text-sm text-muted-foreground mt-1">{run.id}</p>
      <div className="text-xs text-muted-foreground mt-1 flex items-center gap-2">
        <Badge variant="outline" className={statusBadgeClassName(run.state)}>
          {run.state}
        </Badge>
        <span>
          {new Date(run.created_at).toLocaleString('en-US', {
            dateStyle: 'medium',
            timeStyle: 'short',
          })}
        </span>
        <span>{formatDuration(run.created_at, run.updated_at, run.state)}</span>
      </div>
      {source && (
        <div className="text-xs text-muted-foreground mt-1">
          <span className="font-medium">Source: </span>
          {source}
        </div>
      )}

      <Separator className="my-6" />

      {/* Outputs section */}
      <h2 className="text-xl font-semibold">Outputs</h2>
      {renderOutputsSection()}
    </div>
  )
}
