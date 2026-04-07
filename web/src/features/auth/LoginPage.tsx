import { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router'
import { useForm } from 'react-hook-form'
import { z } from 'zod'
import { zodResolver } from '@hookform/resolvers/zod'
import { Loader2 } from 'lucide-react'

import { apiClient, ApiError } from '@/lib/apiClient'
import { useAuthStore } from '@/store/authStore'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'

const loginSchema = z.object({
  username: z.string().min(1, 'Username is required'),
  password: z.string().min(1, 'Password is required'),
})

type LoginFormValues = z.infer<typeof loginSchema>

export function LoginPage() {
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const [serverError, setServerError] = useState<string | null>(null)

  const form = useForm<LoginFormValues>({
    resolver: zodResolver(loginSchema),
    defaultValues: {
      username: '',
      password: '',
    },
  })

  const { isSubmitting } = form.formState

  async function onSubmit(values: LoginFormValues) {
    setServerError(null)

    try {
      const { token } = await apiClient.post<{ token: string }>('/auth/login', {
        username: values.username,
        password: values.password,
      })

      // Guard: if no token returned (auth-disabled backend mode), redirect to app root
      if (!token) {
        navigate('/', { replace: true })
        return
      }

      // Decode JWT payload to extract username and role
      const payload = JSON.parse(atob(token.split('.')[1]))
      useAuthStore.getState().setAuth(token, payload.username, payload.role)

      // Navigate to redirect URL or default to /
      const redirect = searchParams.get('redirect')
      navigate(redirect ?? '/', { replace: true })
    } catch (error) {
      if (error instanceof ApiError) {
        setServerError('Invalid username or password. Please try again.')
      } else {
        setServerError('Unable to connect. Check your connection and try again.')
      }
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-background">
      <Card className="w-full max-w-sm">
        <CardContent className="p-6">
          <div className="mb-6">
            <h1 className="text-[28px] font-semibold leading-[1.2]">flowy</h1>
            <p className="text-sm text-muted-foreground">Sign in to continue</p>
          </div>

          <Form {...form}>
            <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
              <FormField
                control={form.control}
                name="username"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Username</FormLabel>
                    <FormControl>
                      <Input type="text" autoComplete="username" {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="password"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Password</FormLabel>
                    <FormControl>
                      <Input type="password" autoComplete="current-password" {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {serverError && (
                <p className="text-xs text-destructive">{serverError}</p>
              )}

              <Button type="submit" className="w-full" disabled={isSubmitting}>
                {isSubmitting ? (
                  <>
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    Signing in...
                  </>
                ) : (
                  'Sign in'
                )}
              </Button>
            </form>
          </Form>
        </CardContent>
      </Card>
    </div>
  )
}
