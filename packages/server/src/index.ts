import { serve } from '@hono/node-server'
import app from './app'

const port = 3001

console.log(`Server is running on port ${port}`)

serve({
  fetch: app.fetch,
  port
})
