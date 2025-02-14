import * as child_process from 'node:child_process';
import * as fs from 'node:fs';
import * as os from 'node:os';
import * as path from 'node:path';
import * as url from 'node:url';

import * as cwl from '@flowy/cwl-ts-auto';
import { v4 as uuidv4 } from 'uuid';
import { ExpressionJob } from './command_line_tool.js';
import { Directory, File, WorkflowInputParameter } from './cwltypes.js';
import { ValidationException } from './errors.js';
import { CommandLineJob, JobBase } from './job.js';
import { StdFsAccess } from './stdfsaccess.js';
import type { ToolRequirement } from './types.js';
import type { WorkflowJob } from './workflow_job.js';
import { CommandString } from './commandstring.js';
import { JobGroup } from './jobgroup.js';

let __random_outdir: string | null = null;

export const CONTENT_LIMIT = 64 * 1024;

export const DEFAULT_TMP_PREFIX = '/tmp/';

export type MutableSequence<T> = T[];
export type MutableMapping<T> = {
  [key: string]: T;
};
export type CWLOutputAtomType =
  | undefined
  | boolean
  | string
  | number
  | File
  | Directory
  | MutableSequence<undefined | boolean | string | number | MutableSequence<any> | MutableMapping<any>>
  | MutableMapping<undefined | boolean | string | number | MutableSequence<any> | MutableMapping<any>>;

export type CWLOutputType =
  | boolean
  | string
  | number
  | File
  | Directory
  | MutableSequence<CWLOutputAtomType>
  | MutableMapping<CWLOutputAtomType>;
export type CWLObjectType = MutableMapping<CWLOutputType | undefined>;

export type JobsType = JobBase; //  ;
export type JobsGeneratorType = AsyncGenerator<JobsType, void>;
export type JobStatus = "created" | "queued" | "started" | "success" | "permanentFail" | "temporaryFail";
export type OutputCallbackType = (arg1: CWLObjectType, arg2: JobStatus) => void;
// type ResolverType = (Loader, string)=>string?;
// type DestinationsType = MutableMapping<string, CWLOutputType?>;
export type ScatterDestinationsType = MutableMapping<(CWLOutputType | undefined)[]>;
export type ScatterOutputCallbackType = (arg1: ScatterDestinationsType, arg2: string) => void;
export type SinkType = CWLOutputType | CWLObjectType;
export type DirectoryType = {
  class: string;
  listing: CWLObjectType[];
  basename: string;
};
// type JSONAtomType = MutableMapping<string, any> | MutableSequence<any> | string| number| boolean| null;
// type JSONType = MutableMapping<string, JSONAtomType>| MutableSequence<JSONAtomType>| string| number| boolean| null;
// type WorkflowStateItem = NamedTuple<
//     'WorkflowStateItem',
//     [
//         ['parameter', CWLObjectType],
//         ['value', Optional<CWLOutputType>],
//         ['success', string]
//     ]
// >;
export class WorkflowStateItem {
  // / """Workflow state item."""

  parameter: WorkflowInputParameter;
  value?: CWLOutputType;
  success: string;
  constructor(parameter: WorkflowInputParameter, value: CWLOutputType | undefined, success: string) {
    this.parameter = parameter;
    this.value = value;
    this.success = success;
  }
}
export function isString(value: unknown): value is string {
  return typeof value === 'string';
}
export function isStringOrStringArray(value: unknown): value is string | string[] {
  if (Array.isArray(value)) {
    return value.every((v) => typeof v === 'string');
  } else {
    return typeof value === 'string';
  }
}
export function urldefrag(url: string): { url: string; fragment: string } {
  const [urlWithoutFragment, fragment] = url.split('#');
  return { url: urlWithoutFragment, fragment: fragment || '' };
}
export type ParametersType = CWLObjectType[];
export type StepType = CWLObjectType;

export type LoadListingType = 'no_listing' | 'shallow_listing' | 'deep_listing';
export async function which(cmd: string): Promise<string | null> {
  return new Promise((resolve) => {
    child_process.exec(`which ${cmd}`, (error, stdout) => {
      if (error) {
        resolve(null);
        return;
      }
      resolve(stdout.trim());
    });
  });
}
export function fileUri(inputPath: string, splitFrag = false): string {
  if (inputPath.startsWith('file://')) {
    return inputPath;
  }
  let frag = '';
  let urlPath: string;
  if (splitFrag) {
    const pathSp = inputPath.split('#', 2);
    if (pathSp.length === 2) {
      frag = `#${encodeURIComponent(pathSp[1])}`;
    }
    urlPath = pathToFileURL(pathSp[0]);
  } else {
    urlPath = pathToFileURL(inputPath);
  }
  if (urlPath.startsWith('/')) {
    return `file:${urlPath}${frag}`;
  }
  return `${urlPath}${frag}`;
}
export function copyTree(src: string, dest: string): void {
  if (!fs.existsSync(dest)) {
    fs.mkdirSync(dest, { recursive: true });
  }

  const entries = fs.readdirSync(src, { withFileTypes: true });

  for (const entry of entries) {
    const srcPath = path.join(src, entry.name);
    const destPath = path.join(dest, entry.name);

    if (entry.isDirectory()) {
      copyTree(srcPath, destPath);
    } else {
      fs.copyFileSync(srcPath, destPath);
    }
  }
}
export async function checkOutput(commands: string[]): Promise<string> {
  return new Promise((resolve, reject) => {
    child_process.exec(commands.join(' '), (error, stdout, stderr) => {
      if (error) {
        reject(new Error(`Command failed: ${stderr || error.message}`));
        return;
      }
      resolve(stdout);
    });
  });
}
export function uriFilePath(inputUrl: string): string {
  const split = new url.URL(inputUrl);
  if (split.protocol === 'file:') {
    return `${fileURLToPath(split.href)}${split.hash ? `#${decodeURIComponent(split.hash.slice(1))}` : ''}`;
  }
  throw new Error(`Not a file URI: ${inputUrl}`);
}

function pathToFileURL(inputPath: string): string {
  return new url.URL(`file://${path.resolve(inputPath)}`).toString();
}

function fileURLToPath(inputUrl: string): string {
  const u = new url.URL(inputUrl);
  if (u.protocol !== 'file:' && u.protocol !== 's3:') {
    throw new Error(`Not a file URL: ${inputUrl}`);
  }
  return decodeURIComponent(u.pathname);
}
export function splitPath(filePath: string): [string, string] {
  const idx = filePath.lastIndexOf('/');
  if (idx === -1) {
    return ['', filePath];
  }
  return [filePath.substring(0, idx), filePath.substring(idx + 1)];
}
export function mkdtemp(prefix = '', dir?: string): string {
  if (!dir) {
    dir = DEFAULT_TMP_PREFIX;
  }
  const uniqueName = prefix + uuidv4();
  const tempDirPath = path.join(dir, uniqueName);
  return tempDirPath;
}
export function versionstring(): string {
  return `flowy-cwl v1.0`;
}

export function aslist<T>(thing: T | T[]): T[] {
  if (Array.isArray(thing)) {
    return thing;
  }
  if (thing === undefined || thing == null) {
    return [];
  }
  return [thing];
}
// equivalent to  os.path.split in python
function osPathSplit(path_str: string): [string, string] {
  if (path_str.endsWith('/')) {
    return [path_str.substring(0, path_str.length - 1), ''];
  } else {
    return [path.dirname(path_str), path.basename(path_str)];
  }
}
export function createTmpDir(tmpdirPrefix: string): string {
  const [tmpDir, tmpPrefix] = osPathSplit(tmpdirPrefix);

  // デフォルトのtmpディレクトリを使用する場合
  const finalTmpDir = tmpDir || os.tmpdir();

  // 一時ディレクトリを作成する
  let p = path.join(finalTmpDir, tmpPrefix);
  if (!p.endsWith('/')) {
    p = `${p}/`;
  }
  const fullTmpDir = fs.mkdtempSync(p);

  return fullTmpDir;
}

export function copytree_with_merge(src: string, dst: string): void {
  if (!fs.existsSync(dst)) {
    fs.mkdirSync(dst);
    fs.copyFileSync(src, dst);
  }
  const lst = fs.readdirSync(src);
  for (const item of lst) {
    const spath = path.join(src, item);
    const dpath = path.join(dst, item);
    if (fs.statSync(spath).isDirectory()) {
      copytree_with_merge(spath, dpath);
    } else {
      fs.copyFileSync(spath, dpath);
    }
  }
}
/**
 * Converts an object to a JSON-formatted string.
 * To pass the conformance_test, it is formatted to match the output of Python's json.dumps.
 *
 * @param obj - The object to be converted.
 * @returns The JSON-formatted string.
 */
export function josonStringifyLikePython(obj: unknown): string {
  if (obj === undefined) {
    return 'null';
  }
  if (obj instanceof Object) {
    if (obj instanceof Array) {
      const str = obj.map((item) => josonStringifyLikePython(item)).join(', ');
      return `[${str}]`;
    }
    const str = Object.keys(obj)
      .map((key) => `${JSON.stringify(key)}: ${josonStringifyLikePython(obj[key])}`)
      .join(', ');
    return `{${str}}`;
  }
  return JSON.stringify(obj, null, '');
}
export function isInstanceOf<T>(input: unknown, constructor: { new (...args: unknown[]): T }): input is T {
  return input instanceof constructor;
}
export function isInstanceOfAny(input: unknown, constructors: { new (...args: unknown[]): unknown }[]): boolean {
  for (const constructor of constructors) {
    if (input instanceof constructor) {
      return true;
    }
  }
  return false;
}
export function visitClass<T>(input: unknown, isT: (value: unknown) => value is T, callback: (arg: T) => void) {
  if (Array.isArray(input)) {
    input.forEach((item) => visitClass(item, isT, callback));
  } else {
    if (isT(input)) {
      callback(input);
    }
    if (typeof input === 'object' && input !== null) {
      for (const key in input) {
        // eslint-disable-next-line no-prototype-builtins
        if (input.hasOwnProperty(key)) {
          visitClass(input[key], isT, callback);
        }
      }
    }
  }
}
export function isFile(value: unknown): value is File {
  return value && typeof value === 'object' && 'class' in value && value['class'] === 'File';
}
export function isDirectory(value: unknown): value is Directory {
  return value && typeof value === 'object' && 'class' in value && value['class'] === 'Directory';
}
export function isFileOrDirectory(value: unknown): value is File | Directory {
  return (
    value &&
    typeof value === 'object' &&
    'class' in value &&
    ('Directory' === value['class'] || 'File' === value['class'])
  );
}
export const visitFile = (rec: unknown, callback: (f: File) => void) => visitClass<File>(rec, isFile, callback);
export const visitDirectory = (rec: unknown, callback: (f: Directory) => void) =>
  visitClass<Directory>(rec, isDirectory, callback);
export const visitFileDirectory = (rec: unknown, callback: (f: File | Directory) => void) =>
  visitClass(rec, isFileOrDirectory, callback);

export function filePathToURI(filePath: string): string {
  let flag: string | undefined = undefined;
  const splits = filePath.split('#');
  if (splits.length > 1) {
    filePath = splits[0];
    flag = splits[1];
  }
  const pathName = path.resolve(filePath).replace(/\\/gu, '/');
  return url.format({
    protocol: 'file',
    slashes: true,
    pathname: pathName,
    hash: flag,
  });
}

export function random_outdir(): string {
  if (!__random_outdir) {
    __random_outdir = `/${Array.from({ length: 6 }, () => Math.random().toString(36)[2]?.toUpperCase()).join('')}`;
    return __random_outdir;
  }
  return __random_outdir;
}

export const adjustFileObjs = (rec: unknown, op: (dir: File) => void) => visitClass(rec, isFile, op);

export const adjustDirObjs = (rec: unknown, op: (dir: Directory) => void) => visitClass(rec, isDirectory, op);
const _find_unsafe = /[^a-zA-Z0-9@%+=:,./-]/u;
export function quote(s: string): string {
  /** Return a shell-escaped version of the string *s*. */
  if (!s) {
    return "''";
  }
  if (!_find_unsafe.test(s)) {
    return s;
  }

  // use single quotes, and put single quotes into double quotes
  // the string $'b is then quoted as '$'"'"'b'
  return `'${s.replace(/'/gu, "'\"'\"'")}'`;
}
export function urlJoin(...parts: string[]): string {
  return parts.reduce((accumulator, part) => {
    if (!accumulator) return part;

    const accSlash = accumulator.endsWith('/');
    const partSlash = part.startsWith('/');

    if (accSlash && partSlash) {
      return accumulator + part.slice(1);
    } else if (!accSlash && !partSlash) {
      return `${accumulator}/${part}`;
    } else {
      return accumulator + part;
    }
  }, '');
}

export function dedup(listing: (File | Directory)[]): (File | Directory)[] {
  const marksub = new Set();

  for (const entry of listing) {
    if (entry instanceof cwl.Directory) {
      for (const e of entry.listing || []) {
        adjustFileObjs(e, (e) => marksub.add(e.location));
        adjustDirObjs(e, (e) => marksub.add(e.location));
      }
    }
  }

  const dd: (File | Directory)[] = [];
  const markdup = new Set();

  for (const r of listing) {
    if (!marksub.has(r.location) && !markdup.has(r.location)) {
      dd.push(r);
      markdup.add(r.location);
    }
  }

  return dd;
}
function url2pathname(url: string): string {
  const myURL = new URL(url);

  // On Windows, Node.js's URL uses '/' as path separator. We should convert it to the correct one.
  if (path.sep === '\\') {
    return myURL.pathname.split('/').join('\\').slice(1);
  } else {
    return myURL.pathname;
  }
}

export async function get_listing(fs_access: StdFsAccess, rec: unknown, recursive = true) {
  if (!isDirectory(rec)) {
    const finddirs: Directory[] = [];
    adjustDirObjs(rec, (val) => finddirs.push(val));
    for (let _i = 0, finddirs_1 = finddirs; _i < finddirs_1.length; _i++) {
      const f = finddirs_1[_i];
      await get_listing(fs_access, f, recursive);
    }
    return;
  }
  if (rec.listing) {
    return;
  }
  const listing: (File | Directory)[] = [];
  const loc = rec.location;
  const _b = await fs_access.listdir(loc);
  for (let _a = 0; _a < _b.length; _a++) {
    const ld = _b[_a];
    const bn = path.basename(url2pathname(ld));
    const isDir = await fs_access.isdir(ld);
    if (isDir) {
      const ent: Directory = {
        class: 'Directory',
        location: ld,
        basename: bn,
      };
      if (recursive) {
        await get_listing(fs_access, ent, recursive);
      }
      listing.push(ent);
    } else {
      listing.push({ class: 'File', location: ld, basename: bn });
    }
  }
  rec.listing = listing;
}
export function isMissingOrNull(obj: object, key: string) {
  return !(key in obj) || obj[key] === null;
}
export function downloadHttpFile(_httpurl: string): [string, Date] {
  // TODO
  // let cache_session = null;
  // let directory;
  // if ("XDG_CACHE_HOME" in process.env) {
  //     directory = process.env.XDG_CACHE_HOME;
  // }
  // else if ("HOME" in process.env) {
  //     directory = process.env.HOME;
  // }
  // else {
  //     directory = require("os").homedir();
  // }
  // cache_session = new CacheControl(requests.Session(), {
  //     cache: new FileCache(path.join(directory, ".cache", "cwltool"))
  // });
  // const r = cache_session.get(httpurl, {
  //     stream: true
  // });
  // const f = tmp.fileSync({ mode: "wb" });
  // const tempFilePath = f.name;
  // for (const chunk of r.iter_content({
  //     chunk_size: 16384
  // })) {
  //     if (chunk) {
  //         f.writeSync(chunk);
  //     }
  // }
  // r.close();
  // const date_raw = r.headers.get("Last-Modified");
  // const date = date_raw ? parsedate_to_datetime(date_raw) : null;
  // if (date) {
  //     const date_epoch = date.getTime() / 1000;
  //     fs.utimesSync(tempFilePath, date_epoch, date_epoch);
  // }
  return ['tempFilePath', new Date()];
}
export function str<T>(val: T): string {
  return JSON.stringify(val, null, 4);
}
export function ensureWritable(targetPath: string, includeRoot = false): void {
  if (targetPath.startsWith('s3://')) {
    return;
  }
  //
  // Ensure that 'path' is writable.
  //
  // If 'path' is a directory, then all files and directories under 'path' are
  // made writable, recursively. If 'path' is a file or if 'include_root' is
  // `True`, then 'path' itself is made writable.
  //

  function addWritableFlag(p: string): void {
    const mode = fs.statSync(p).mode;
    const newMode = mode | 0o200; // Adding write permission for the owner
    fs.chmodSync(p, newMode);
  }

  if (fs.statSync(targetPath).isDirectory()) {
    if (includeRoot) {
      addWritableFlag(targetPath);
    }

    fs.readdirSync(targetPath).forEach((item) => {
      const itemPath = path.join(targetPath, item);
      if (fs.statSync(itemPath).isDirectory()) {
        ensureWritable(itemPath, true); // Recursive call for directories
      } else {
        addWritableFlag(itemPath); // Directly add flag for files
      }
    });
  } else {
    addWritableFlag(targetPath);
  }
}
export function trim_listing(obj: Directory) {
  //
  // Remove 'listing' field from Directory objects that are file references.
  //
  // It redundant and potentially expensive to pass fully enumerated Directory
  // objects around if not explicitly needed, so delete the 'listing' field when
  // it is safe to do so.
  //
  const location = obj.location;
  if (location && location.startsWith('file://') && obj.listing) {
    obj.listing = undefined;
  }
}

/**
 * parse id(file:///home/foo/bar.cwl#step1/name1) and return name(name1)
 * @param id
 * @returns
 */
export function get_filed_name(id: string): string {
  const name = id.substring(id.indexOf('#'), id.length).split('/').pop();
  return name;
}

export function ensure_non_writable(targetPath: string): void {
  function removeWritableFlag(p: string): void {
    const mode = fs.statSync(p).mode;
    // Remove write permissions for owner, group, and others
    const newMode = mode & ~0o200 & ~0o020 & ~0o002;
    fs.chmodSync(p, newMode);
  }

  if (fs.statSync(targetPath).isDirectory()) {
    fs.readdirSync(targetPath).forEach((item) => {
      const itemPath = path.join(targetPath, item);
      removeWritableFlag(itemPath); // Remove write permissions

      if (fs.statSync(itemPath).isDirectory()) {
        ensure_non_writable(itemPath); // Recursive call for directories
      }
    });
  } else {
    removeWritableFlag(targetPath);
  }
}
export function splitext(p: string): [string, string] {
  const ext = path.extname(p);
  const base = p.substring(0, p.length - ext.length);
  return [base, ext];
}
export function normalizeFilesDirs(job: unknown) {
  function addLocation(d: File | Directory) {
    if (!d.location) {
      if (isFile(d) && d.contents === undefined) {
        throw new ValidationException("Anonymous file object must have 'contents' and 'basename' fields.");
      }
      if (isDirectory(d) && (d.listing === undefined || d.basename === undefined)) {
        throw new ValidationException("Anonymous directory object must have 'listing' and 'basename' fields.");
      }
      d.location = `_:${uuidv4()}`;
      if (!d.basename) {
        d.basename = d.location.substring(2);
      }
    }

    let path2 = d.location;
    try {
      path2 = fileURLToPath(d.location);
    } catch {}
    // strip trailing slash
    if (path2.endsWith('/')) {
      if (!isDirectory(d)) {
        throw new ValidationException(`location '${d.location}' ends with '/' but is not a Directory`);
      }
      path2 = d.location.slice(0, -1);
      d.location = path2;
    }

    if (!d.basename) {
      if (path2.startsWith('_:')) {
        d.basename = path2.substring(2);
      } else {
        d.basename = path.basename(path2);
      }
    }

    if (isFile(d)) {
      const [nr, ne] = splitext(d.basename);
      if (d.nameroot !== nr) {
        d.nameroot = String(nr);
      }
      if (d.nameext !== ne) {
        d.nameext = String(ne);
      }
    }
  }

  visitFileDirectory(job, addLocation);
}
function reversed<T>(arrays: T[]): T[] {
  return [...arrays].reverse();
}
export function pathJoin(base: string, name: string) {
  if (base.startsWith('s3://') || base.startsWith("file://")) {
    if(base.endsWith('/')){
      return `${base}${name}`;
    }else{
      return `${base}/${name}`;
    }
  } else {
    return path.join(base, name);
  }
}
export interface RequirementParam {
  requirements?: undefined | ToolRequirement;
  hints?: undefined | ToolRequirement;
}

export function getRequirement<T>(reqs: RequirementParam, cls: new (any) => T): [T | undefined, boolean] {
  if (reqs.requirements) {
    const req = reversed(reqs.requirements).find((item) => item instanceof cls);
    if (req) {
      return [req as T, true];
    }
  }
  if (reqs.hints) {
    const req = reversed(reqs.hints).find((item) => item instanceof cls);

    if (req) {
      return [req as T, false];
    }
  }
  return [undefined, false];
}

export class HasReqsHints {
  // Base class for get_requirement().
  requirements: CWLObjectType[] = [];
  hints: CWLObjectType[] = [];

  public get_requirement(feature: string): [CWLObjectType | undefined, boolean | undefined] {
    // / Retrieve the named feature from the requirements field, or the hints field."""
    for (const item of reversed(this.requirements)) {
      if (item['class'] == feature) {
        return [item, true];
      }
    }
    for (const item of reversed(this.hints)) {
      if (item['class'] == feature) {
        return [item, false];
      }
    }
    return [undefined, undefined];
  }
}
