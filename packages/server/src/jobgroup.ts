import { RuntimeContext } from "./context"
import { CommandLineJob } from "./job"
import { getManager, JobRequest } from "./server/manager"


export class JobGroup{
    outdir:string
    jobs:CommandLineJob[]
    constructor(jobs:CommandLineJob[]){
        this.jobs = jobs
        this.outdir = jobs[0].outdir
    }
    getOutdirs(){
        const outdirs = [];
        for(const job of this.jobs){
            if(job.outdir){
                outdirs.push(job.outdir)
            }
        }
        return outdirs
    }
    async run(runtimeContext: RuntimeContext, tmpdir_lock?: any): Promise<void> {
        const manager = getManager()
        const rqs:JobRequest[] =[]
        for(const job of this.jobs){
            const jobExec = await job.run2(runtimeContext)
            rqs.push({job:job,jobExec:jobExec})
        }
        await manager.executeJobs(rqs,runtimeContext)
    }
}