import * as path from 'node:path';
import { Builder } from './builder.js';
import {
  CommandOutputParameter,
  CommandOutputRecordField,
  Directory,
  File,
  RecordType,
} from './cwltypes.js';
import { ValidationException, WorkflowException } from './errors.js';
import { _logger } from './loghandler.js';
import { compute_checksums, shortname } from './process.js';
import { StdFsAccess } from './stdfsaccess.js';
import {
  CONTENT_LIMIT,
  CWLObjectType,
  CWLOutputType,
  adjustFileObjs,
  aslist,
  fileUri,
  isDirectory,
  isFile,
  normalizeFilesDirs,
  str,
  uriFilePath,
  visitFile,
  visitFileDirectory,
} from './utils.js';
import { validate } from './validate.js';
export interface OutputPortsType {
  [key: string]: CWLOutputType | undefined;
}
function remove_path(f: File | Directory): void {
  if (f.path) {
    f.path = undefined;
  }
}
function revmap_file(builder: Builder, outdir: string, f: File | Directory): File | Directory | null {
  if (outdir.startsWith('/')) {
    outdir = fileUri(outdir);
  }
  if (f.location && f.location.startsWith('s3://')) {
    f.path = undefined;
    return f;
  }
  if (f.location && !f.path) {
    const location: string = f.location;
    if (location.startsWith('file://')) {
      f.path = uriFilePath(location);
    } else {
      f.location = builder.fs_access.join(outdir, f.location);
      return f;
    }
  }
  if (f['dirname']) {
    delete f['dirname'];
  }
  if (f.path) {
    const path1 = builder.fs_access.join(builder.outdir, f.path);
    const uripath = fileUri(path1);
    f.path = undefined;
    if (!f.basename) {
      f.basename = path.basename(path1);
    }
    if (!builder.pathmapper) {
      throw new Error("Do not call revmap_file using a builder that doesn't have a pathmapper.");
    }
    const revmap_f = builder.pathmapper.reversemap(path1);
    if (revmap_f && !builder.pathmapper.mapper(revmap_f[0]).type.startsWith('Writable')) {
      // If the file is not writable, then we need to copy it to the output directory
      f.location = revmap_f[1];
    } else if (uripath == outdir || uripath.startsWith(outdir + path.sep) || uripath.startsWith(`${outdir}/`)) {
      f.location = uripath;
    } else if (
      path1 == builder.outdir ||
      path1.startsWith(builder.outdir + path.sep) ||
      path1.startsWith(`${builder.outdir}/`)
    ) {
      const joined_path = builder.fs_access.join(
        outdir,
        encodeURIComponent(path1.substring(builder.outdir.length + 1)),
      );
      f.location = joined_path;
    } else {
      console.log("Output File object is missing both 'location' and 'path' fields: ${str(f)}");
      // throw new WorkflowException(
      //   `Output file path ${path1} must be within designated output directory ${builder.outdir} or an input file pass through.`,
      // );
    }
    return f;
  }
  throw new WorkflowException(`Output File object is missing both 'location' and 'path' fields: ${str(f)}`);
}

async function checkValidLocations(fsAccess: StdFsAccess, ob: Directory | File): Promise<void> {
  const location = ob.location;
  if (location.startsWith('_:')) {
    return;
  }
  if (location.startsWith('s3://')) {
    return;
  }
  if (isFile(ob) && !fsAccess.isfile(location)) {
    throw new Error(`Does not exist or is not a File: '${location}'`);
  }
  if (isDirectory(ob)) {
    const isDir = await fsAccess.isdir(location);
    if (!isDir) {
      throw new Error(`Does not exist or is not a Directory: '${location}'`);
    }
  }
}
function wait(ms:number) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

export async function collect_output_ports(
  ports: CommandOutputParameter[],
  fields: CommandOutputRecordField[],
  builder: Builder,
  outdir: string,
  rcode: number,
  isCwlOutput: boolean,
  ret: OutputPortsType,
  _jobname: string,
): Promise<OutputPortsType> {
  // const ret: OutputPortsType = {};
  // const debug = _logger.isDebugEnabled();
  builder.resources['exitCode'] = rcode;
  try {
    // eslint-disable-next-line new-cap
    const fs_access = new builder.make_fs_access(outdir);
    // if (outfiles['cwl.output.json']) {
    //   const outjsons = outfiles['cwl.output.json'];
    //   if (!(outjsons.length == 1 && isFile(outjsons[0]))) {
    //     throw new WorkflowException('Expected cwl.output.json to be a single file');
    //   }
    //   const outjson = outjsons[0];
    //   if (outjson.size > CONTENT_LIMIT) {
    //     const jsonString = await contentLimitRespectedReadBytes(outjson.location, true);
    //     ret = JSON.parse(jsonString);
    //   } else {
    //     ret = JSON.parse(outjson.contents);
    //   }
    //   if (debug) {
    //     _logger.debug(`Raw output from ${outjson.location}: ${JSON.stringify(ret, null, 4)}`);
    //   }
    //   convertDictToFileDirectory(ret);
    // } else
    if (isCwlOutput) {
    } else {
      if (Array.isArray(ports)) {
        const ret2 = {};
        for (let i = 0; i < ports.length; i++) {
          const port = ports[i];
          const fragment = shortname(port.id);
          ret2[fragment] = await collect_output(port, builder, outdir, ret, fs_access);
        }
        ret = ret2;
      }
    }
    if (ret) {
      //   const revmap = (val) => revmap_file(builder, outdir, val);
      //   // adjustDirObjs(ret, trim_listing);
      //   visitFileDirectory(ret, revmap);
      visitFileDirectory(ret, remove_path);
      normalizeFilesDirs(ret);
      const promises = [];
      visitFileDirectory(ret, (val) => promises.push(checkValidLocations(fs_access, val)));
      await Promise.all(promises);
      // const expected_schema = ((this.names.get_name("outputs_record_schema", null)) as Schema);
    }
    validate({ type: RecordType, fields }, ret, true);
    return ret || {};
  } catch (e) {
    if (e instanceof Error) {
      _logger.warn(`Error collecting output: ${e.stack}`);
    }
    if (e instanceof ValidationException) {
      throw new WorkflowException(`Error validating output record. ${e.message}\n in ${JSON.stringify(ret, null, 4)}`);
    }
    throw e;
  }
}
async function handle_output_format(
  schema: CommandOutputParameter,
  builder: Builder,
  result: CWLOutputType,
): Promise<void> {
  if (schema.format) {
    const format_field: string = schema.format;
    if (format_field.includes('$(') || format_field.includes('${')) {
      const results = aslist(result);
      for (let index = 0; index < results.length; index++) {
        const primary = results[index];
        const format_eval: CWLOutputType = await builder.do_eval(format_field, primary);
        if (typeof format_eval !== 'string') {
          let message = `'format' expression must evaluate to a string. Got ${str(format_eval)} from ${format_field}.`;
          if (Array.isArray(result)) {
            message += ` 'self' had the value of the index ${index} result: ${str(primary)}.`;
          }
          throw new WorkflowException(message);
        }
        primary['format'] = format_eval;
      }
    } else {
      aslist(result).forEach((primary) => {
        primary['format'] = format_field;
      });
    }
  }
}
async function collect_output(
  schema: CommandOutputParameter,
  builder: Builder,
  outdir: string,
  outfiles: OutputPortsType,
  fs_access: StdFsAccess,
  compute_checksum = true,
): Promise<CWLOutputType | undefined> {
  const empty_and_optional = false;
  const debug = _logger.isDebugEnabled();
  let result: CWLOutputType | undefined = undefined;
  if (schema.outputBinding) {
    const binding = schema.outputBinding;
    const r = outfiles[schema.name];
    let optional = false;
    let single = false;
    if (Array.isArray(schema.type)) {
      if (schema.type.includes('null')) {
        optional = true;
      }
      if (schema.type.includes('File') || schema.type.includes('Directory')) {
        single = true;
      }
    } else if (schema.type === 'File' || schema.type === 'Directory') {
      single = true;
    }
    // if (binding.outputEval) {
    //   result = await builder.do_eval(binding.outputEval, r);
    // } else {
    result = r as CWLOutputType;
    // }
    if (single) {
      try {
        if (!result && !optional) {
          throw new WorkflowException(`Did not find output file with glob pattern: ${str(binding.glob)}.`);
        } else if (!result && optional) {
          // Do nothing
        } else if (Array.isArray(result)) {
          if (result.length > 1) {
            throw new WorkflowException('Multiple matches for output item that is a single file.');
          } else {
            result = result[0] as CWLOutputType;
          }
        }
      } catch {
        // Log the error here
      }
    }
    await handle_output_format(schema, builder, result);
    // adjustFileObjs(result, revmap);
    if (optional && (!result || (result instanceof Array && result.length === 0))) {
      return result === 0 || result === '' ? result : null;
    }
  }
  if (!result && !empty_and_optional && typeof schema.type === 'object' && schema.type['type'] === 'record') {
    const out = {};
    for (const field of schema['type']['fields'] as CWLObjectType[]) {
      out[shortname(field['name'] as string)] = await collect_output(
        field as unknown as CommandOutputParameter,
        builder,
        outdir,
        outfiles,
        fs_access,
        compute_checksum,
      );
    }
    return out;
  }
  if (compute_checksum) {
    const promises: Promise<void>[] = [];
    visitFile(result, (vals) => promises.push(compute_checksums(fs_access, vals)));
    await Promise.all(promises);
  }

  return result;
}
