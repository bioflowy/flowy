import { useNavigate } from 'react-router'
import { useAuthStore } from '@/store/authStore'
import { useProjectStore } from '@/store/projectStore'
import { useProjects } from '@/features/projects/useProjects'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

export function ProjectsPage() {
  const navigate = useNavigate()
  const role = useAuthStore((s) => s.role)
  const setActiveProject = useProjectStore((s) => s.setActiveProject)
  const { data: projects, isLoading } = useProjects()

  const isAdmin = role === 'admin'

  function handleCardClick(slug: string) {
    setActiveProject(slug)
    navigate(`/projects/${slug}`)
  }

  return (
    <div className="p-6">
      {/* Page header */}
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">Projects</h1>
        {isAdmin && (
          <Button variant="default" onClick={() => navigate('/projects/new')}>
            Create Project
          </Button>
        )}
      </div>

      {/* Grid */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6 mt-8">
        {isLoading ? (
          // Loading skeleton cards
          Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="h-32 animate-pulse rounded bg-muted" />
          ))
        ) : !projects || projects.length === 0 ? (
          // Empty state (spans full grid)
          <div className="col-span-full flex flex-col items-center justify-center py-16 text-center">
            <h2 className="text-xl font-semibold">No projects yet.</h2>
            <p className="text-sm text-muted-foreground mt-2">
              You are not a member of any projects.
            </p>
            {isAdmin && (
              <>
                <p className="text-sm text-muted-foreground mt-1">
                  Create one to get started.
                </p>
                <Button
                  variant="default"
                  className="mt-4"
                  onClick={() => navigate('/projects/new')}
                >
                  Create Project
                </Button>
              </>
            )}
          </div>
        ) : (
          projects.map((project) => (
            <Card
              key={project.id}
              className="cursor-pointer hover:bg-sidebar transition-colors"
              role="button"
              tabIndex={0}
              onClick={() => handleCardClick(project.slug)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') handleCardClick(project.slug)
              }}
            >
              <CardHeader>
                <CardTitle>{project.slug}</CardTitle>
              </CardHeader>
              {project.description && (
                <CardContent>
                  <p className="text-sm text-muted-foreground line-clamp-2">
                    {project.description}
                  </p>
                </CardContent>
              )}
            </Card>
          ))
        )}
      </div>
    </div>
  )
}
