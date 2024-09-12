import { Hono, HonoRequest } from 'hono'
import { extendZodWithOpenApi } from '@asteasolutions/zod-to-openapi';
import { z } from 'zod';
extendZodWithOpenApi(z);
import { swaggerUI } from '@hono/swagger-ui'
import { showRoutes } from 'hono/dev';
import { createFactory } from 'hono/factory';
import { WorkerStartedHandler, WorkerStartedPath } from './server/workerstarted.js';
import { GetExecutableJobHandler, GetExecutableJobPath } from './server/getExecutableJobs.js';
import { JobFinishedHandler, JobFinishedPath } from './server/jobFinished.js';
import { JobFailedHandler, JobFailedPath } from './server/jobFailed.js';
import { JobEvalHandler, DoEvalPath } from './server/doEval.js';
import { executeJobHandler, executeJobPath } from './server/executeJob.js';
import { WorkerApiHandler,ClientApiHandler} from './server/openapi.js';
import { getJobInfoHandler, getJobInfoPath } from './server/getJobInfo.js';
import { hc } from 'hono/client'

const app = new Hono()
app.get("worker-docs",...WorkerApiHandler);
app.get("client-docs",...ClientApiHandler);
app.post(WorkerStartedPath.path,...WorkerStartedHandler)
                  .post(GetExecutableJobPath.path,...GetExecutableJobHandler)
                  .post(JobFinishedPath.path, ...JobFinishedHandler)
                  .post(JobFailedPath.path,...JobFailedHandler)
                  .post(DoEvalPath.path,...JobEvalHandler)
                  .post(executeJobPath.path, ...executeJobHandler)
                  .post(getJobInfoPath.path,...getJobInfoHandler)

export default app
