import * as fs from 'node:fs';
import * as jsYaml from 'js-yaml';
import { JobExec } from '../JobExecutor.js';
import { Builder } from '../builder.js';
import { Directory, File } from '../cwltypes.js';
import { WorkflowException } from '../errors.js';
import { CWLOutputType } from '../utils.js';
import { ServerConfig, ServerConfigSchema } from './config.js';

export class Manager {
  private config: ServerConfig;
  private builders: Map<string, Builder> = new Map();
  private executableJobs: JobExec[] = [];
  private jobPromises: Map<
    string,
    { resolve: (value: [number, boolean, Record<string,any>]) => void; reject: (error: Error) => void }
  > = new Map();
  constructor(setings: string = 'config.yml') {
    this.config = ServerConfigSchema.parse(jsYaml.load(fs.readFileSync(setings, 'utf-8')));
  }
  getServerConfig(): ServerConfig {
    return this.config;
  }
  async initialize(configPath: string = 'config.yml') {
    const data = jsYaml.load(fs.readFileSync(configPath, 'utf-8'));
    this.config = ServerConfigSchema.parse(data);
  }
  addBuilder(id: string, builder: Builder) {
    this.builders.set(id, builder);
  }
  async execute(id: string, job: JobExec): Promise<[number, boolean, Record<string,any>]> {
    this.executableJobs.push(job);
    const promise = new Promise<[number, boolean, Record<string,any>]>((resolve, reject) => {
      this.jobPromises.set(id, { resolve, reject });
    });
    return promise;
  }
  async evaluate(id: string, ex: string, context: File | Directory): Promise<CWLOutputType|undefined> {
    const builder = this.builders.get(id);
    if (!builder) {
      return undefined;
    }
    return builder.do_eval(ex, context, false);
  }
  getExecutableJob(): JobExec | undefined {
    if (this.executableJobs.length === 0) {
      return undefined;
    } else {
      return this.executableJobs.pop();
      // return this.executableJobs[0];
    }
  }
  jobfinished(
    id: string,
    ret_code: number,
    isCwlOutput: boolean,
    outputResults: { [key: string]: (File | Directory)[] },
  ) {
    const promise = this.jobPromises.get(id);
    if (promise) {
      promise.resolve([ret_code, isCwlOutput, outputResults]);
      this.jobPromises.delete(id);
    }
  }
  jobfailed(id: string, errorMsg: string) {
    const promise = this.jobPromises.get(id);
    if (promise) {
      promise.reject(new WorkflowException(errorMsg));
      this.jobPromises.delete(id);
    }
  }
}
let manager: Manager;
export function getManager(): Manager {
  if (!manager) {
    manager = new Manager();
  }
  return manager;
}
export function initManager(configPath: string = 'config.yml'): Manager {
  manager = new Manager(configPath);
  return manager;
}
