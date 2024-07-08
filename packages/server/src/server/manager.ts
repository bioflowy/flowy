import * as fs from 'node:fs';
import * as jsYaml from 'js-yaml';
import { exec } from '../main.js';
import { JobExec } from '../JobExecutor.js';
import { Builder } from '../builder.js';
import { Directory, File } from '../cwltypes.js';
import { WorkflowException } from '../errors.js';
import { CWLOutputType, JobStatus } from '../utils.js';
import { ServerConfig, ServerConfigSchema } from './config.js';
import { _logger } from '../loghandler.js';
import { RuntimeContext } from '../context.js';
import { string } from 'yargs';
import { v4 as uuidv4 } from 'uuid';
import { JobBase } from '../job.js';

interface SubmitJob {
  readonly jobId: string;
  readonly tool_path:string;
  readonly job_path:string;
  status:"queued" | "running" | "finished";
  results?:CWLOutputType;
  resultStatus?: JobStatus;
}
export class Manager {
  queueJob(runtimeContext: RuntimeContext, tool_path: string, job_path: string):string {
    const job:SubmitJob = {
      jobId: uuidv4(),
      tool_path: tool_path,
      job_path: job_path,
      status: "queued"
    }
    this.queuedJobs[job.jobId] = job;
    exec(runtimeContext,tool_path,job_path).then(([results,status])=>{
      this.jobStatusChange(job.jobId,results,status)
    }).catch(e=>{
      this.jobStatusChange(job.jobId,e,"permanentFail")
    });
    return job.jobId
  }
  jobStatusChange(jobId:string,results:CWLOutputType,status:JobStatus){
    const job = this.queuedJobs[jobId]
    job.results = results
    job.resultStatus = status
    job.status = "finished"
  }
  getJobInfo(jobId:string):SubmitJob{
    return this.queuedJobs[jobId]
  }
  private queuedJobs: Map<string,SubmitJob> = new Map();
  private jobWatcher: Map<string,Promise<SubmitJob>[]> = new Map();
  private config: ServerConfig;
  private jobPromises: Map<
    string,
    { job:JobBase,resolve: (value: [number, boolean, Record<string,any>]) => void; reject: (error: Error) => void }
  > = new Map();
  private queuedTasks: JobExec[] = [];
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
  async execute(job:JobBase, jobExec: JobExec): Promise<[number, boolean, Record<string,any>]> {
    this.queuedTasks.push(jobExec);
    const promise = new Promise<[number, boolean, Record<string,any>]>((resolve, reject) => {
      this.jobPromises.set(jobExec.id, { job,resolve, reject });
    });
    return promise;
  }
  async evaluate(id: string, ex: string, context: File | Directory, exitCode?: number): Promise<CWLOutputType> {
    const {job} = this.jobPromises.get(id);
    if (exitCode != undefined) {
      job.resources['exitCode'] = exitCode;
    }
    return job.do_eval(ex, context, false);
  }
  getExecutableJob(): JobExec | undefined {
    if (this.queuedTasks.length === 0) {
      return undefined;
    } else {
      return this.queuedTasks.pop();
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
      this.jobPromises.delete(id);
      promise.resolve([ret_code, isCwlOutput, outputResults]);
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
  _logger.info(`initManager by ${configPath}`);
  manager = new Manager(configPath);
  return manager;
}
