import * as path from 'node:path';
import type { Logger } from 'winston';
import { RuntimeContext, getDefault } from './context.js';
import { ValidationException, WorkflowException } from './errors.js';
import { _logger } from './loghandler.js';
import { Process, cleanIntermediate, relocateOutputs } from './process.js';
import { createRequirements } from './types.js';
import { JobStatus, type CWLObjectType, type MutableSequence } from './utils.js';
import { JobBase } from './job.js';

abstract class JobExecutor {
  final_output: MutableSequence<CWLObjectType | undefined>;
  final_status: JobStatus[];
  output_dirs: string[];

  constructor() {
    this.final_output = [];
    this.final_status = [];
    this.output_dirs = [];
  }

  async __call__(
    process: Process,
    job_order_object: CWLObjectType,
    runtime_context: RuntimeContext,
    logger: Logger = _logger,
  ): Promise<JobBase> {
    return this.execute(process, job_order_object, runtime_context, logger);
  }

  output_callback(out: CWLObjectType | undefined, process_status: JobStatus): void {
    this.final_status.push(process_status);
    this.final_output.push(out);
  }

  abstract run_jobs(
    _process: Process,
    _job_order_object: CWLObjectType,
    _logger: Logger,
    _runtime_context: RuntimeContext,
  ): Promise<JobBase>;

  async execute(
    process: Process,
    job_order_object: CWLObjectType,
    runtime_context: RuntimeContext,
    logger: Logger = _logger,
  ): Promise<JobBase> {
    this.final_output = [];
    this.final_status = [];

    if (!runtime_context.basedir) {
      throw new WorkflowException("Must provide 'basedir' in runtimeContext");
    }

    let finaloutdir: string | null = null;
    const original_outdir = runtime_context.relocateOutputs ? runtime_context.clientWorkDir : null;
    if (typeof original_outdir === 'string') {
      finaloutdir = path.resolve(original_outdir);
    }
    // runtime_context = runtime_context.copy();
    const outdir = runtime_context.createOutdir();
    this.output_dirs.push(outdir);
    runtime_context.outdir = outdir;
    //    runtime_context.mutation_manager = new MutationManager();
    runtime_context.toplevel = true;

    let job_reqs: CWLObjectType[] | null = null;
    if ('cwl:requirements' in job_order_object) {
      job_reqs = job_order_object['cwl:requirements'] as CWLObjectType[];
    }
    if (job_reqs !== null) {
      for (const req of job_reqs) {
        const r = createRequirements(req);
        if (r) {
          process.requirements.push(r);
        }
      }
    }

    const job =await this.run_jobs(process, job_order_object, logger, runtime_context);

    // if (this.final_output && this.final_output[0] !== undefined && finaloutdir !== null) {
    //   this.final_output[0] = await relocateOutputs(
    //     this.final_output[0],
    //     finaloutdir,
    //     new Set(this.output_dirs),
    //     runtime_context.move_outputs,
    //     runtime_context.make_fs_access(''),
    //     getDefault(runtime_context.compute_checksum, false),
    //   );
    // }

    // if (runtime_context.rm_tmpdir) {
    //   let output_dirs: string[];
    //   if (!runtime_context.cachedir) {
    //     output_dirs = this.output_dirs;
    //   } else {
    //     output_dirs = this.output_dirs.filter((x) => !x.startsWith(runtime_context.cachedir));
    //   }
    //   await cleanIntermediate(output_dirs);
    // }
    return job
  }
}
export class SingleJobExecutor extends JobExecutor {
  override async run_jobs(
    process: Process,
    job_order_object: CWLObjectType,
    logger: Logger,
    runtime_context: RuntimeContext,
  ): Promise<JobBase> {
    const jobiter = await process.job(
      job_order_object,
      (out: CWLObjectType | undefined, process_status: JobStatus) => this.output_callback(out, process_status),
      runtime_context,
      null
    );
    jobiter.run(runtime_context)
    return jobiter;
  }
}
