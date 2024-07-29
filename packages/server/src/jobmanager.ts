import { RuntimeContext } from "./context";
import { JobUpdate, type NewJob, db } from "./databases";
import { CommandLineJob, JobBase } from "./job";
import { exec } from "./main";
import { getJobWatcher, JobListener, JobWatcher } from "./server/job_watcher";


export class JobManager implements JobListener{
    private jobs:{[key:string]:JobBase} = {}
    getJobInfo(id:string){
        return this.jobs[id]
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
    async jobFinished(job: JobBase,rcode:number,output:any) {
        this.jobs[job.id] = job
        const j: JobUpdate ={
            status: 'Finished',
            exitCode: rcode,
            outputs: JSON.stringify(output),
        }
        await db.updateTable('job').set(j).where('id',"=",job.id).execute()

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