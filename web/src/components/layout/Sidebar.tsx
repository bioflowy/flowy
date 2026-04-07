import { NavLink, useNavigate } from 'react-router'
import { Package, Play, Users2, Shield, UserCircle } from 'lucide-react'
import { Separator } from '@/components/ui/separator'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useAuthStore } from '@/store/authStore'
import { useProjectStore } from '@/store/projectStore'
import { useProjects } from '@/features/projects/useProjects'

const globalNavItems = [
  { to: '/users', label: 'Users', icon: Shield },
]

export function Sidebar() {
  const username = useAuthStore((s) => s.username)
  const role = useAuthStore((s) => s.role)
  const navigate = useNavigate()

  const { data: projects } = useProjects()
  const activeSlug = useProjectStore((s) => s.activeProjectSlug)
  const setActiveProject = useProjectStore((s) => s.setActiveProject)

  return (
    <aside className="flex w-60 flex-col border-r bg-sidebar">
      {/* App name */}
      <div className="flex h-14 items-center px-4 font-semibold text-lg">
        flowy
      </div>

      <Separator />

      {/* Project dropdown selector */}
      <div className="px-2 py-3">
        <Select
          value={activeSlug ?? ''}
          onValueChange={(slug) => {
            setActiveProject(slug)
            navigate(`/projects/${slug}`)
          }}
        >
          <SelectTrigger className="w-full">
            <SelectValue placeholder="Select a project..." />
          </SelectTrigger>
          <SelectContent>
            {projects?.map((p) => (
              <SelectItem key={p.slug} value={p.slug}>
                {p.slug}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <Separator />

      {/* Project-scoped navigation — only when a project is selected */}
      {activeSlug && (
        <nav className="flex-1 space-y-1 px-2 py-2">
          {[
            { to: `/projects/${activeSlug}`, label: 'Overview', icon: Package },
            { to: `/projects/${activeSlug}/registry`, label: 'Registry', icon: Package },
            { to: `/projects/${activeSlug}/runs`, label: 'Runs', icon: Play },
            { to: `/projects/${activeSlug}`, label: 'Members', icon: Users2 },
          ].map((item) => (
            <NavLink
              key={item.label}
              to={item.to}
              end={item.label === 'Overview' || item.label === 'Members'}
              className={({ isActive }) =>
                `flex items-center gap-2 rounded-md px-3 py-2 text-sm ${
                  isActive
                    ? 'bg-primary text-primary-foreground font-medium'
                    : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground'
                }`
              }
            >
              <item.icon className="h-4 w-4" />
              {item.label}
            </NavLink>
          ))}
        </nav>
      )}

      {/* Spacer when no project selected */}
      {!activeSlug && <div className="flex-1" />}

      {/* Global navigation — admin only */}
      {role === 'admin' && (
        <>
          <Separator />
          <nav className="space-y-1 px-2 py-2">
            {globalNavItems.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                className={({ isActive }) =>
                  `flex items-center gap-2 rounded-md px-3 py-2 text-sm ${
                    isActive
                      ? 'bg-primary text-primary-foreground font-medium'
                      : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground'
                  }`
                }
              >
                <item.icon className="h-4 w-4" />
                {item.label}
              </NavLink>
            ))}
          </nav>
        </>
      )}

      <Separator />

      {/* Account menu */}
      <button
        onClick={() => navigate('/account')}
        className="flex w-full items-center gap-2 px-4 py-3 text-sm text-muted-foreground hover:text-foreground transition-colors"
      >
        <UserCircle className="h-4 w-4" />
        {username ?? 'Account'}
      </button>
    </aside>
  )
}
