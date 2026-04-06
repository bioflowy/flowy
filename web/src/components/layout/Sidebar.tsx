import { NavLink, useNavigate } from 'react-router'
import { Package, Play, Users, Shield, UserCircle } from 'lucide-react'
import { Separator } from '@/components/ui/separator'
import { useAuthStore } from '@/store/authStore'

const projectNavItems = [
  { to: '/registry', label: 'Registry', icon: Package },
  { to: '/runs', label: 'Runs', icon: Play },
  { to: '/members', label: 'Members', icon: Users },
]

const globalNavItems = [
  { to: '/users', label: 'Users', icon: Shield },
]

export function Sidebar() {
  const username = useAuthStore((s) => s.username)
  const role = useAuthStore((s) => s.role)
  const navigate = useNavigate()

  return (
    <aside className="flex w-60 flex-col border-r bg-sidebar">
      {/* App name */}
      <div className="flex h-14 items-center px-4 font-semibold text-lg">
        flowy
      </div>

      <Separator />

      {/* Project selector placeholder */}
      <div className="px-4 py-3 text-sm text-muted-foreground">
        Select project
      </div>

      <Separator />

      {/* Project-scoped navigation */}
      <nav className="flex-1 space-y-1 px-2 py-2">
        {projectNavItems.map((item) => (
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
