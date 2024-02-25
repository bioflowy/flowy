import { Hono } from 'hono'
import { extendZodWithOpenApi } from '@asteasolutions/zod-to-openapi';
import { z } from 'zod';
extendZodWithOpenApi(z);
import { swaggerUI } from '@hono/swagger-ui'
import {
  OpenAPIRegistry,
  OpenApiGeneratorV3,
  RouteConfig
} from '@asteasolutions/zod-to-openapi';
import { showRoutes } from 'hono/dev';
import { createFactory } from 'hono/factory';
import { WorkerStartedHandler, WorkerStartedPath } from './server/workerstarted.js';
import { GetExecutableJobHandler, GetExecutableJobPath } from './server/getExecutableJobs.js';
import { JobFinishedHandler, JobFinishedPath } from './server/jobFinished.js';
import { JobFailedHandler, JobFailedPath } from './server/jobFailed.js';
import { JobEvalHandler, DoEvalPath } from './server/doEval.js';
import { executeJobHandler, executeJobPath } from './server/executeJob.js';
const registry = new OpenAPIRegistry();
registry.registerPath(WorkerStartedPath)
registry.registerPath(GetExecutableJobPath)
registry.registerPath(JobFinishedPath)
registry.registerPath(JobFailedPath)
registry.registerPath(DoEvalPath)
const factory = createFactory()
const app = new Hono()
const route = app.post(WorkerStartedPath.path,...WorkerStartedHandler)
                  .post(GetExecutableJobPath.path,...GetExecutableJobHandler)
                  .post(JobFinishedPath.path, ...JobFinishedHandler)
                  .post(JobFailedPath.path,...JobFailedHandler)
                  .post(DoEvalPath.path,...JobEvalHandler)
showRoutes(route)
const executeJobRoute = app.post(executeJobPath.path, ...executeJobHandler)
app.get('/doc', (c) => {
  const generator = new OpenApiGeneratorV3(registry.definitions);
  const doc = generator.generateDocument({
    openapi: '3.0.0',
    info: {
      version: '1.0.0',
      title: 'My API',
      description: 'This is the API',
    },
    servers: [{ url: '' }],
  });
  return c.json(doc)
})
app.get('/ui', swaggerUI({ url: '/doc' }))
export type ExecuteJobRoute = typeof executeJobRoute
export default app