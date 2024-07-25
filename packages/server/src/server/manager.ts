import * as fs from 'node:fs';
import * as jsYaml from 'js-yaml';
import { exec } from '../main.js';
import { JobExec } from '../JobExecutor.js';
import { Directory, File } from '../cwltypes.js';
import { WorkflowException } from '../errors.js';
import { CWLOutputType, JobStatus } from '../utils.js';
import { ServerConfig, ServerConfigSchema } from './config.js';
import { _logger } from '../loghandler.js';
import { RuntimeContext } from '../context.js';
import { string } from 'yargs';
import { v4 as uuidv4 } from 'uuid';
import { CommandLineJob } from '../job.js';
import { AsyncFIFOQueue } from '../utils/fifo.js';
import { getJobWatcher } from './job_watcher.js';

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
      if(e instanceof Error){
        console.log(e)
     }
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
  private config: ServerConfig;
  private jobPromises: Map<
    string,
    { job:CommandLineJob,resolve: (value: [number, boolean, Record<string,any>]) => void; reject: (error: Error) => void }
  > = new Map();
  private queuedTasks = new AsyncFIFOQueue<JobExec[]>();
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
  async execute(job:CommandLineJob, jobExec: JobExec): Promise<[number, boolean, Record<string,any>]> {
    this.queuedTasks.push([jobExec]);
    const promise = new Promise<[number, boolean, Record<string,any>]>((resolve, reject) => {
      this.jobPromises.set(jobExec.id, { job,resolve, reject });
    });
    return promise;
  }
  async wait(job:CommandLineJob, jobExec: JobExec): Promise<[number, boolean, Record<string,any>]>{
    const promise = new Promise<[number, boolean, Record<string,any>]>((resolve, reject) => {
      this.jobPromises.set(jobExec.id, { job,resolve, reject });
    });
    return promise
  }
  async executeJobs(rq:JobRequest[],runtimeContext:RuntimeContext): Promise<void> {
    const jobExecs = rq.map((r)=>r.jobExec)
    const promises:Promise<void>[] =  []
    for(const r of rq){
      const promise = new Promise<void>(async (resolve,reject) => {
        try{
          getJobWatcher().jobStarted(r.job)
          const [rcode,isCwlOutput,fileMap] = await this.wait(r.job,r.jobExec)
          await r.job.executed(rcode,isCwlOutput,fileMap,r.jobExec.stdout_path,r.jobExec.stderr_path,runtimeContext)
          resolve()  
        }catch(e){
          reject(e)
        }
      })
      promises.push(promise)
    }
    this.queuedTasks.push(jobExecs);
    await Promise.all(promises)
  }
  async evaluate(id: string, ex: string, context: File | Directory, exitCode?: number): Promise<CWLOutputType> {
    const {job} = this.jobPromises.get(id);
    if (exitCode != undefined) {
      job.resources['exitCode'] = exitCode;
    }
    return job.do_eval(ex, context, false);
  }
  getExecutableJob(): Promise<JobExec[]> {
    return this.queuedTasks.pop(60*1000);
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
export interface JobRequest{
  job:CommandLineJob,
  jobExec: JobExec
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
