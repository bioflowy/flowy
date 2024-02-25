import { z } from "zod";
import { RouteConfig, extendZodWithOpenApi } from '@asteasolutions/zod-to-openapi';
import { createFactory } from "hono/factory";
import { zValidator } from "@hono/zod-validator";
import { getManager } from "./manager.js";

extendZodWithOpenApi(z);
export const JobFinishedRequestSchema = z
  .object({
    id: z.string(),
    isCwlOutput: z.boolean(),
    exitCode: z.number().int(),
    results: z.record(z.any()),
  })
  .openapi('JobFinishedRequest');

export const JobFinishedPath: RouteConfig = {
    method: 'post',
    path: '/api/jobFinished',
    description: 'report job finished',
    summary: 'report job finished',
    request: {
      body: {
        content: {
          'application/json': {
            schema: JobFinishedRequestSchema,
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
export const JobFinishedHandler =  createFactory().createHandlers(
  zValidator('json',JobFinishedRequestSchema), async (c) => {
    const jsonData = await c.req.valid('json')
    getManager().jobfinished(jsonData.id, jsonData.exitCode, jsonData.isCwlOutput, jsonData.results);
    return c.json('OK')
})