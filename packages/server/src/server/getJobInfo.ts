import { z } from "zod";
import { RouteConfig, extendZodWithOpenApi } from '@asteasolutions/zod-to-openapi';
import { createFactory } from "hono/factory";
import { zValidator } from "@hono/zod-validator";
import { getManager } from "./manager.js";
import { RuntimeContext } from "../context.js";
import { exec } from "../main.js";
import { _logger } from "../loghandler.js";
import { getJobManager } from "../jobmanager.js";

extendZodWithOpenApi(z);

export const GetJobInfoInputSchema = z.object({
  jobId:z.string()
});

export const GetJobInfoOutputSchema = z.object({
  result: z.any(),
  status: z.string(),
});
type GetJobInfoOutput = z.infer<typeof GetJobInfoOutputSchema>
export const getJobInfoPath: RouteConfig = {
    method: 'get',
    path: '/api/getJobInfo',
    description: 'get job detailed imformation',
    summary: 'get job detailed imformation',
    request: {
      body: {
        content: {
          'application/json': {
            schema: GetJobInfoInputSchema,
          },
        },
      },
    },
    responses: {
      200: {
        description: 'Exectable jobs',
        content: {
          'application/json': {
            schema: GetJobInfoOutputSchema,
        }
        },
      },
    },
  }
  export const getJobInfoHandler =  createFactory().createHandlers(
    zValidator('json',GetJobInfoInputSchema), async (c) => {
      try{
        const input = await c.req.valid('json')
        const manager = getJobManager()
        const job = manager.getJobInfo(input.jobId)
        const result:GetJobInfoOutput ={
          result: job.results,
          status: job.processStatus,
        }
        return c.json(result);
      }catch(e){
        console.log(e)
      }
  })