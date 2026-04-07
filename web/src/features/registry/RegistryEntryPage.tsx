import { useState, useEffect } from 'react'
import { useParams } from 'react-router'
import { Trash2 } from 'lucide-react'

import { useRegistryVersions, useRegistryContent } from '@/features/registry/useRegistry'
import { useProjectMembers } from '@/features/projects/useProjects'
import { useAuthStore } from '@/store/authStore'
import { WDLSourceViewer } from '@/features/registry/WDLSourceViewer'
import { CreateTagDialog } from '@/features/registry/CreateTagDialog'
import { DeleteTagDialog } from '@/features/registry/DeleteTagDialog'
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

function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  })
}

function truncateDigest(digest: string): string {
  if (digest.startsWith('sha256:')) return `sha256:${digest.slice(7, 23)}...`
  return digest
}

export function RegistryEntryPage() {
  const { slug, name } = useParams<{ slug: string; name: string }>()

  const { data: versions, isLoading: versionsLoading } = useRegistryVersions(slug!, name!)
  const [selectedDigest, setSelectedDigest] = useState<string>('')

  useEffect(() => {
    if (versions && versions.length > 0 && !selectedDigest) {
      setSelectedDigest(versions[0].digest)
    }
  }, [versions, selectedDigest])

  const {
    data: content,
    isLoading: contentLoading,
    isError: contentError,
  } = useRegistryContent(slug!, name!, selectedDigest)

  const { data: members } = useProjectMembers(slug!)
  const currentUsername = useAuthStore((s) => s.username)
  const currentMember = members?.find((m) => m.username === currentUsername)
  const isProjectAdmin = currentMember?.role === 'admin'

  const [createTagOpen, setCreateTagOpen] = useState(false)
  const [deleteTagName, setDeleteTagName] = useState<string | null>(null)

  return (
    <div className="p-6">
      {/* Entry header */}
      <h1 className="text-xl font-semibold">{name}</h1>
      <div className="text-xs text-muted-foreground mt-1 flex items-center gap-2">
        {content ? (
          <>
            <Badge
              variant="outline"
              className={
                content.kind === 'workflow'
                  ? 'border-purple-300 text-purple-700 bg-purple-50'
                  : 'border-blue-300 text-blue-700 bg-blue-50'
              }
            >
              {content.kind}
            </Badge>
            <span>{versions?.length ?? 0} version{(versions?.length ?? 0) !== 1 ? 's' : ''}</span>
          </>
        ) : (
          <span>Loading...</span>
        )}
      </div>

      {/* Versions section */}
      <div className="mt-8">
        <div className="flex items-center justify-between">
          <h2 className="text-xl font-semibold">Versions</h2>
          {isProjectAdmin && (
            <Button onClick={() => setCreateTagOpen(true)}>Create Tag</Button>
          )}
        </div>

        <div className="mt-4">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Digest</TableHead>
                <TableHead>Tags</TableHead>
                <TableHead>Created</TableHead>
                <TableHead className="w-[50px]"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {versionsLoading ? (
                Array.from({ length: 3 }).map((_, i) => (
                  <TableRow key={i} data-testid="skeleton-row">
                    <TableCell>
                      <div className="h-4 w-48 animate-pulse rounded bg-muted" />
                    </TableCell>
                    <TableCell>
                      <div className="h-4 w-24 animate-pulse rounded bg-muted" />
                    </TableCell>
                    <TableCell>
                      <div className="h-4 w-20 animate-pulse rounded bg-muted" />
                    </TableCell>
                    <TableCell />
                  </TableRow>
                ))
              ) : !versions || versions.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={4} className="text-center text-muted-foreground">
                    No versions found.
                  </TableCell>
                </TableRow>
              ) : (
                versions.map((v) => (
                  <TableRow
                    key={v.id}
                    className={
                      v.digest === selectedDigest
                        ? 'bg-muted/50 cursor-pointer'
                        : 'cursor-pointer hover:bg-muted/50'
                    }
                    onClick={() => setSelectedDigest(v.digest)}
                  >
                    <TableCell>{truncateDigest(v.digest)}</TableCell>
                    <TableCell>
                      {v.tags.length > 0 ? (
                        v.tags.map((tag) => (
                          <Badge variant="outline" key={tag} className="mr-1">
                            {tag}
                          </Badge>
                        ))
                      ) : (
                        '\u2014'
                      )}
                    </TableCell>
                    <TableCell>{formatDate(v.created_at)}</TableCell>
                    <TableCell>
                      {isProjectAdmin &&
                        v.tags.map((tag) => (
                          <Button
                            key={tag}
                            variant="ghost"
                            size="icon"
                            className="h-7 w-7"
                            onClick={(e) => {
                              e.stopPropagation()
                              setDeleteTagName(tag)
                            }}
                            aria-label={`Delete tag ${tag}`}
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                          </Button>
                        ))}
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
      </div>

      <Separator className="my-6" />

      {/* Source section */}
      <div>
        <h2 className="text-xl font-semibold">Source</h2>
        <p className="text-xs text-muted-foreground mt-1">{truncateDigest(selectedDigest)}</p>
        <div className="mt-4">
          {contentLoading ? (
            <div className="h-[200px] animate-pulse rounded bg-muted" data-testid="source-skeleton" />
          ) : contentError ? (
            <p className="text-muted-foreground">Failed to load source for this version.</p>
          ) : content ? (
            <WDLSourceViewer content={content.content} />
          ) : null}
        </div>
      </div>

      {/* Dialogs */}
      <CreateTagDialog
        open={createTagOpen}
        onOpenChange={setCreateTagOpen}
        slug={slug!}
        name={name!}
        versions={versions ?? []}
      />
      <DeleteTagDialog
        open={!!deleteTagName}
        onOpenChange={(open) => {
          if (!open) setDeleteTagName(null)
        }}
        slug={slug!}
        entryName={name!}
        tagName={deleteTagName}
      />
    </div>
  )
}
