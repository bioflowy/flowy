import { z } from "zod"
import { extendZodWithOpenApi } from '@asteasolutions/zod-to-openapi';
extendZodWithOpenApi(z);

export const ParamSchema = z.object({
    name: z.string().openapi({ example: '1212121' }),
  })
export const ReturnSchema = z.object({
    message: z.string().openapi({ example: '1212121' }),
  })
  