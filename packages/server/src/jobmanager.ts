import { JobUpdate, type NewJob, db } from "./databases";
import { CommandLineJob, JobBase } from "./job";
import { JobListener, JobWatcher } from "./server/job_watcher";


export class JobManager implements JobListener{
    async jobCreated(job: JobBase) {
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
        const j: JobUpdate ={
            status: 'Finished',
            exitCode: rcode,
            outputs: JSON.stringify(output),
        }
        await db.updateTable('job').set(j).where('id',"=",job.id).execute()

    }
    async jobStarted(job: JobBase) {
        const j: JobUpdate ={
            status: 'Started',
        }
        await db.updateTable('job').set(j).where('id',"=",job.id).execute()        
    }
}
