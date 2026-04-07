import { useParams, useNavigate } from 'react-router'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { useRuns, RunListItem } from '@/features/runs/useRuns'

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

export function formatDuration(createdAt: string, updatedAt: string, state: string): string {
  if (state === 'PENDING') return '\u2014'

  const start = new Date(createdAt).getTime()
  const end = state === 'RUNNING' ? Date.now() : new Date(updatedAt).getTime()
  const totalSeconds = Math.floor((end - start) / 1000)

  if (totalSeconds < 60) return `${totalSeconds}s`
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60
  if (minutes < 60) return `${minutes}m ${seconds}s`
  const hours = Math.floor(minutes / 60)
  const mins = minutes % 60
  return `${hours}h ${mins}m`
}

function truncateSource(source: string | undefined): string {
  if (!source) return '\u2014'
  if (source.length > 40) return source.slice(0, 40) + '...'
  return source
}

export function RunsPage() {
  const { slug } = useParams<{ slug: string }>()
  const navigate = useNavigate()
  const { data: runs, isLoading, isError } = useRuns(slug ?? '')

  return (
    <div className="p-6">
      <h1 className="text-xl font-semibold">Runs</h1>

      <div className="mt-8">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Run ID</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Source</TableHead>
              <TableHead>Started</TableHead>
              <TableHead>Duration</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              Array.from({ length: 5 }).map((_, i) => (
                <TableRow key={i} data-testid="skeleton-row">
                  <TableCell>
                    <div className="h-4 w-20 animate-pulse rounded bg-muted" />
                  </TableCell>
                  <TableCell>
                    <div className="h-4 w-16 animate-pulse rounded bg-muted" />
                  </TableCell>
                  <TableCell>
                    <div className="h-4 w-32 animate-pulse rounded bg-muted" />
                  </TableCell>
                  <TableCell>
                    <div className="h-4 w-24 animate-pulse rounded bg-muted" />
                  </TableCell>
                  <TableCell>
                    <div className="h-4 w-12 animate-pulse rounded bg-muted" />
                  </TableCell>
                </TableRow>
              ))
            ) : isError ? (
              <TableRow>
                <TableCell colSpan={5} className="text-center text-muted-foreground">
                  Failed to load runs. Please refresh the page.
                </TableCell>
              </TableRow>
            ) : !runs || runs.length === 0 ? (
              <TableRow>
                <TableCell colSpan={5} className="text-center text-muted-foreground">
                  No runs yet.
                  <p className="text-xs text-muted-foreground mt-1">
                    Submit a workflow using the flowy CLI to see runs here.
                  </p>
                </TableCell>
              </TableRow>
            ) : (
              runs.map((run: RunListItem) => (
                <TableRow
                  key={run.id}
                  className="cursor-pointer hover:bg-muted/50"
                  onClick={() => navigate(`/projects/${slug}/runs/${run.id}`)}
                >
                  <TableCell>
                    <TooltipProvider>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <span className="font-mono text-sm">{run.id.slice(0, 8)}</span>
                        </TooltipTrigger>
                        <TooltipContent>{run.id}</TooltipContent>
                      </Tooltip>
                    </TooltipProvider>
                  </TableCell>
                  <TableCell>
                    <Badge variant="outline" className={statusBadgeClassName(run.state)}>
                      {run.state}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    {truncateSource(run.registry_ref || run.wdl_path)}
                  </TableCell>
                  <TableCell>
                    {new Date(run.created_at).toLocaleString('en-US', {
                      dateStyle: 'medium',
                      timeStyle: 'short',
                    })}
                  </TableCell>
                  <TableCell>
                    {formatDuration(run.created_at, run.updated_at, run.state)}
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}
