import { CommandLineJob, JobBase } from "../job";
import { JobManager } from "../jobmanager";
import { CWLOutputType } from "../utils";
import { AsyncFIFOQueue } from "../utils/fifo";

export interface JobListener{
    jobCreated(job:JobBase):Promise<void>
    jobStarted(job:JobBase):Promise<void>
    jobFinished(job:JobBase, rcode: number, outputs: CWLOutputType):Promise<void>
}
type JobCreated = {
    type :"Created"
    job: JobBase
}
type JobStarted = {
    type :"Started"
    job: JobBase
}
type jobFinished = {
    type :"Finished"
    job: JobBase,
    rcode: number, 
    outputs: CWLOutputType
}
export class JobWatcher {
    private eventQueue = new AsyncFIFOQueue<JobCreated|JobStarted|jobFinished>()
    private listeners: JobListener[] = []

    addListener(listener:JobListener){
        this.listeners.push(listener)
    }
    jobCreated(job:JobBase){
        this.eventQueue.push({type:"Created",job:job})
    }
    jobStarted(job:JobBase){
        this.eventQueue.push({type:"Started",job:job})
    }
    jobFinished(job: JobBase, rcode: number, outputs: CWLOutputType){
        this.eventQueue.push({type:"Finished",job:job,rcode:rcode,outputs:outputs})
    }
    async dispatch():Promise<void>{
        while(true){
            const event = await this.eventQueue.pop()
            switch(event.type){
                case "Created":
                    for(const l of this.listeners){
                        await l.jobCreated(event.job);
                    }
                    break;
                case "Started":
                    for(const l of this.listeners){
                        await l.jobStarted(event.job);
                    }
                    break;
                case "Finished":
                    for(const l of this.listeners){
                        await l.jobFinished(event.job,event.rcode,event.outputs);
                    }
                    break;
            }
        }
    }
}
const watcher = new JobWatcher()
void watcher.dispatch()
export function getJobWatcher(){
    return watcher
}
