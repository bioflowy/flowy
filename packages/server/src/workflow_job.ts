
/* eslint-disable no-prototype-builtins */
import * as cwlTsAuto from '@flowy/cwl-ts-auto';
import { cloneDeep } from 'lodash-es';
import { contentLimitRespectedReadBytes } from './builder.js';
import { canAssignSrcToSinkType } from './checker.js';
import { RuntimeContext, getDefault, make_default_fs_access } from './context.js';
import { CommandOutputParameter, IOType, isChainableOutput, IWorkflowStep, WorkflowStepInput } from './cwltypes.js';
import { WorkflowException } from './errors.js';
import { do_eval } from './expression.js';
import { _logger } from './loghandler.js';
import { shortname, uniquename } from './process.js';
import { v4 as uuidv4 } from 'uuid';
import {
  type CWLObjectType,
  type CWLOutputType,
  type JobsGeneratorType,
  type OutputCallbackType,
  WorkflowStateItem,
  adjustDirObjs,
  aslist,
  get_listing,
  getRequirement,
  type ScatterOutputCallbackType,
  type ScatterDestinationsType,
  isMissingOrNull,
  str,
} from './utils.js';
import { Workflow, WorkflowStep } from './workflow.js';
import { JobGroup } from './jobgroup.js';
import { CommandLineJob, JobBase } from './job.js';
import { getJobWatcher } from './server/job_watcher.js';
import { FlowyJobURL, FlowyToolURL } from './flowyurl.js';

class WorkflowJobStep {
  step: WorkflowStep;
  tool: IWorkflowStep;
  id: string;
  submitted: boolean;
  iterable?: JobsGeneratorType;
  completed: boolean;
  name: string;

  constructor(step: WorkflowStep) {
    this.step = step;
    this.tool = step.tool;
    this.id = step.id;
    this.submitted = false;
    this.completed = false;
    this.name = uniquename(`step ${shortname(this.id)}`);
  }

  job(joborder: CWLObjectType, output_callback: OutputCallbackType, runtimeContext: RuntimeContext,workflow_id:FlowyJobURL | null): Promise<JobBase> {
    const context = runtimeContext.copy();
    context.part_of = this.name;
    context.name = shortname(this.id);

    _logger.info(`[${this.name}] start`);
    const jobs = this.step.job(joborder, output_callback, context,workflow_id);
    return jobs;
  }
}
class ReceiveScatterOutput {
  dest: ScatterDestinationsType;
  completed: number;
  processStatus: string;
  total: number;
  output_callback: ScatterOutputCallbackType;
  steps: (JobsGeneratorType | null)[];

  constructor(output_callback: ScatterOutputCallbackType, dest: ScatterDestinationsType, total: number) {
    this.dest = dest;
    this.completed = 0;
    this.processStatus = 'success';
    this.total = total;
    this.output_callback = output_callback;
    this.steps = [];
  }

  receive_scatter_output = (index: number, jobout: CWLObjectType, processStatus: string): void => {
    for (const key in jobout) {
      if (jobout.hasOwnProperty(key)) {
        this.dest[key][index] = jobout[key];
      }
    }

    // Release the iterable related to this step to reclaim memory
    if (this.steps.length > 0) {
      this.steps[index] = null;
    }

    if (processStatus !== 'success') {
      if (this.processStatus !== 'permanentFail') {
        this.processStatus = processStatus;
      }
    }

    this.completed += 1;

    if (this.completed === this.total) {
      this.output_callback(this.dest, this.processStatus);
    }
  };

  setTotal(total: number, steps: (JobsGeneratorType | null)[]): void {
    this.total = total;
    this.steps = steps;
    if (this.completed === this.total) {
      this.output_callback(this.dest, this.processStatus);
    }
  }
}

async function* nested_crossproduct_scatter(
  process: WorkflowJobStep,
  joborder: CWLObjectType,
  scatter_keys: string[],
  output_callback: ScatterOutputCallbackType,
  runtimeContext: RuntimeContext,
  parent_id: FlowyJobURL | null
): JobsGeneratorType {
  const scatter_key = scatter_keys[0];
  const jobl = (joborder[scatter_key] as unknown[]).length;
  const output: ScatterDestinationsType = {};
  for (const i of process.tool['outputs']) {
    output[i['id']] = new Array(jobl).fill(null);
  }

  const rc = new ReceiveScatterOutput(output_callback, output, jobl);
  const steps: (JobsGeneratorType | null)[] = [];
  for (let index = 0; index < jobl; index++) {
    let sjob: CWLObjectType = { ...joborder };
    sjob[scatter_key] = (joborder[scatter_key] as any)[index];

    if (scatter_keys.length === 1) {
      if (runtimeContext.postScatterEval !== undefined) {
        sjob = await runtimeContext.postScatterEval(sjob);
      }
      const curriedcallback = rc.receive_scatter_output.bind(rc, index);
      if (sjob !== null) {
        steps.push(generate(process.job(sjob, curriedcallback, runtimeContext,parent_id)));
      } else {
        curriedcallback({}, 'skipped');
        steps.push(null);
      }
    } else {
      steps.push(
        nested_crossproduct_scatter(
          process,
          sjob,
          scatter_keys.slice(1),
          rc.receive_scatter_output.bind(rc, index),
          runtimeContext,
          parent_id
        ),
      );
    }
  }

  rc.setTotal(jobl, steps);
  yield* parallel_steps(steps, rc, runtimeContext);
}
function crossproduct_size(joborder: CWLObjectType, scatter_keys: string[]): number {
  const scatter_key = scatter_keys[0];
  let ssum;

  if (scatter_keys.length == 1) {
    ssum = (joborder[scatter_key] as unknown[]).length;
  } else {
    ssum = 0;
    for (let _ = 0; _ < (joborder[scatter_key] as unknown[]).length; _++) {
      ssum += crossproduct_size(joborder, scatter_keys.slice(1));
    }
  }
  return ssum;
}
async function *generate(jobPromise:Promise<JobBase>):JobsGeneratorType{
  const job = await jobPromise
  yield job
}
async function flat_crossproduct_scatter(
  process: WorkflowJobStep,
  joborder: CWLObjectType,
  scatter_keys: string[],
  output_callback: ScatterOutputCallbackType,
  runtimeContext: RuntimeContext,
  parent_id: FlowyJobURL | null
): Promise<JobsGeneratorType> {
  const output: ScatterDestinationsType = {};

  for (const i of process.tool.outputs) {
    output[i.id] = Array(crossproduct_size(joborder, scatter_keys)).fill(null);
  }
  const callback = new ReceiveScatterOutput(output_callback, output, 0);
  const [steps, total] = await _flat_crossproduct_scatter(process, joborder, scatter_keys, callback, 0, runtimeContext,parent_id);
  callback.setTotal(total, steps);
  return parallel_steps(steps, callback, runtimeContext);
}

async function _flat_crossproduct_scatter(
  process: WorkflowJobStep,
  joborder: CWLObjectType,
  scatter_keys: string[],
  callback: ReceiveScatterOutput,
  startindex: number,
  runtimeContext: RuntimeContext,
  parent_id: FlowyJobURL | null
): Promise<[(JobsGeneratorType | undefined)[], number]> {
  const scatter_key = scatter_keys[0];
  const jobl = (joborder[scatter_key] as unknown[]).length;
  let steps: (JobsGeneratorType | undefined)[] = [];
  let put = startindex;

  for (let index = 0; index < jobl; index++) {
    let sjob: CWLObjectType = cloneDeep(joborder);
    sjob[scatter_key] = joborder[scatter_key][index];

    if (scatter_keys.length == 1) {
      if (runtimeContext.postScatterEval !== null) {
        sjob = await runtimeContext.postScatterEval(sjob);
      }
      const curriedcallback = callback.receive_scatter_output.bind(null, put);
      if (sjob !== null) {
        steps.push(generate(process.job(sjob, curriedcallback, runtimeContext,parent_id)));
      } else {
        curriedcallback({}, 'skipped');
        steps.push(null);
      }
      put += 1;
    } else {
      const [add, _] = await _flat_crossproduct_scatter(
        process,
        sjob,
        scatter_keys.slice(1),
        callback,
        put,
        runtimeContext,
        parent_id
      );
      put += add.length;
      steps = steps.concat(add);
    }
  }

  return [steps, put];
}
async function* parallel_steps(
  steps: (JobsGeneratorType | null)[],
  rc: ReceiveScatterOutput,
  runtimeContext: RuntimeContext,
): JobsGeneratorType {
  while (rc.completed < rc.total) {
    let made_progress = false;
    for (let index = 0; index < steps.length; index++) {
      const step = steps[index];
      if (
        getDefault(runtimeContext.on_error, 'stop') === 'stop' &&
        !['success', 'skipped'].includes(rc.processStatus)
      ) {
        break;
      }
      if (step === null) {
        continue;
      }
      try {
        for await (const j of step) {
          if (
            getDefault(runtimeContext.on_error, 'stop') === 'stop' &&
            !['success', 'skipped'].includes(rc.processStatus)
          ) {
            break;
          }
          if (j !== null) {
            made_progress = true;
            yield j;
          } else {
            break;
          }
        }
        if (made_progress) {
          break;
        }
      } catch (exc) {
        if (exc instanceof WorkflowException) {
          _logger.error(`Cannot make scatter job: ${exc.message}`);
          _logger.debug('', { exc_info: true }); // Assuming exc_info is a placeholder for additional logging details
          rc.receive_scatter_output(index, {}, 'permanentFail');
        }
      }
    }
    if (!made_progress && rc.completed < rc.total) {
      yield null;
    }
  }
}
function getArrayLength(a: unknown): number {
  if (Array.isArray(a)) {
    return a.length;
  }
  throw new Error(`Expected An Array but ${typeof a} founded.`);
}
async function dotproduct_scatter(
  process: WorkflowJobStep,
  joborder: CWLObjectType,
  scatter_keys: string[],
  output_callback: ScatterOutputCallbackType,
  runtimeContext: RuntimeContext,
  parent_id: FlowyJobURL | null
): Promise<JobsGeneratorType> {
  let jobl: number | null = null;
  for (const key of scatter_keys) {
    if (jobl === null) {
      jobl = getArrayLength(joborder[key]); // Assuming joborder[key] is an array
    } else if (jobl !== getArrayLength(joborder[key])) {
      throw new Error('Length of input arrays must be equal when performing dotproduct scatter.');
    }
  }

  if (jobl === null) {
    throw new Error('Impossible codepath');
  }

  const output: ScatterDestinationsType = {};
  for (const i of process.tool['outputs']) {
    output[i['id']] = Array(jobl).fill(null);
  }

  const rc = new ReceiveScatterOutput(output_callback, output, jobl); // Assuming ReceiveScatterOutput is a class you've defined

  const steps: (JobsGeneratorType | null)[] = [];
  for (let index = 0; index < jobl; index++) {
    let sjobo: CWLObjectType | null = { ...joborder }; // Shallow copy of the object
    for (const key of scatter_keys) {
      sjobo[key] = (joborder[key] as CWLOutputType[])[index];
    }

    if (runtimeContext.postScatterEval) {
      sjobo = await runtimeContext.postScatterEval(sjobo);
    }
    const curriedcallback = (jobout: CWLObjectType, processStatus: string) =>
      rc.receive_scatter_output(index, jobout, processStatus); // Binding index as the first argument
    if (sjobo) {
      steps.push(generate(process.job(sjobo, curriedcallback, runtimeContext,parent_id)));
    } else {
      curriedcallback({}, 'skipped');
      steps.push(null);
    }
  }

  rc.setTotal(jobl, steps);
  return parallel_steps(steps, rc, runtimeContext); // Assuming parallel_steps is a function you've defined
}
function matchTypes(
  input:WorkflowStepInput,
  sinktype: IOType,
  src: WorkflowStateItem,
  iid: string,
  inputobj: CWLObjectType,
  linkMerge: string | null,
  valueFrom: string | null,
): boolean {
  if (sinktype instanceof Array) {
    for (const st of sinktype) {
      if (matchTypes(input,st, src, iid, inputobj, linkMerge, valueFrom)) {
        return true;
      }
    }
  } else if (src.parameter.type instanceof Array) {
    const original_types = src.parameter.type;
    for (const source_type of original_types) {
      src.parameter.type = source_type;
      const match = matchTypes(input,sinktype, src, iid, inputobj, linkMerge, valueFrom);
      if (match) {
        src.parameter.type = original_types;
        return true;
      }
    }
    src.parameter['type'] = original_types;
    return false;
  } else if (linkMerge) {
    if (!(iid in inputobj)) {
      inputobj[iid] = [];
    }
    const sourceTypes: CWLOutputType[] | undefined = inputobj[iid] as CWLOutputType[] | undefined;
    if (linkMerge === 'merge_nested') {
      sourceTypes.push(src.value);
    } else if (linkMerge === 'merge_flattened') {
      if (src.value instanceof Array) {
        sourceTypes.push(...src.value);
      } else {
        sourceTypes.push(src.value);
      }
    } else {
      throw new Error(`Unrecognized linkMerge enum '${linkMerge}'`);
    }
    return true;
  } else if (valueFrom !== null || canAssignSrcToSinkType(src.parameter.type, sinktype) || sinktype === 'Any') {
    inputobj[iid] = cloneDeep(src.value);
    if(sinktype === "File" && input.streamable){
      const obj = inputobj[iid]
      obj["streamable"] = input.streamable
    }
    return true;
  }
  return false;
}
function objectFromState(
  state: { [key: string]: WorkflowStateItem | null | undefined },
  params: WorkflowStepInput[],
  fragOnly: boolean,
  supportsMultipleInput: boolean,
  sourceField: string,
  incomplete = false,
): CWLObjectType | null {
  const inputobj: CWLObjectType = {};
  for (const inp of params) {
    const originalId = inp['id'];
    let iid = originalId;
    if (fragOnly) {
      iid = shortname(iid);
    }
    if (inp[sourceField]) {
      const connections = aslist(inp[sourceField]);
      if (connections.length > 1 && !supportsMultipleInput) {
        throw new WorkflowException(
          'Workflow contains multiple inbound links to a single ' +
            'parameter but MultipleInputFeatureRequirement is not ' +
            'declared.',
        );
      }
      for (const src of connections) {
        const aState = state[src] || null;
        if (aState && (aState.success === 'success'|| aState.success === 'streamable' && inp.streamable || aState.success === 'skipped' || incomplete)) {
          if (
            !matchTypes(
              inp,
              inp.type,
              aState,
              iid,
              inputobj,
              (inp['linkMerge'] || (connections.length > 1 ? 'merge_nested' : null)) as string | null,
              inp.valueFrom,
            )
          ) {
            throw new WorkflowException(
              `Type mismatch between source '${src}' (${str(aState.parameter['type'])}) and ` +
                `sink '${originalId}' (${str(inp['type'])})`,
            );
          }
        } else if (!(src in state)) {
          throw new WorkflowException(`Connect source '${src}' on parameter '${originalId}' does not exist`);
        } else if (!incomplete) {
          return null;
        }
      }
    }
    if (inp.pickValue && Array.isArray(inputobj[iid])) {
      const seq = inputobj[iid] as (CWLOutputType | null)[];
      if (inp.pickValue === cwlTsAuto.PickValueMethod.FIRST_NON_NULL) {
        let found = false;
        for (const v of seq) {
          if (v !== null) {
            found = true;
            inputobj[iid] = v;
            break;
          }
        }
        if (!found) {
          throw new WorkflowException(`All sources for '${shortname(originalId)}' are null`);
        }
      } else if (inp.pickValue === cwlTsAuto.PickValueMethod.THE_ONLY_NON_NULL) {
        let found = false;
        for (const v of seq) {
          if (v !== null) {
            if (found) {
              throw new WorkflowException(
                `Expected only one source for '${shortname(originalId)}' to be non-null, got ${str(seq)}`,
              );
            }
            found = true;
            inputobj[iid] = v;
          }
        }
        if (!found) {
          throw new WorkflowException(`All sources for '${shortname(originalId)}' are null`);
        }
      } else if (inp.pickValue === cwlTsAuto.PickValueMethod.ALL_NON_NULL) {
        inputobj[iid] = seq.filter((v) => v !== null);
      }
    }
    if (isMissingOrNull(inputobj, iid) && inp.default_ !== undefined) {
      inputobj[iid] = inp.default_;
    }
    if (!(iid in inputobj) && ('valueFrom' in inp || incomplete)) {
      inputobj[iid] = null;
    }
    if (!(iid in inputobj)) {
      throw new WorkflowException(`Value for ${originalId} not specified`);
    }
  }
  return inputobj;
}
export class WorkflowJob extends JobBase{
  workflow: Workflow;
  //   prov_obj: ProvenanceProfile | null;
  //   parent_wf: ProvenanceProfile | null;
  tool: cwlTsAuto.Workflow;
  steps: WorkflowJobStep[];
  state: { [key: string]: WorkflowStateItem | null };
  did_callback: boolean;
  made_progress: boolean | null;
  outdir: string;
  output_callback: OutputCallbackType;
  runtimeContext: RuntimeContext;
  _get_type() {
    return "Workflow"
  }
  constructor(workflow: Workflow, runtimeContext: RuntimeContext,parent_id:FlowyJobURL | null,output_callback: OutputCallbackType,job: CWLObjectType) {
    super(new FlowyJobURL(uuidv4()),uniquename(
      `workflow ${getDefault(runtimeContext.name, shortname(workflow.tool.id || 'embedded'))}`,
    ),"Workflow")
    this.parent_id = parent_id;
    this.workflow = workflow;
    this.joborder = job;
    this.output_callback = output_callback
    // this.prov_obj = null;
    // this.parent_wf = null;
    this.tool = workflow.tool;
    // if (runtimeContext.research_obj !== null) {
    //   this.prov_obj = workflow.provenance_object;
    //   this.parent_wf = workflow.parent_wf;
    // }
    this.steps = workflow.steps.map((s) => new WorkflowJobStep(s));
    this.state = {};
    this.processStatus = 'created';
    this.did_callback = false;
    this.made_progress = null;
    this.outdir = runtimeContext.getOutdir();

    _logger.debug(
      '[%s] initialized from %s',
      this.name,
      this.tool.id || `workflow embedded in ${runtimeContext.part_of}`,
    );
  }
  getOutdirs(){
    return this.outdir?[this.outdir]:[]
  }
  do_output_callback(final_output_callback) {
    const supportsMultipleInput = Boolean(this.workflow.getRequirement(cwlTsAuto.MultipleInputFeatureRequirement)[0]);

    let wo: CWLOutputType | null = null;
    try {
      wo = objectFromState(this.state, this.tool.outputs, true, supportsMultipleInput, 'outputSource', true);
      this.results = wo
    } catch (err) {
      if (err instanceof Error) {
        _logger.error(`[${this.name}] Cannot collect workflow output: ${err.message} ${err.stack}`);
      }
      this.processStatus = 'permanentFail';
    }
    // if (this.prov_obj && this.parent_wf && this.prov_obj.workflow_run_uri !== this.parent_wf.workflow_run_uri) {
    //   const process_run_id: string | null = null;
    //   this.prov_obj.generate_output_prov(wo || {}, process_run_id, this.name);
    //   this.prov_obj.document.wasEndedBy(this.prov_obj.workflow_run_uri, null, this.prov_obj.engine_uuid, new Date());
    //   const prov_ids: any = this.prov_obj.finalize_prov_profile(this.name);
    //   this.parent_wf.activity_has_provenance(this.prov_obj.workflow_run_uri, prov_ids);
    // }

    _logger.info(`[${this.name}] completed ${this.processStatus}'`);
    if (_logger.isDebugEnabled()) {
      _logger.debug(`[${this.name}] outputs ${JSON.stringify(wo, null, 4)}`);
    }

    this.did_callback = true;
    if(["created" , "queued" , "started"].includes(this.processStatus) ){
      this.processStatus = "success"
    }
    getJobWatcher().jobFinished(this,0,wo)

    final_output_callback(wo, this.processStatus);
  }
  receive_output(
    step: WorkflowJobStep,
    outputparms: CommandOutputParameter[],
    final_output_callback: OutputCallbackType,
    jobout: CWLObjectType,
    processStatus: string,
  ) {
    for (const i of outputparms) {
      if ('id' in i) {
        const iid = i['id'];
        if (iid in jobout) {
          this.state[iid] = new WorkflowStateItem(i, jobout[iid], processStatus);
        } else {
          _logger.error(`[${step.name}] Output is missing expected field ${iid}`);
          processStatus = 'permanentFail';
        }
      }
    }

    if (_logger.isDebugEnabled()) {
      _logger.debug(`[${step.name}] produced output ${JSON.stringify(jobout, null, 4)}`);
    }

    if (!['success', 'skipped'].includes(processStatus)) {
      if (this.processStatus != 'permanentFail') {
        this.processStatus = processStatus as any;
      }
      _logger.warn(`[${step.name}] completed ${processStatus}`);
    } else {
      _logger.info(`[${step.name}] completed ${processStatus}`);
    }

    step.completed = true;
    step.iterable = null;
    this.made_progress = true;

    const completed = this.steps.filter((s) => s.completed).length;
    if (completed == this.steps.length) {
      this.do_output_callback(final_output_callback);
    }else{
      void this.rerun()
    }
  }
  async tryMakeJob(
    step: WorkflowJobStep,
    finalOutputCallback: OutputCallbackType | undefined,
    runtimeContext: RuntimeContext,
    workflow_id: FlowyJobURL | null
  ): Promise<JobsGeneratorType> {
    let containerEngine = 'docker';
    if (runtimeContext.podman) {
      containerEngine = 'podman';
    } else if (runtimeContext.singularity) {
      containerEngine = 'singularity';
    }
    if (step.submitted) {
      return undefined;
    }

    const inputParms = step.step.tool.inputs;
    const outputParms = step.step.tool.outputs;

    const supportsMultipleInput = Boolean(this.workflow.getRequirement(cwlTsAuto.MultipleInputFeatureRequirement)[0]);

    try {
      const inputObj = objectFromState(this.state, inputParms, false, supportsMultipleInput, 'source');
      if (!inputObj) {
        _logger.debug(`[${this.name}] job step ${step.id} not ready`);
        return undefined;
      }

      _logger.info(`[${this.name}] starting ${step.name}`);
      // eslint-disable-next-line no-inner-declarations
      const callback = (jobout: CWLObjectType, processStatus: string) => {
        return this.receive_output(step, outputParms, finalOutputCallback, jobout, processStatus);
      };

      const valueFrom: {
        [key: string]: any;
      } = {};
      step.step.tool.inputs.forEach((i: any) => {
        if (i.valueFrom) valueFrom[i.id] = i.valueFrom;
      });

      const loadContents = new Set(step.step.tool.inputs.map((i) => i.loadContents && i.id));

      if (
        Object.keys(valueFrom).length > 0 &&
        !this.workflow.getRequirement(cwlTsAuto.StepInputExpressionRequirement)[0]
      ) {
        throw new WorkflowException(
          'Workflow step contains valueFrom but StepInputExpressionRequirement not in requirements',
        );
      }

      const postScatterEval = async (io: CWLObjectType): Promise<CWLObjectType | undefined> => {
        const shortio: CWLObjectType = {};
        for (const k in io) {
          shortio[shortname(k)] = io[k];
        }

        const fs_access = getDefault(runtimeContext.make_fs_access, make_default_fs_access)('');
        for (const k in io) {
          if (loadContents.has(k)) {
            const val: CWLObjectType = io[k] as CWLObjectType;
            if (!val['contents']) {
              // Assuming fs_access.open() returns something that can be used within a TypeScript async context
              val['contents'] = await contentLimitRespectedReadBytes(val['location'] as string);
            }
          }
        }

        const valueFromFunc = async (k: string, v: CWLOutputType | null): Promise<CWLOutputType | null> => {
          if (k in valueFrom) {
            const promises = [];
            adjustDirObjs(v, (val) => {
              promises.push(get_listing(fs_access, val, true));
            });
            await Promise.all(promises);
            const [inline] = getRequirement(this.workflow, cwlTsAuto.InlineJavascriptRequirement);
            return do_eval(valueFrom[k], shortio, inline, null, null, {}, v);
          }
          return v;
        };

        const psio: { [key: string]: CWLOutputType | null } = {};
        for (const k in io) {
          psio[k] = await valueFromFunc(k, io[k]);
        }

        if (step.tool['when']) {
          const evalinputs: { [key: string]: CWLOutputType | null } = {};
          for (const k in psio) {
            evalinputs[shortname(k)] = psio[k];
          }
          const [InlineJavascriptRequirement] = getRequirement(this.workflow, cwlTsAuto.InlineJavascriptRequirement);
          const whenval = await do_eval(
            step.tool['when'],
            evalinputs,
            InlineJavascriptRequirement,
            null,
            null,
            {},
            runtimeContext.debug,
            runtimeContext.js_console,
          );

          if (whenval === true) {
            // Do nothing, analogous to Python's pass
          } else if (whenval === false) {
            _logger.debug(`[${step.name}] conditional ${step.tool['when']} evaluated to ${whenval}`);
            _logger.debug(`[${step.name}] inputs was ${JSON.stringify(evalinputs, null, 2)}`);
            return null;
          } else {
            throw new Error("Conditional 'when' must evaluate to 'true' or 'false'");
          }
        }
        return psio;
      };
      if (_logger.isDebugEnabled()) {
        _logger.debug('[%s] job input %s', step.name, JSON.stringify(inputObj, null, 4));
      }
      let jobs: JobsGeneratorType;
      if (step.tool.scatter) {
        const scatter: string[] = aslist(step.tool.scatter);
        const method = step.tool.scatterMethod;
        if (method === undefined && scatter.length != 1) {
          throw new WorkflowException('Must specify scatterMethod when scattering over multiple inputs');
        }
        runtimeContext = runtimeContext.copy();
        runtimeContext.postScatterEval = postScatterEval;

        const emptyscatter = scatter.filter((value) => {
          const inputobj = inputObj[value];
          return inputobj && inputobj instanceof Array && inputobj.length === 0;
        });
        if (emptyscatter) {
          _logger.warn(
            `[job ${step.name}] Notice: scattering over empty input in '${emptyscatter.join(
              "', '",
            )}'.  All outputs will be empty.`,
          );
        }
        if (!method || method === cwlTsAuto.ScatterMethod.DOTPRODUCT) {
          jobs = await dotproduct_scatter(step, inputObj, scatter, callback, runtimeContext,workflow_id);
        } else if (method == cwlTsAuto.ScatterMethod.NESTED_CROSSPRODUCT) {
          jobs = nested_crossproduct_scatter(step, inputObj, scatter, callback, runtimeContext,workflow_id);
        } else if (method == cwlTsAuto.ScatterMethod.FLAT_CROSSPRODUCT) {
          jobs = await flat_crossproduct_scatter(step, inputObj, scatter, callback, runtimeContext,workflow_id);
        }
      } else {
        const inputobj = await postScatterEval(inputObj);
        if (inputobj) {
          if (_logger.isDebugEnabled()) {
            _logger.debug('[%s] evaluated job input to %s', step.name, JSON.stringify(inputobj, null, 4));
          }
          // if step.step.get_requirement("http://commonwl.org/cwltool#Loop")[0]:
          //     jobs = WorkflowJobLoopStep(
          //         step=step, container_engine=container_engine
          //     ).job(inputobj, callback, runtimeContext)
          // else:
          jobs = generate(step.job(inputobj, callback, runtimeContext,workflow_id));
        } else {
          _logger.info('[%s] will be skipped', step.name);
          const result = {};
          outputParms.forEach((e) => {
            result[e.id] = null;
          });
          callback(result, 'skipped');
          step.completed = true;
          return undefined;
        }
      }
      step.submitted = true;
      return jobs;
      // else:
      //     _logger.info("[%s] will be skipped", step.name)
      //     callback({k["id"]: None for k in outputparms}, "skipped")
      //     step.completed = True
      //     jobs = (_ for _ in ())
    } catch (err) {
      if (err instanceof WorkflowException) throw err;
      _logger.error('Unhandled exception', err);

      this.processStatus = 'permanentFail';
      step.completed = true;
    }
    throw new Error('TODO');
  }
  async run(_runtimeContext: RuntimeContext): Promise<void> {
    this.runtimeContext = _runtimeContext;
    this.processStatus = 'started';

    _logger.info(`[${this.name}] start`);
    this.tool.inputs.forEach((inp: cwlTsAuto.WorkflowInputParameter) => {
      const inp_id = shortname(inp.id);
      if (inp_id in this.joborder) {
        this.state[inp.id] = new WorkflowStateItem(inp, this.joborder[inp_id], 'success');
      } else if (inp.default_) {
        this.state[inp.id] = new WorkflowStateItem(inp, inp.default_, 'success');
      } else {
        throw new WorkflowException(`Input '${inp.id}' not in input object and does not have a default value.`);
      }
    });
    this.steps.forEach((step) => {
      step.step.tool.outputs.forEach((out) => {
        this.state[out['id']] = null;
      });
    });
    const runtimeContext = _runtimeContext.copy();
    runtimeContext.toplevel = false;

    const jobs = await this.createExectableJobs(this.joborder,this.output_callback,runtimeContext,this.id)
    if(jobs){
      for(const job of jobs){
        await job.run(runtimeContext)
      }  
    }
    const completed = this.steps.filter((s) => s.completed).length;
    if(completed>=this.steps.length){
      if (!this.did_callback && this.output_callback) {
        this.do_output_callback(this.output_callback);
      }
    }
  }
  async rerun(){
    const jobs = await this.createExectableJobs(this.joborder,this.output_callback,this.runtimeContext,this.id)
    for(const job of jobs){
      await job.run(this.runtimeContext)
    }
  }
  /**
   * Generate and return executable Jobs for each step
   * @param joborder 
   * @param output_callback 
   * @param runtimeContext 
   * @param workflow_id 
   * @returns 
   */
  async createExectableJobs(
    joborder: CWLObjectType,
    output_callback: OutputCallbackType,
    runtimeContext: RuntimeContext,
    workflow_id: FlowyJobURL | null
  ): Promise<(JobBase | JobGroup)[]> {
    const executableJobs :(JobBase | JobGroup)[] = []
    if (_logger.isDebugEnabled()) {
      _logger.debug(`[${this.name}] inputs ${JSON.stringify(joborder, null, 4)}`);
    }

    runtimeContext = runtimeContext.copy();
    runtimeContext.outdir = undefined;

    let completed = 0;
    let submittableJobs:CommandLineJob[]= []
    while (completed < this.steps.length) {
      this.made_progress = false;
      for (const step of this.steps) {
        if (getDefault(runtimeContext.on_error, 'stop') === 'stop' && this.processStatus !== 'started') break;
        if (!step.submitted) {
          try {
            step.iterable = await this.tryMakeJob(step, output_callback, runtimeContext,workflow_id);
          } catch (exc: unknown) {
            if (exc instanceof Error) {
              _logger.error(`[${step.name}] Cannot make job: ${exc.message} ${exc.stack}`);
            }
            this.processStatus = 'permanentFail';
          }
        }
        if (step.iterable) {
          try {
          for await (const newjob of step.iterable) {
            if (getDefault(runtimeContext.on_error, 'stop') === 'stop' && this.processStatus !== 'started') break;
            if (newjob) {
              this.made_progress = true;
              if(newjob instanceof CommandLineJob){
                for (const i of newjob.tool.outputs) {
                  if ('id' in i) {
                    const outputFileUrl = isChainableOutput(newjob,i)
                    if(outputFileUrl){
                      const iid = i['id'];
                      this.state[iid] = new WorkflowStateItem(i, {class:"File",location:outputFileUrl}, "streamable");
                    }
                  }
                }
                // もしCommandLineJobならStateを"streamable"にして、さらに実行可能なCommandLineJobがないか調べる
                submittableJobs.push(newjob)
              }else{
                if(submittableJobs.length>0){
                  // もしCommandLineJob以外のJobがきて、submittableJobsがあれば、submitしておく
                  const prevJob = new JobGroup(submittableJobs)
                  submittableJobs = []
                  executableJobs.push(prevJob)
                }
                executableJobs.push(newjob)
              }
            } else {
              break;
            }
          }
          } catch (exc) {
            if (exc instanceof Error) {
              _logger.error(`[${step.name}] Cannot make job: ${exc.message} ${exc.stack}`);
            }
            this.processStatus = 'permanentFail';
          }
        }
      }
      if(submittableJobs.length>0){
        // もしsubmittableJobsがあれば、submitしておく
        const prevJob = new JobGroup(submittableJobs)
        submittableJobs = []
        executableJobs.push(prevJob)
        this.made_progress = true
      }
      return executableJobs;
    //   completed = this.steps.filter((s) => s.completed).length;
    //   if (!this.made_progress){ 
    //     if(completed < this.steps.length) {
    //     if (this.processStatus !== 'success'){
    //       break;
    //     }else{
    //        yield undefined;
    //     }
    //   }
    // }
    // }
    // if (!this.did_callback && output_callback) {
    //   this.do_output_callback(output_callback);
    // }
  }
}
}
