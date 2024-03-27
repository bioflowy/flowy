import { z } from "zod";
import { createFactory } from "hono/factory";
import {
    OpenAPIRegistry,
    OpenApiGeneratorV3,
  } from '@asteasolutions/zod-to-openapi';
import { WorkerStartedPath } from "./workerstarted.js";
import { GetExecutableJobPath } from "./getExecutableJobs.js";
import { JobFailedPath } from "./jobFailed.js";
import { DoEvalPath } from "./doEval.js";
import { JobFinishedPath } from "./jobFinished.js";
import * as yaml from "js-yaml";
  
export const OpenApiHandler =  createFactory().createHandlers(async (c) => {
    const registry = new OpenAPIRegistry();
    registry.registerPath(WorkerStartedPath)
    registry.registerPath(GetExecutableJobPath)
    registry.registerPath(JobFinishedPath)
    registry.registerPath(JobFailedPath)
    registry.registerPath(DoEvalPath)
    const generator = new OpenApiGeneratorV3(registry.definitions);
    const docs = generator.generateDocument({
    openapi: '3.0.0',
    info: {
        version: '1.0.0',
        title: 'My API',
        description: 'This is the API',
    },
    servers: [{ url: '' }],
    })
    const fileContent = yaml.dump(docs);
    return c.text(fileContent)
})