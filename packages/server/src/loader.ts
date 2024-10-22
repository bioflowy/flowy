import * as crypto from 'node:crypto';
import * as path from 'node:path';
import * as cwlTsAuto from '@flowy/cwl-ts-auto';
import type { LoadingContext } from './context.js';
import { ValidationException } from './errors.js';
import type { Process } from './process.js';
import { filePathToURI } from './utils.js';
import { default_make_tool } from './workflow.js';

function sha1(data: string): string {
  const hash = crypto.createHash('sha1');
  hash.update(data);
  return hash.digest('hex');
}
export async function load_tool(tool_path: string, loadingContext: LoadingContext) {
  const doc = await cwlTsAuto.loadDocument(tool_path);
  return doc;
}
export type CwlDocument = cwlTsAuto.CommandLineTool | cwlTsAuto.ExpressionTool | cwlTsAuto.Workflow | cwlTsAuto.Operation | Array<cwlTsAuto.CommandLineTool | cwlTsAuto.ExpressionTool | cwlTsAuto.Workflow | cwlTsAuto.Operation>;
export async function loadTool(doc: CwlDocument,tool_path: string,loadingContext: LoadingContext): Promise<[Process | undefined, string]>{
  if (doc instanceof Array) {
    let tool_id = tool_path;
    if (!(tool_id.startsWith('file://') || tool_id.startsWith('s3:/'))) {
      if (!path.isAbsolute(tool_id)) {
        tool_id = path.join(process.cwd(), tool_id);
      }
      tool_id = filePathToURI(tool_id);
    }

    for (let index = 0; index < doc.length; index++) {
      if (doc[index].id === tool_id) {
        return [await default_make_tool(doc[index], loadingContext), 'success'];
      }
    }
    for (let index = 0; index < doc.length; index++) {
      if (doc[index].id.endsWith('#main')) {
        return [await default_make_tool(doc[index], loadingContext), 'success'];
      }
    }
  } else {
    return [await default_make_tool(doc, loadingContext), 'success'];
  }
  return [undefined, 'failed'];

}
export async function loadDocument(
  tool_path: string,
  loadingContext: LoadingContext,
): Promise<CwlDocument> {
  let tool_file_path = tool_path;
  if (tool_file_path.includes('#')) {
    tool_file_path = tool_file_path.split('#')[0];
  }
  return await cwlTsAuto.loadDocument(tool_file_path, loadingContext.baseuri, loadingContext.loadingOptions);
}

