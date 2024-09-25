import { z } from "zod";
import { RouteConfig, extendZodWithOpenApi } from '@asteasolutions/zod-to-openapi';
import { createFactory } from "hono/factory";
import { zValidator } from "@hono/zod-validator";
import { getManager } from "./manager.js";
import { RuntimeContext } from "../context.js";
import { exec } from "../main.js";
import { _logger } from "../loghandler.js";
import { getJobManager } from "../jobmanager.js";
import { getToolManager } from "../toolmanager.js";

extendZodWithOpenApi(z);

export const ImportToolInputSchema = z.object({
  toolPath:z.string()
});

export const ImportToolOutputSchema = z.object({
  toolId: z.string(),
});
type ImportToolOutput = z.infer<typeof ImportToolOutputSchema>
export const ImportToolPath: RouteConfig = {
    method: 'post',
    path: '/api/importTool',
    description: 'import tool',
    summary: 'import tool',
    request: {
      body: {
        content: {
          'application/json': {
            schema: ImportToolInputSchema,
          },
        },
      },
    },
    responses: {
      200: {
        description: 'Exectable jobs',
        content: {
          'application/json': {
            schema: ImportToolOutputSchema,
        }
        },
      },
    },
  }
  export const ImportToolHandler =  createFactory().createHandlers(
    zValidator('json',ImportToolInputSchema), async (c) => {
      try{
        const input = await c.req.valid('json')
        const manager = getToolManager()
        const toolInfo = await manager.importTool(input.toolPath)
        const result:ImportToolOutput ={
          toolId: toolInfo.id
        }
        return c.json(result);
      }catch(e){
        console.log(e)
      }
  })