import { JobUpdate, type NewJob, db } from "./databases";
import { CommandLineJob, JobBase } from "./job";
import { getJobWatcher, JobListener } from "./server/job_watcher";
import { _logger } from "./loghandler";
import { OutputPortsType } from "./collect_outputs";
import { ExpressionJob } from "./command_line_tool";
import { CommandOutputParameter, toString } from "./cwltypes";
import { JobStatus } from "./utils";
import { FlowyJobURL } from "./flowyurl";
import { Dictionary } from "./maputils";

interface JobInfo{
    id:FlowyJobURL,
    processStatus: JobStatus,
    outputs: OutputPortsType
}
export class JobManager implements JobListener{
    private jobs:Dictionary<FlowyJobURL,JobBase> = new Dictionary()
    async getJobInfo(id:FlowyJobURL):Promise<JobInfo | undefined>{
        const job = this.jobs.get(id)
        if(this.jobs){
            return {
                id: id,
                processStatus: job.processStatus,
                outputs: job.results
            }
        }
        return this.getJobInfoFromDB(id)
    }
    async getJobInfoFromDB(id: FlowyJobURL) :Promise<JobInfo | undefined>{
        const job = await db.selectFrom('job').select(['id','status']).where('id',"=",id.getId()).executeTakeFirst()
        if(!job){
            return undefined
        }
        const results: OutputPortsType = {}
        const outputs = await db.selectFrom('job_output').selectAll().where('job_id',"=",id.getId()).execute()
        for(const out of outputs){
            results[out.name] = JSON.parse(out.value)
        }
        return {
            id,
            processStatus: job.status,
            outputs: results
        }
    }
    async jobCreated(job: JobBase) {
        this.jobs.add(job.id, job)
        const j:NewJob ={
            id:job.id.getId(),
            name: job.name,
            status: "created",
            inputs: JSON.stringify(job.joborder),
            type: job.type,
            parent_id: job.parent_id?.toString()
        }
        console.log(`type=${job.type}`)
        await db.insertInto('job').values(j).execute()
    }
    async jobFinished(job: JobBase,rcode:number,outputs:OutputPortsType) {
        this.jobs.add(job.id, job)
        const j: JobUpdate ={
            status: 'success',
            exitCode: rcode,
        }
        _logger.info(`job ${job.id} ${job.name} finished with code ${rcode}`)
        await db.updateTable('job').set(j).where('id',"=",job.id.getId()).execute()
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
                    job_id: job.id.getId(),
                    name: bind.id,
                    type: toString(bind.type),
                    value: JSON.stringify(output)
                }
                await db.insertInto('job_output').values(jout).execute()
                }
        }
    }
    async jobStarted(job: JobBase) {
        this.jobs.add(job.id, job)
        const j: JobUpdate ={
            status: 'started',
        }
        await db.updateTable('job').set(j).where('id',"=",job.id.toString()).execute()        
    }
}
const jobManager = new JobManager()
getJobWatcher().addListener(jobManager);
export function getJobManager(){
    return jobManager;
}