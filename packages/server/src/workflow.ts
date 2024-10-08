import * as crypto from 'node:crypto';
import * as cwlTsAuto from '@flowy/cwl-ts-auto';
import { circular_dependency_checker, static_checker } from './checker.js';
import * as command_line_tool from './command_line_tool.js';
import { LoadingContext, RuntimeContext, getDefault } from './context.js';
import { deepcopy } from './copy.js';
import {
  ArrayType,
  CommandInputParameter,
  CommandOutputParameter,
  IWorkflowStep,
  WorkflowStepInput,
  WorkflowStepOutput,
} from './cwltypes.js';
import { ValidationException, WorkflowException } from './errors.js';
import { loadDocument } from './loader.js';
import { _logger } from './loghandler.js';
import { Process, shortname } from './process.js';
import { transferProperties } from './types.js';

import {
  isString,
  type CWLObjectType,
  type JobsGeneratorType,
  type OutputCallbackType,
  aslist,
  CWLOutputType,
  JobStatus,
} from './utils.js';
import { WorkflowJob } from './workflow_job.js';
import { getJobWatcher } from './server/job_watcher.js';
import { JobBase } from './job.js';
import { Job } from './databases.js';
import { FlowyJobURL, FlowyToolURL } from './flowyurl.js';

function sha1(data: string): string {
  const hash = crypto.createHash('sha1');
  hash.update(data);
  return hash.digest('hex');
}

function _convert_stdstreams_to_files(tool: cwlTsAuto.CommandLineTool) {
  for (const out of tool.outputs) {
    for (const streamtype of ['stdout', 'stderr']) {
      if (out.type === streamtype) {
        if (out.outputBinding) {
          throw new ValidationException(`Not allowed to specify outputBinding when using ${streamtype} shortcut.`);
        }
        let filename = undefined;
        if (streamtype === 'stdout') {
          filename = tool.stdout;
        } else if (streamtype === 'stderr') {
          filename = tool.stderr;
        }
        if (!filename) {
          filename = sha1(JSON.stringify(tool));
        }
        if (streamtype === 'stdout') {
          tool.stdout = filename;
        } else if (streamtype === 'stderr') {
          tool.stderr = filename;
        }
        out.type = 'File';
        out.outputBinding = new cwlTsAuto.CommandOutputBinding({ glob: filename });
      }
    }
  }
  for (const inp of tool.inputs) {
    if (inp.type === 'stdin') {
      if (inp.inputBinding) {
        throw new ValidationException('Not allowed to specify inputBinding when using stdin shortcut.');
      }
      if (tool.stdin) {
        throw new ValidationException('Not allowed to specify stdin path when using stdin type shortcut.');
      } else {
        tool.stdin = `$(inputs.${inp.id.split('#').pop()?.split('/').pop()}.path)`;
        inp.type = 'File';
      }
    }
  }
}
function validate(
  toolpath_object: cwlTsAuto.ExpressionTool | cwlTsAuto.CommandLineTool | cwlTsAuto.Workflow | cwlTsAuto.Operation,
  loadingContext: LoadingContext,
) {
  const schemas = toolpath_object.loadingOptions.schemas;
  if (schemas && Array.isArray(schemas)) {
    for (const schema of schemas) {
      const baseuri = loadingContext.baseuri.endsWith('/') ? loadingContext.baseuri : `${loadingContext.baseuri}/`;
      const schema_path = toolpath_object.loadingOptions.fetcher.urljoin(baseuri, schema);
      loadingContext.formatGraph.addOntology(schema_path);
    }
  }
}
// context.default_make_tool = default_make_tool;

export class WorkflowStep extends Process {
  declare tool: IWorkflowStep;
  id: string;
  embedded_tool: Process;
  pos: number;
  constructor(doc: cwlTsAuto.WorkflowStep, pos: number) {
    super(doc);
    this.pos = pos;
  }
  override async init(loadingContext: LoadingContext) {
    if (this.tool.id) {
      this.id = this.tool.id;
    } else {
      this.id = `#step${this.pos}`;
    }

    loadingContext = loadingContext.copy();

    const parent_requirements = getDefault(loadingContext.requirements, []);
    loadingContext.requirements = this.tool.requirements || [];
    if (loadingContext.requirements === null) throw new Error('');

    for (const parent_req of parent_requirements) {
      let found_in_step = false;
      for (const step_req of loadingContext.requirements) {
        if (parent_req.class_ === step_req.class_) {
          found_in_step = true;
          break;
        }
      }
      if (!found_in_step) {
        // && parent_req.get('class_') !== 'http://commonwl.org/cwltool#Loop') {
        loadingContext.requirements.push(parent_req);
      }
    }
    // loadingContext.requirements = loadingContext.requirements.concat(
    //   cast(
    //     List[CWLObjectType],
    //     get_overrides(getdefault(loadingContext.overrides_list, []), this.id).get('requirements', []),
    //   ),
    // );

    let hints = loadingContext.hints ? [...loadingContext.hints] : [];
    hints = hints.concat(this.tool.hints ?? []);
    loadingContext.hints = hints;

    if (isString(this.tool.run)) {
      loadingContext.metadata = {};
      const [tool, _] = await loadDocument(this.tool.run, loadingContext);
      this.embedded_tool = tool;
    } else {
      this.embedded_tool = await loadingContext.construct_tool_object(this.tool.run, loadingContext);
    }
    const validation_errors = [];
    const bound = new Set();

    if (this.embedded_tool.getRequirement(cwlTsAuto.SchemaDefRequirement)[0]) {
      if (!this.tool.requirements) {
        this.tool.requirements = [];
      }
      this.tool.requirements.push(this.embedded_tool.getRequirement(cwlTsAuto.SchemaDefRequirement)[0]);
    }
    this.tool.inputs = handleInput(this.tool, this.embedded_tool, this.tool.in_, bound, validation_errors);
    this.tool.outputs = handleOutput(this.embedded_tool, this.tool.out, bound, validation_errors);

    const missing_values = [];
    for (const tool_entry of this.embedded_tool.tool.inputs) {
      if (!bound.has(shortname(tool_entry.id))) {
        if (!aslist(tool_entry.type).includes('null') && !tool_entry.default_) {
          missing_values.push(shortname(tool_entry['id']));
        }
      }
    }

    if (missing_values.length > 0) {
      validation_errors.push(
        new WorkflowException(
          `Step is missing required parameter${missing_values.length > 1 ? 's' : ''} '${missing_values.join("', '")}'`,
        ),
      );
    }

    if (validation_errors.length > 0) throw new ValidationException(validation_errors.join('\n'));

    super.init(loadingContext);

    if (this.embedded_tool.tool instanceof cwlTsAuto.Workflow) {
      const [feature, _] = this.getRequirement(cwlTsAuto.SubworkflowFeatureRequirement);
      if (!feature) {
        throw new WorkflowException(
          'Workflow contains embedded workflow but SubworkflowFeatureRequirement not in requirements',
        );
      }
    }

    if (this.tool.scatter) {
      const [feature, _] = this.getRequirement(cwlTsAuto.ScatterFeatureRequirement);
      if (!feature) {
        throw new WorkflowException('Workflow contains scatter but ScatterFeatureRequirement not in requirements');
      }

      const inputparms = [...this.tool.inputs];
      const outputparms = [...this.tool.outputs];
      const scatter = aslist(this.tool.scatter);

      const method = this.tool.scatterMethod;
      if (method === null && scatter.length !== 1) {
        throw new ValidationException('Must specify scatterMethod when scattering over multiple inputs');
      }

      const inp_map = inputparms.reduce((acc, i) => ({ ...acc, [i['id']]: i }), {});
      for (const inp of scatter) {
        if (!(inp in inp_map)) {
          throw new ValidationException(
            `Scatter parameter '${shortname(
              inp,
            )}' does not correspond to an input parameter of this step, expecting '${Object.keys(inp_map)
              .map((k) => shortname(k))
              .join("', '")}'`,
          );
        }

        inp_map[inp].type = new cwlTsAuto.InputArraySchema({ type: ArrayType, items: inp_map[inp].type });
      }

      let nesting;
      if (this.tool.scatterMethod === cwlTsAuto.ScatterMethod.NESTED_CROSSPRODUCT) {
        nesting = scatter.length;
      } else {
        nesting = 1;
      }

      for (let i = 0; i < nesting; i++) {
        for (const oparam of outputparms) {
          oparam.type = { type: ArrayType, items: oparam.type };
        }
      }
      this.tool.inputs = inputparms;
      this.tool.outputs = outputparms;
    }
    // this.prov_obj = null;
    // if (loadingContext.research_obj !== null) {
    //     this.prov_obj = parentworkflowProv;
    //     if (this.embedded_tool.tool["class"] === "Workflow") {
    //         this.parent_wf = this.embedded_tool.parent_wf;
    //     } else {
    //         this.parent_wf = this.prov_obj;
    //     }
    // }
  }
  // override checkRequirements(_rec: unknown, _supported_process_requirements: Iterable<string>): void {
  // supported_process_requirements = [...supported_process_requirements];
  // supported_process_requirements.push('http://commonwl.org/cwltool#Loop');
  // super.checkRequirements(rec, supported_process_requirements);
  // }

  receive_output(output_callback: OutputCallbackType, jobout: CWLObjectType, processStatus: JobStatus): void {
    const output: { [key: string]: CWLOutputType } = {};
    for (const i of this.tool.outputs) {
      const field = shortname(i['id']);
      if (field in jobout) {
        output[i['id']] = jobout[field];
      } else {
        processStatus = 'permanentFail';
      }
    }
    output_callback(output, processStatus);
  }

  async job(
    job_order: CWLObjectType,
    output_callbacks: OutputCallbackType | null,
    runtimeContext: RuntimeContext,
    workflow_id: FlowyJobURL | null
  ): Promise<JobBase> {
    // if (
    //   this.embedded_tool.tool['class'] == 'Workflow' &&
    //   runtimeContext.research_obj &&
    //   this.prov_obj &&
    //   this.embedded_tool.provenance_object
    // ) {
    //   this.embedded_tool.parent_wf = this.prov_obj;
    //   const process_name = this.tool['id'].split('#')[1];
    //   this.prov_obj.start_process(process_name, new Date(), this.embedded_tool.provenance_object.workflow_run_uri);
    // }

    const step_input: { [key: string]: CWLOutputType } = {};
    for (const inp of this.tool.inputs) {
      const field = shortname(inp.id);
      if (!inp.not_connected) {
        step_input[field] = job_order[inp.id];
      }
    }

    try {
      const jobiter = this.embedded_tool.job(
        step_input,
        (output: CWLObjectType, processStatus: JobStatus) => this.receive_output(output_callbacks, output, processStatus),
        runtimeContext,
        workflow_id
      );
      return jobiter;
    } catch (e) {
      if (e instanceof WorkflowException) {
        _logger.error(`Exception on step '${runtimeContext.name}'`);
        throw e;
      } else {
        _logger.warn('Unexpected exception');
        throw e;
      }
    }
  }
}

export class Workflow extends Process {
  declare tool: cwlTsAuto.Workflow;
  steps: WorkflowStep[];

  override async init(loadingContext: LoadingContext) {
    for (const i of this.tool.inputs) {
      const c: CommandInputParameter = i;
      c.name = shortname(i.id);
    }
    for (const i of this.tool.outputs) {
      const c: CommandOutputParameter = i;
      c.name = shortname(i.id);
    }
    super.init(loadingContext);
    loadingContext = loadingContext.copy();
    loadingContext.requirements = this.requirements;
    loadingContext.hints = this.hints;

    this.steps = [];
    const validation_errors = [];

    for (const [index, step] of this.tool.steps.entries()) {
      try {
        const s = await this.make_workflow_step(step, index, loadingContext);
        this.steps.push(s); // , loadingContext.prov_obj));
      } catch (vexc) {
        if (_logger.isDebugEnabled()) {
          if (vexc instanceof Error) {
            console.error(vexc);
            _logger.warn(`Validation failed ${vexc.message} at ${vexc.stack}`);
          }
        }
        validation_errors.push(vexc);
      }
    }

    if (validation_errors.length) {
      throw new ValidationException(validation_errors.map((v) => `\n${v}`).join(''));
    }

    this.steps = this.steps.sort(() => Math.random() - 0.5);

    const workflow_inputs = this.tool.inputs;
    const workflow_outputs = this.tool.outputs;

    let step_inputs: WorkflowStepInput[] = [];
    let step_outputs: WorkflowStepOutput[] = [];
    const param_to_step: { [key: string]: IWorkflowStep } = {};

    for (const step of this.steps) {
      step_inputs = step_inputs.concat(step.tool.inputs);
      step_outputs = step_outputs.concat(step.tool.outputs);
      for (const s of step.tool.inputs) {
        param_to_step[s.id] = step.tool;
      }
      for (const s of step.tool.outputs) {
        param_to_step[s.name] = step.tool;
      }
    }

    if (loadingContext.do_validate ?? true) {
      static_checker(workflow_inputs, workflow_outputs, step_inputs, step_outputs, param_to_step);
      circular_dependency_checker(step_inputs);
      //   loop_checker(Array.from(this.steps, (step) => step.tool));
    }
  }
  async make_workflow_step(
    toolpath_object: cwlTsAuto.WorkflowStep,
    pos: number,
    loadingContext: LoadingContext,
    // ,parentworkflowProv?: ProvenanceProfile,
  ) {
    const t = new WorkflowStep(toolpath_object, pos);
    await t.init(loadingContext);
    return t;
    // , parentworkflowProv);
  }

  async job(
    job_order: CWLObjectType,
    output_callbacks: OutputCallbackType,
    runtimeContext: RuntimeContext,
    workflow_id: FlowyJobURL | null
  ): Promise<WorkflowJob> {
    const builder = await this._init_job(job_order, runtimeContext);

    if (runtimeContext.research_obj != null) {
      if (runtimeContext.toplevel) {
        // Record primary-job.json
        // runtimeContext.research_obj.fsaccess = runtimeContext.make_fs_access('');
        // create_job(runtimeContext.research_obj, builder.job);
      }
    }

    const job = new WorkflowJob(this, runtimeContext,workflow_id,output_callbacks,builder.job);
    getJobWatcher().jobCreated(job)
    return job;

    // runtimeContext = runtimeContext.copy();
    // runtimeContext.part_of = `workflow ${job.name}`;
    // runtimeContext.toplevel = false;

    // const jobiter = job.job(builder.job, output_callbacks, runtimeContext,job.id);
    // for await (const j of jobiter) {
    //   if( j instanceof JobBase){
    //     j.parent_id = job.id
    //   }
    //   yield j;
    // }
  }
}

function used_by_step(step: IWorkflowStep, shortinputid: string): boolean {
  for (const st of step.in_) {
    if (st.valueFrom) {
      if (st.valueFrom.includes(`inputs.${shortinputid}`)) {
        return true;
      }
    }
  }
  if (step.when) {
    if (step.when.includes(`inputs.${shortinputid}`)) {
      return true;
    }
  }
  return false;
}
function handleInput(
  tool: IWorkflowStep,
  embedded_tool: Process,
  toolpath_object: WorkflowStepInput[],
  bound: Set<unknown>,
  _validation_errors,
): WorkflowStepInput[] {
  const inputs: WorkflowStepInput[] = [];
  for (const step_entry of toolpath_object) {
    const inputid = step_entry.id;
    const param: WorkflowStepInput = step_entry;
    const shortinputid = shortname(inputid);
    let found = false;

    for (const tool_entry of embedded_tool.tool.inputs) {
      const frag = shortname(tool_entry.id);
      if (frag === shortinputid) {
        let step_default = null;
        if (param.default_ && tool_entry.default_) {
          step_default = param.default_;
        }
        transferProperties(tool_entry, param);
        param._tool_entry = tool_entry;
        if (step_default !== null) {
          param.default_ = step_default;
        }
        found = true;
        bound.add(frag);
        break;
      }
    }
    if (!found) {
      param.type = 'Any';
      param.used_by_step = used_by_step(tool, shortinputid);
      param.not_connected = true;
    }

    param.id = inputid;
    inputs.push(param);
  }
  return inputs;
}
function handleOutput(
  embedded_tool: Process,
  stepOutputs: (WorkflowStepOutput | string)[],
  bound: Set<unknown>,
  validation_errors,
): WorkflowStepOutput[] {
  const outputs: CommandOutputParameter[] = [];
  stepOutputs.forEach((step_entry) => {
    let param: WorkflowStepOutput;
    let inputid;

    if (isString(step_entry)) {
      param = { type: 'Any' };
      inputid = step_entry;
    } else {
      param = deepcopy(step_entry);
      inputid = step_entry.id;
    }

    const shortinputid = shortname(inputid);
    let found = false;

    for (const tool_entry of embedded_tool.tool.outputs) {
      const frag = shortname(tool_entry.id);
      if (frag === shortinputid) {
        const step_default = null;
        transferProperties(tool_entry, param);
        param._tool_entry = tool_entry;
        if (step_default !== null) {
          param.default_ = step_default;
        }
        found = true;
        bound.add(frag);
        break;
      }
    }
    if (!found) {
      let step_entry_name;

      if (step_entry instanceof Map) {
        step_entry_name = step_entry['id'];
      } else {
        step_entry_name = step_entry;
      }

      validation_errors.push(
        `Workflow step output '${shortname(step_entry_name)}' does not correspond to\n` +
          '\n' +
          `  tool output (expected '${this.embedded_tool.tool.outputs
            .map((tool_entry) => shortname(tool_entry.id))
            .join("', '")}`,
      );
    }

    param.id = inputid;
    outputs.push(param);
  });
  return outputs;
}
export async function default_make_tool(
  toolpath_object: cwlTsAuto.ExpressionTool | cwlTsAuto.CommandLineTool | cwlTsAuto.Workflow | cwlTsAuto.Operation,
  loadingContext: LoadingContext,
): Promise<Process> {
  validate(toolpath_object, loadingContext);
  if (toolpath_object instanceof cwlTsAuto.CommandLineTool) {
    _convert_stdstreams_to_files(toolpath_object);
    const t = new command_line_tool.CommandLineTool(toolpath_object);
    t.init(loadingContext);
    return t;
  } else if (toolpath_object instanceof cwlTsAuto.ExpressionTool) {
    const t = new command_line_tool.ExpressionTool(toolpath_object);
    t.init(loadingContext);
    return t;
  } else if (toolpath_object instanceof cwlTsAuto.Workflow) {
    const t = new Workflow(toolpath_object);
    await t.init(loadingContext);
    return t;
  }
  throw new WorkflowException(`Missing or invalid 'class' field in`); // ${toolpath_object.name}, expecting one of: CommandLineTool, ExpressionTool, Workflow`,);
}
