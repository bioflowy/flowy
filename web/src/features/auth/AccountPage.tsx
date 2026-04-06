import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { ChangePasswordDialog } from '@/features/auth/ChangePasswordDialog'
import { useLogout } from '@/features/auth/useLogout'

export function AccountPage() {
  const [dialogOpen, setDialogOpen] = useState(false)
  const logout = useLogout()

  return (
    <div className="max-w-2xl">
      <h1 className="text-xl font-semibold leading-[1.2] mb-6">Account</h1>

      {/* Password section */}
      <section className="mb-8">
        <h2 className="text-xs font-normal text-muted-foreground uppercase tracking-wider mb-3">
          Password
        </h2>
        <Button variant="outline" onClick={() => setDialogOpen(true)}>
          Change Password
        </Button>
      </section>

      {/* Session section */}
      <section>
        <h2 className="text-xs font-normal text-muted-foreground uppercase tracking-wider mb-3">
          Session
        </h2>
        <Button variant="outline" onClick={logout}>
          Sign out
        </Button>
      </section>

      <ChangePasswordDialog open={dialogOpen} onOpenChange={setDialogOpen} />
    </div>
  )
}
