import { z } from "zod";
import { RouteConfig, extendZodWithOpenApi } from '@asteasolutions/zod-to-openapi';
import { createFactory } from "hono/factory";
import { zValidator } from "@hono/zod-validator";
import { getManager } from "./manager.js";
import { createFlowyJobURL } from "../flowyurl.js";

extendZodWithOpenApi(z);
const DoEvalRequestSchema = z.object({
  id: z.string(),
  ex: z.string(),
  exitCode: z.number().int().optional(),
  context: z.any(),
});
export type DoEvalRequest = z.infer<typeof DoEvalRequestSchema>;
  
  
export const DoEvalResultSchema = z
.object({
    string_value: z.string().optional(),
    json_value: z.record(z.any()).optional(),
    boolean_value: z.boolean().optional(),
})
.openapi('DoEvalResult');
  
export const DoEvalPath: RouteConfig = {
    method: 'post',
    path: '/api/do_eval',
    description: 'report job failed',
    summary: 'report job failed',
    request: {
      body: {
        content: {
          'application/json': {
            schema: DoEvalRequestSchema,
          },
        },
      },
    },
    responses: {
      200: {
        description: 'Exectable jobs',
        content: {
          'application/json': {
            schema: z.any(),
          },
        },
      },
    },
  }
  export const JobEvalHandler =  createFactory().createHandlers(
    zValidator('json',DoEvalRequestSchema), async (c) => {
      const jsonData = await c.req.valid('json')
      const result = await getManager().evaluate(createFlowyJobURL(jsonData.id), jsonData.ex, jsonData.context,jsonData.exitCode);
      
      return c.json(result)
  })