import { extendZodWithOpenApi } from '@asteasolutions/zod-to-openapi';
import { z } from 'zod';
import { MapperEntSchema } from './pathmapper.js';

import { CWLOutputType } from './utils.js';
import { CommandStringSchema } from './commandstring.js';
extendZodWithOpenApi(z);

export async function fastifyEvaluator(
  url: string,
  id: string,
  ex: CWLOutputType | undefined,
  context: CWLOutputType | undefined,
): Promise<CWLOutputType> {
  const postData = JSON.stringify({ id, ex, context });
  const res = await fetch('http://localhost:3000/api/do_eval', {
    method: 'POST',
    body: postData,
    headers: {
      'Content-Type': 'application/json',
    },
  });
  const json: JSON = await res.json();
  if ('string_value' in json) {
    return json['string_value'] as string;
  } else {
    return json['json_value'];
  }
}
const LoadListingEnumSchema = z.enum(['no_listing', 'shallow_listing', 'deep_listing']).openapi('LoadListingEnum');
export const SecondaryFileSchema = z.object({
  pattern: z.string(),
  // Separate required property into two values of type string and type boolean
  // because oapi-codegen, golang's open-api code generator, has a bug in generating union type.
  requiredBoolean: z.boolean().optional(),
  requiredString: z.string().optional(),
});
// OutputBinding Schema
const OutputBindingSchema = z
  .object({
    name: z.string(),
    secondaryFiles: z.array(SecondaryFileSchema),
    loadContents: z.boolean().optional(),
    loadListing: LoadListingEnumSchema.optional(),
    glob: z.array(z.string()).optional(),
    outputEval: z.string().optional(),
    streamable: z.boolean().optional().default(false)
  })
  .openapi('OutputBinding');
export type OutputBinding = z.infer<typeof OutputBindingSchema>;

export type OutputSecondaryFile = z.infer<typeof SecondaryFileSchema>;

export const RuntimeSchema =  z.object({
  custom_net: z.string().optional()
}).openapi("runtime")
// JobExec Schema
export const JobExecSchema = z.object({
  id: z.string(),
  commands: z.array(CommandStringSchema),
  stdin_path: z.string().optional(),
  stdout_path: z.string().optional(),
  stderr_path: z.string().optional(),
  env: z.record(z.string()),
  cwd: z.string(),
  containerOutdir: z.string(),
  tmpDir: z.string(),
  removeTmpDir: z.boolean(),
  timelimit: z.number().int().optional(),
  outputBindings: z.array(OutputBindingSchema),
  fileitems: z.array(MapperEntSchema),
  generatedlist: z.array(MapperEntSchema),
  inplace_update: z.boolean(),
  networkaccess: z.boolean(),
  outputBaseDir: z.string().optional(),
  dockerExec: z.string().optional(),
  dockerImage: z.string().optional(),
  runtime: RuntimeSchema,
});
export type Runtime = z.infer<typeof RuntimeSchema>;
export type JobExec = z.infer<typeof JobExecSchema>;
