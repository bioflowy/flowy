import { JobUpdate, type NewJob, db } from "./databases";
import { CommandLineJob, JobBase } from "./job";
import { getJobWatcher, JobListener } from "./server/job_watcher";
import { _logger } from "./loghandler";
import { OutputPortsType } from "./collect_outputs";
import { Expression } from "@flowy/cwl-ts-auto";
import { ExpressionJob } from "./command_line_tool";
import { CommandOutputParameter, toString } from "./cwltypes";
import { JobStatus } from "./utils";

interface JobInfo{
    id:string,
    processStatus: JobStatus,
    outputs: OutputPortsType
}
export class JobManager implements JobListener{
    private jobs:{[key:string]:JobBase} = {}
    async getJobInfo(id:string):Promise<JobInfo | undefined>{
        if(id in this.jobs){
            const job = this.jobs[id]
            return {
                id: job.id,
                processStatus: job.processStatus,
                outputs: job.results
            }
        }
        return this.getJobInfoFromDB(id)
    }
    async getJobInfoFromDB(id: string) :Promise<JobInfo | undefined>{
        const job = await db.selectFrom('job').select(['id','status']).where('id',"=",id).executeTakeFirst()
        if(!job){
            return undefined
        }
        const results: OutputPortsType = {}
        const outputs = await db.selectFrom('job_output').selectAll().where('job_id',"=",id).execute()
        for(const out of outputs){
            results[out.name] = JSON.parse(out.value)
        }
        return {
            id: job.id,
            processStatus: job.status,
            outputs: results
        }
    }
    async jobCreated(job: JobBase) {
        this.jobs[job.id] = job
        const j:NewJob ={
            id:job.id,
            name: job.name,
            status: "Created",
            inputs: JSON.stringify(job.joborder),
            type: job.type,
            parent_id: job.parent_id
        }
        console.log(`type=${job.type}`)
        await db.insertInto('job').values(j).execute()
    }
    async jobFinished(job: JobBase,rcode:number,outputs:OutputPortsType) {
        this.jobs[job.id] = job
        const j: JobUpdate ={
            status: 'Finished',
            exitCode: rcode,
        }
        _logger.info(`job ${job.id} ${job.name} finished with code ${rcode}`)
        await db.updateTable('job').set(j).where('id',"=",job.id).execute()
        let bindings:CommandOutputParameter[] = [];
        if(job instanceof CommandLineJob){
            bindings = job.tool.outputs
        }else if(job instanceof ExpressionJob){
            bindings = job.tool.outputs
        }
        for(const bind of bindings){
            if(bind.name in outputs){
                const output = outputs[bind.name]
                const jout = {
                    id: `${job.id}-${bind.id}`,
                    job_id: job.id,
                    name: bind.id,
                    type: toString(bind.type),
                    value: JSON.stringify(output)
                }
                await db.insertInto('job_output').values(jout).execute()
                }
        }
    }
    async jobStarted(job: JobBase) {
        this.jobs[job.id] = job
        const j: JobUpdate ={
            status: 'Started',
        }
        await db.updateTable('job').set(j).where('id',"=",job.id).execute()        
    }
}
const jobManager = new JobManager()
getJobWatcher().addListener(jobManager);
export function getJobManager(){
    return jobManager;
}