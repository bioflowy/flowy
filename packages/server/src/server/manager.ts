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
  results?:CWLOutputType;
  resultStatus?: JobStatus;
}
export class CommandLineJobManager {
  jobStatusChange(jobId:string,results:CWLOutputType,status:JobStatus){
    const job = this.queuedJobs[jobId]
    job.results = results
    job.resultStatus = status
    job.status = "finished"
  }
  getJobInfo(jobId:string):SubmitJob{
    return this.queuedJobs[jobId]
  }
  private queuedJobs: Map<string,CommandLineJob> = new Map();
  private config: ServerConfig;
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
  async execute(job:CommandLineJob, jobExec: JobExec,runtimeContext:RuntimeContext) {
    return this.executeJobs([{job:job,jobExec:jobExec}],runtimeContext);
  }
  async executeJobs(requests:JobRequest[],runtimeContext:RuntimeContext) {
    const jobExecs = requests.map((r)=>r.jobExec)
    for(const rq of requests){
      this.queuedJobs[rq.job.id] = rq.job
    }
    this.queuedTasks.push(jobExecs);
  }
  async evaluate(id: string, ex: string, context: File | Directory, exitCode?: number): Promise<CWLOutputType> {
    const job = this.queuedJobs.get(id);
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
    const job = this.queuedJobs[id];
    if (job) {
      this.queuedJobs.delete(id);
      job.executed(ret_code,isCwlOutput,outputResults)
    }
  }
  jobfailed(id: string, errorMsg: string) {
    const job = this.queuedJobs.get(id);
    if (job) {
      this.queuedJobs.delete(id);
      job.executed(1,false,errorMsg)
    }
  }
}
export interface JobRequest{
  job:CommandLineJob,
  jobExec: JobExec
}

let manager: CommandLineJobManager;
export function getManager(): CommandLineJobManager {
  if (!manager) {
    manager = new CommandLineJobManager();
  }
  return manager;
}
export function initManager(configPath: string = 'config.yml'): CommandLineJobManager {
  _logger.info(`initManager by ${configPath}`);
  manager = new CommandLineJobManager(configPath);
  return manager;
}
