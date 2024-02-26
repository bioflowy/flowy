import { z } from "zod";
import { RouteConfig, extendZodWithOpenApi } from '@asteasolutions/zod-to-openapi';
import { createFactory } from "hono/factory";
import { zValidator } from "@hono/zod-validator";
import { getManager } from "./manager.js";
import { RuntimeContext } from "../context.js";
import { exec } from "../main.js";
import { _logger } from "../loghandler.js";

extendZodWithOpenApi(z);

export const ExecuteJobInputSchema = z.object({
    tool_path: z.string(),
    job_path: z.string().optional(),
    outdir: z.string().optional(),
    basedir: z.string().optional(),
    clientWorkDir: z.string(),
    move_output: z.enum(['copy', 'leave', 'move']).optional(),
  });

export const executeJobPath: RouteConfig = {
    method: 'post',
    path: '/api/executeJob',
    description: 'report job failed',
    summary: 'report job failed',
    request: {
      body: {
        content: {
          'application/json': {
            schema: ExecuteJobInputSchema,
          },
        },
      },
    },
    responses: {
      200: {
        description: 'Exectable jobs',
        content: {
          'application/json': {
            schema: z.object({
              result: z.any(),
              status: z.string(),
          }),
        }
        },
      },
    },
  }
  export const executeJobHandler =  createFactory().createHandlers(
    zValidator('json',ExecuteJobInputSchema), async (c) => {
      try{
      const input = await c.req.valid('json')
      console.log(input)
      const manager = getManager()
      const runtimeContext = new RuntimeContext({
        clientWorkDir: input.clientWorkDir,
        outdir: input.outdir ? input.outdir : input.clientWorkDir,
        move_output: input.move_output,
        sharedFilesystemConfig: manager.getServerConfig().sharedFileSystem,
      });
      if (input.basedir) {
        runtimeContext.basedir = input.basedir;
      }
      if (!input.tool_path.startsWith('/')) {
        if (input.basedir.endsWith('/')) {
          input.tool_path = `${input.basedir}${input.tool_path}`;
        } else {
          input.tool_path = `${input.basedir}/${input.tool_path}`;
        }
      }
      const [result, status] = await exec(runtimeContext, input.tool_path, input.job_path);
      return c.json({result, status});
    }catch(e){
      _logger.error(e)
      if(e instanceof Error){
        _logger.error(e.message, e.stack)
        return c.json({result: e.message, status: 'exception'})
      }else{
        return c.json({result: e.toString(), status: 'exception'})
      }
    }
  })