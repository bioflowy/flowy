import { z } from "zod";
import { RouteConfig, extendZodWithOpenApi } from '@asteasolutions/zod-to-openapi';
import { SharedFileSystemSchema } from "./config.js";
import { createFactory } from "hono/factory";
import { zValidator } from "@hono/zod-validator";
import { getManager } from "./manager.js";

extendZodWithOpenApi(z);

const WorkerStartedInput = z.object({
    hostname: z.string(),
    cpu: z.number().int(),
    memory: z.number().int().openapi({ description: 'memory in MB' }),
  });
export const WorkerStartedPath: RouteConfig = {
    method: 'post',
    path: '/api/workerStarted',
    description: 'report worker started and return shared file system settings',
    request: {
      body: {
        content: {
          'application/json': {
            schema: WorkerStartedInput,
          },
        },
      },
    },
    responses: {
      200: {
        description: 'Exectable jobs',
        content: {
          'application/json': {
            schema: SharedFileSystemSchema,
          },
        },
      },
    },
  }
export const WorkerStartedHandler =  createFactory().createHandlers(
  zValidator('json',WorkerStartedInput), async (c) => {
    const jsonData = await c.req.valid('json')
    console.log('worker started', jsonData);
    
    return c.json(getManager().getServerConfig().sharedFileSystem)
})