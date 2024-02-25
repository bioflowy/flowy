import { z } from "zod";
import { RouteConfig, extendZodWithOpenApi } from '@asteasolutions/zod-to-openapi';
import { createFactory } from "hono/factory";
import { zValidator } from "@hono/zod-validator";
import { getManager } from "./manager.js";

extendZodWithOpenApi(z);
export const JobFailedRequestSchema = z.object({
    id: z.string(),
    errorMsg: z.string(),
  });
  
export const JobFailedPath: RouteConfig = {
    method: 'post',
    path: '/api/jobFailed',
    description: 'report job failed',
    summary: 'report job failed',
    request: {
      body: {
        content: {
          'application/json': {
            schema: JobFailedRequestSchema,
          },
        },
      },
    },
    responses: {
      200: {
        description: 'Exectable jobs',
        content: {
          'application/json': {
            schema: z.string(),
          },
        },
      },
    },
  }
export const JobFailedHandler =  createFactory().createHandlers(
  zValidator('json',JobFailedRequestSchema), async (c) => {
    const jsonData = await c.req.valid('json')
    getManager().jobfailed(jsonData.id, jsonData.errorMsg);
    
    return c.json('OK')
})