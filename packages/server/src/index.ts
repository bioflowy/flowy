import { serve } from '@hono/node-server'
import { Hono } from 'hono'
import app from './app.js'


serve({
    fetch: app.fetch,
    port: 5173,
  })