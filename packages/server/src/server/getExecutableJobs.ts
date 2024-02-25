import { z } from "zod";
import { RouteConfig, extendZodWithOpenApi } from '@asteasolutions/zod-to-openapi';
import { createFactory } from "hono/factory";
import { getManager } from "./manager.js";
import { JobExec, JobExecSchema } from "../JobExecutor.js";

extendZodWithOpenApi(z);

export const GetExecutableJobPath: RouteConfig = {
    method: 'post',
    path: '/api/getExectableJob',
    description: 'Get executable job',
    summary: 'Get a single user',
    responses: {
      200: {
        description: 'Exectable jobs',
        content: {
          'application/json': {
            schema: JobExecSchema.array(),
          },
        },
      },
    },
  }
  export const GetExecutableJobHandler =  createFactory().createHandlers(async (c) => {
    const jobs: JobExec[] = [];
    const ret = getManager().getExecutableJob();
    if (ret) {
      jobs.push(ret);
    }
      return c.json(jobs)
  })