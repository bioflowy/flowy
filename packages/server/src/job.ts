import * as fs from 'node:fs';
import * as path from 'node:path';
import { DockerRequirement, ShellCommandRequirement } from '@flowy/cwl-ts-auto';
import { v4 as uuidv4 } from 'uuid';
import { JobExec, OutputBinding, OutputSecondaryFile, Runtime } from './JobExecutor.js';
import { Builder } from './builder.js';
import { OutputPortsType } from './collect_outputs.js';
import { RuntimeContext } from './context.js';
import * as expression from './expression.js';
import * as cwlTsAuto from '@flowy/cwl-ts-auto';

import {
  CommandOutputParameter,
  Directory,
  File,
  SecondaryFileSchema,
  Tool,
  isCommandOutputRecordSchema,
} from './cwltypes.js';
import { UnsupportedRequirement, ValueError, WorkflowException } from './errors.js';
import { removeIgnorePermissionError } from './fileutils.js';
import { _logger } from './loghandler.js';
import { MakePathMapper, MapperEnt, PathMapper } from './pathmapper.js';
import { stage_files } from './process.js';
import { SecretStore } from './secrets.js';
import {
  type CWLObjectType,
  type OutputCallbackType,
  getRequirement,
  str,
  quote,
  aslist,
  isStringOrStringArray,
  which,
  checkOutput,
  JobStatus,
  CWLOutputType,
} from './utils.js';
import { getManager } from './server/manager.js';
import { CommandString, CommandStringToString } from './commandstring.js';
import { getJobWatcher, JobWatcher } from './server/job_watcher.js';
import { Expression } from 'kysely';

export function _job_popen(
  job: CommandLineJob,
  outputBaseDir: string,
  commands: CommandString[],
  stdin_path: string | undefined,
  stdout_path: string | undefined,
  stderr_path: string | undefined,
  env: { [key: string]: string },
  cwd: string,
  removeTmpDir: boolean,
  builder: Builder,
  outputBindings: OutputBinding[],
  fileitems: MapperEnt[],
  generatedlist: MapperEnt[],
  make_job_dir: () => string,
  inplace_update: boolean,
  timelimit: number | undefined = undefined,
  dockerExec: string | undefined,
  dockerImage: string | undefined,
  networkaccess:boolean,
  runtime: Runtime

): JobExec {
  const id = job.id;
  const server = getManager();
  const jobExec:JobExec = {
    outputBaseDir,
    id,
    commands,
    stdin_path,
    stdout_path,
    stderr_path,
    env,
    cwd,
    containerOutdir: builder.outdir,
    tmpDir: job.tmpdir,
    removeTmpDir: removeTmpDir,
    outputBindings,
    fileitems,
    generatedlist,
    timelimit,
    inplace_update,
    dockerExec,
    dockerImage,
    networkaccess,
    runtime,
  }
  return jobExec;
}
type CollectOutputsType = (
  str: string,
  int: number,
  isCwlOutput: boolean,
  results: OutputPortsType,
) => Promise<CWLObjectType>; // Assuming functools.partial as any

export abstract class JobBase {
  readonly id: string;
  readonly name: string;
  readonly type: "CommandLine" | "Expression" | "Workflow" ;
  processStatus: JobStatus;
  results: CWLObjectType;
  joborder: CWLObjectType;
  parent_id: string;
  abstract getOutdirs(): string[]
  abstract run(runtimeContext: RuntimeContext): Promise<void>
  constructor(id:string,name: string,type :"CommandLine" | "Expression" | "Workflow"){
    this.id = id
    this.name = name
    this.type = type
  }
}
export class CommandLineJob extends JobBase{
  builder: Builder;
  base_path_logs: string;
  make_path_mapper: MakePathMapper;
  tool: Tool;
  stdin?: string;
  stderr?: string;
  stdout?: string;
  resources: { [key: string]: number };
  successCodes: number[];
  temporaryFailCodes: number[];
  permanentFailCodes: number[];
  command_line: CommandString[];
  pathmapper: PathMapper;
  generatemapper?: PathMapper;
  collect_outputs?: CollectOutputsType;
  output_callback?: OutputCallbackType;
  outdir: string;
  tmpdir: string;
  environment: { [key: string]: string };
  generatefiles: Directory = { listing: [], basename: '', class: 'Directory' };
  stagedir?: string;
  inplace_update: boolean;
  prov_obj?: any; // ProvenanceProfile;
  parent_wf?: any; // ProvenanceProfile;
  timelimit?: number;
  networkaccess: boolean;
  mpi_procs?: number;
  cwlVersion: string = "unkown";

  constructor(
    builder: Builder,
    joborder: CWLObjectType,
    make_path_mapper: (
      param1: (File | Directory)[],
      param2: string,
      param3: RuntimeContext,
      param4: boolean,
    ) => PathMapper,
    tool: Tool,
    name: string,
    workflow_id: string,
  ) {
    super(uuidv4(),name,"CommandLine")
    this.parent_id = workflow_id
    this.builder = builder;
    this.joborder = joborder;
    this.resources = builder.resources;
    // TODO
    this.base_path_logs = '/tmp';
    this.stdin = undefined;
    this.stderr = undefined;
    this.stdout = undefined;
    this.successCodes = [];
    this.temporaryFailCodes = [];
    this.permanentFailCodes = [];
    this.tool = tool;
    this.command_line = [];
    this.pathmapper = new PathMapper([], '', '');
    this.make_path_mapper = make_path_mapper;
    this.generatemapper = undefined;
    this.collect_outputs = undefined;
    this.output_callback = undefined;
    this.outdir = '';
    this.tmpdir = '';
    this.environment = {};
    this.inplace_update = false;
    this.prov_obj = undefined;
    this.parent_wf = undefined;
    this.timelimit = undefined;
    this.networkaccess = false;
    this.mpi_procs = undefined;

  }
  getOutdirs():string[]{
    return this.outdir?[this.outdir]:[]
  }
  async do_eval(
    ex: CWLOutputType | undefined,
    context: any = undefined,
    recursive = false,
    strip_whitespace = true,
  ): Promise<CWLOutputType | undefined> {
    if (recursive) {
      if (ex instanceof Map) {
        const mutatedMap: { [key: string]: any } = {};
        ex.forEach((value, key) => {
          mutatedMap[key] = this.do_eval(value, context, recursive);
        });
        return mutatedMap;
      }
      if (Array.isArray(ex)) {
        const rets: CWLOutputType[] = [];
        for (let index = 0; index < ex.length; index++) {
          const ret = await this.do_eval(ex[index], context, recursive);
          rets.push(ret);
        }
        return rets;
      }
    }

    let resources = this.resources;
    if (this.resources && this.resources['cores']) {
      const cores = resources['cores'];
      resources = { ...resources };
      resources['cores'] = Math.ceil(cores);
    }
    const [javascriptRequirement] = getRequirement(this.tool, cwlTsAuto.InlineJavascriptRequirement);
    const ret = await expression.do_eval(
      ex as CWLObjectType,
      this.joborder,
      javascriptRequirement,
      this.outdir,
      this.tmpdir,
      resources,
      context,
      strip_whitespace,
      this.cwlVersion,
    );
    return ret;
  }

  toString(): string {
    return `CommandLineJob(${this.name})`;
  }
  _get_dockerExec(){
    return this.docker_exec
  }
  _get_dockerImage(){
    return this.dockerImage
  }

  _setup(runtimeContext: RuntimeContext): void {

    const is_streamable = (file: string): boolean => {
      if (!runtimeContext.streaming_allowed) return false;
      for (const inp of Object.values(this.joborder)) {
        if (typeof inp === 'object' && inp['location'] == file) return inp['streamable'];
      }
      return false;
    };

    for (const knownfile of this.pathmapper.files()) {
      const p = this.pathmapper.mapper(knownfile);
      if (p.type == 'File' && p.resolved.startsWith('file:/') && !fs.existsSync(p.resolved) && p.staged) {
        if (!(is_streamable(knownfile) && fs.statSync(p.resolved).isFIFO())) {
          throw new WorkflowException(
            `Input file ${knownfile} (at ${
              this.pathmapper.mapper(knownfile).resolved
            }) not found or is not a regular file.`,
          );
        }
      }
    }

    if (this.generatefiles.listing) {
      runtimeContext.outdir = this.outdir;
      this.generatemapper = this.make_path_mapper(
        this.generatefiles.listing,
        this.outdir,
        runtimeContext,
        false,
      );
      // if (_logger.isEnabledFor(logging.DEBUG)) {
      //     _logger.debug(
      //         "[job %s] initial work dir %s",
      //         this.name,
      //         JSON.stringify({ p: this.generatemapper.mapper(p) for(p of this.generatemapper.files()) }, null, 4),
      //     );
      // }
    }
    this.base_path_logs = runtimeContext.set_log_dir(this.outdir, runtimeContext.log_dir, this.name);
  }
  async _execute(
    runtime: string[],
    env: { [id: string]: string },
    runtimeContext: RuntimeContext,
  ) {
    const manager = getManager();
    const jobExec = await this._execute2(runtime,env,runtimeContext)
    await manager.execute(this,jobExec,runtimeContext)
  }
  async _execute2(
    runtime: string[],
    env: { [id: string]: string },
    runtimeContext: RuntimeContext,
  ) :Promise<JobExec> {
    const scr = getRequirement(this.tool, ShellCommandRequirement)[0];

    const shouldquote = scr !== null;
    // TODO mpi not supported
    // if (this.mpi_procs) {
    //   const menv = runtimeContext.mpi_config;
    //   const mpi_runtime = [menv.runner, menv.nproc_flag, this.mpi_procs.toString(), ...menv.extra_flags];
    //   runtime = [...mpi_runtime, ...runtime];
    //   menv.pass_through_env_vars(env);
    //   menv.set_env_vars(env);
    // }
    const command_line = runtime
      .concat(this.command_line.map(CommandStringToString))
      .map((arg) => (shouldquote ? quote(arg.toString()) : arg.toString())) // TODO
      .join(' \\\n');
    const tmp2 = [
      this.stdin ? ` < ${this.stdin}` : '',
      this.stdout ? ` > ${path.join(this.base_path_logs, this.stdout)}` : '',
      this.stderr ? ` 2> ${path.join(this.base_path_logs, this.stderr)}` : '',
    ];
    _logger.info(`[job ${this.name}] %${this.outdir}$ ${command_line} ${tmp2[0]} ${tmp2[1]} ${tmp2[2]}`);
    try {
      let stdin_path: string | undefined;
      if (this.stdin !== undefined) {
        const rmap = this.pathmapper.reversemap(this.stdin);
        if (rmap === undefined) {
          throw new WorkflowException(`${this.stdin} missing from pathmapper`);
        } else {
          if(!rmap[0].startsWith("_:")){
            stdin_path = rmap[1];
          }else{
            stdin_path = this.stdin
          }
        }
      }

      const stderr_stdout_log_path = (
        base_path_logs: string,
        stderr_or_stdout: string | undefined,
      ): string | undefined => {
        if (stderr_or_stdout !== undefined) {
          return path.join(base_path_logs, stderr_or_stdout);
        }
        return undefined;
      };

      const stderr_path = stderr_stdout_log_path(this.base_path_logs, this.stderr);
      const stdout_path = stderr_stdout_log_path(this.base_path_logs, this.stdout);
      // let commands = runtime.concat(this.command_line).map((x) => x.toString());
      if (runtimeContext.secret_store !== undefined) {
        // TODO 
        // commands = runtimeContext.secret_store.retrieve(commands as any) as string[];
        env = runtimeContext.secret_store.retrieve(env as any) as { [id: string]: string };
      }
      const fileitems: MapperEnt[] = [];
      if (this.builder.pathmapper) {
        for (const [_, item] of this.builder.pathmapper.items_exclude_children()) {
          fileitems.push(item);
        }
      }
      const generatedlist: MapperEnt[] = [];
      if (this.generatefiles.listing) {
        if (this.generatemapper) {
          generatedlist.push(...this.generatemapper.items_exclude_children().map(([_key, value]) => value));
        } else {
          throw new ValueError(`'listing' in self.generatefiles but no generatemapper was setup.`);
        }
      }
      
      const outputBindings = await createOutputBinding(this.tool.outputs, this);
      const jobExec = _job_popen(
        this,
        runtimeContext.basedir,
        this.command_line,
        stdin_path,
        stdout_path,
        stderr_path,
        env,
        this.outdir,
        runtimeContext.rm_tmpdir,
        this.builder,
        outputBindings,
        fileitems,
        generatedlist,
        () => runtimeContext.createOutdir(),
        this.inplace_update,
        this.timelimit,
        this._get_dockerExec(),
        this._get_dockerImage(),
        this.networkaccess,
        {
          custom_net: runtimeContext.custom_net
        }
      )
      return jobExec
    } catch (err) {
      if (err instanceof Error) {
        _logger.error(`[job ${this.name}] Job error${err.message}\n${err.stack}`);
      }
    }
  }
  async executed(rcode, isCwlOutput, fileMap){
    let outputs: any = {};
    let processStatus:JobStatus = 'success';
    try{

      if (this.successCodes.includes(rcode)) {
        processStatus = 'success';
      } else if (this.temporaryFailCodes.includes(rcode)) {
        processStatus = 'temporaryFail';
      } else if (this.permanentFailCodes.includes(rcode)) {
        processStatus = 'permanentFail';
      } else if (rcode === 0) {
        processStatus = 'success';
      } else {
        processStatus = 'permanentFail';
      }

      if (processStatus !== 'success') {
        if (rcode < 0) {
          _logger.warn(`[job ${this.name}] was terminated by signal:`);
        } else {
          _logger.warn(`[job ${this.name}] exited with status: ${rcode}`);
        }
      }

      outputs = await this.collect_outputs(this.outdir, rcode, isCwlOutput, fileMap);
    } catch (err) {
      if (err instanceof Error) {
        _logger.error(`[job ${this.name}] Job error${err.message}\n${err.stack}`);
      }
      processStatus = 'permanentFail';
    }
    //  catch {
    //     _logger.exception("Exception while running job");
    //     processStatus = "permanentFail";
    // }
    if (processStatus !== 'success') {
      _logger.warn(`[job ${this.name}] completed ${processStatus}`);
    } else {
      _logger.info(`[job ${this.name}] completed ${processStatus}`);
    }

    if (_logger.isDebugEnabled()) {
      _logger.debug(`[job ${this.name}] outputs ${JSON.stringify(outputs, null, 4)}`);
    }
    this.processStatus = processStatus
    this.results = outputs
    if (this.output_callback) {
      this.output_callback(outputs, processStatus);
    }

    if (false && this.stagedir !== undefined && fs.existsSync(this.stagedir)) {
      _logger.debug(`[job ${this.name}] Removing input staging directory ${this.stagedir}`);
      await removeIgnorePermissionError(this.stagedir);
    }
    getJobWatcher().jobFinished(this,rcode,outputs)
  }

  _preserve_environment_on_containers_warning(varname?: Iterable<string>): void {
    // By default, don't do anything; ContainerCommandLineJob below
    // will issue a warning.
  }

  prepare_environment(runtimeContext: any, envVarReq: Record<string, string>): void {
    // Start empty
    const env: Record<string, string> = {};

    // Preserve any env vars
    if (runtimeContext.preserve_entire_environment) {
      this._preserve_environment_on_containers_warning();
      Object.assign(env, process.env);
    } else if (runtimeContext.preserve_environment) {
      this._preserve_environment_on_containers_warning(runtimeContext.preserve_environment);
      for (const key of runtimeContext.preserve_environment) {
        if (process.env[key]) {
          env[key] = process.env[key];
        } else {
          console.warn(`Attempting to preserve environment variable ${key} which is not present`);
        }
      }
    }

    // Set required env vars
    Object.assign(env, this._required_env());

    // Apply EnvVarRequirement
    Object.assign(env, envVarReq);

    // Set on ourselves
    this.environment = env;
  }
  docker_exec = 'docker';
  dockerImage:string|undefined;

  async get_image(docker_requirement: DockerRequirement, pull_image: boolean, force_pull: boolean): Promise<boolean> {
    let found = false;

    if (!docker_requirement.dockerImageId && docker_requirement.dockerPull){
      docker_requirement.dockerImageId = docker_requirement.dockerPull;
     this.dockerImage =  docker_requirement.dockerImageId
    }
 
    // synchronized (_IMAGES_LOCK, () => {
    if (docker_requirement.dockerImageId in _IMAGES) return true;
    // });
    const images = await checkOutput([this.docker_exec, 'images', '--no-trunc', '--all']);
    for (const line of images.split('\n')) {
      try {
        const match = line.match('^([^ ]+)\\s+([^ ]+)\\s+([^ ]+)');
        const split = docker_requirement.dockerImageId.split(':');
        if (split.length == 1) split.push('latest');
        else if (split.length == 2) {
          if (!split[1].match('[\\w][\\w.-]{0,127}')) split[0] = `${split[0]}:${split[1]}`;
          split[1] = 'latest';
        } else if (split.length == 3) {
          if (split[2].match('[\\w][\\w.-]{0,127}')) {
            split[0] = `${split[0]}:${split[1]}`;
            split[1] = split[2];
            split.splice(2, 1);
          }
        }

        if (match && ((split[0] == match[1] && split[1] == match[2]) || docker_requirement.dockerImageId == match[3])) {
          this.dockerImage = docker_requirement.dockerImageId
          found = true;
          break;
        }
      } catch (error) {
        _logger.warn(`Error parsing docker images output: ${error}`);
        continue;
      }
    }

    if ((force_pull || !found) && pull_image) {
      let cmd: string[] = [];
      if ('dockerPull' in docker_requirement) {
        cmd = [this.docker_exec, 'pull', docker_requirement['dockerPull'].toString()];
        _logger.info(cmd.toString());
        await checkOutput(cmd);
        found = true;
      }
    }
    if (found) {
      // synchronized (_IMAGES_LOCK, () => {
      _IMAGES.add(docker_requirement['dockerImageId']);
      // });
    }

    return found;
  }
  async get_from_requirements(
    r: DockerRequirement,
    pull_image: boolean,
    force_pull: boolean,
  ): Promise<string | undefined> {
    const rslt = await which(this.docker_exec);
    if (!rslt) {
      throw new WorkflowException(`${this.docker_exec} executable is not available`);
    }
    await this.get_image(r, pull_image, force_pull);
    if (r) {
      return r['dockerImageId'];
    }
    throw new WorkflowException(`Docker image ${r['dockerImageId']} not found`);
  }
  async run(runtimeContext: RuntimeContext): Promise<void> {
    const jobExec = await this.run2(runtimeContext)
    getJobWatcher().jobStarted(this);
    await getManager().execute(this,jobExec,runtimeContext)
  }
  async run2(runtimeContext: RuntimeContext): Promise<JobExec> {
    const [docker_req, docker_is_req] = getRequirement(this.tool, DockerRequirement);
    let img_id: any = undefined;
    const user_space_docker_cmd = runtimeContext.user_space_docker_cmd;
    if (docker_req !== undefined && user_space_docker_cmd) {
      if (docker_req.dockerImageId) {
        img_id = docker_req.dockerImageId;
      } else if (docker_req.dockerPull) {
        img_id = String(docker_req.dockerPull);
        const cmd = [user_space_docker_cmd, 'pull', img_id];
        _logger.info(String(cmd));
      } else {
        throw new WorkflowException(
          "Docker image must be specified as 'dockerImageId' or 'dockerPull' when using user space implementations of Docker",
        );
      }
    } else {
      try {
        if (docker_req !== undefined && runtimeContext.use_container) {
          img_id = await this.get_from_requirements(
            docker_req,
            runtimeContext.pull_image,
            runtimeContext.force_docker_pull
          );
        }
        if (docker_req !== undefined && img_id === undefined && runtimeContext.use_container) {
          throw new Error('Docker image not available');
        }
        if (this.prov_obj !== undefined && img_id !== undefined && runtimeContext.process_run_id !== undefined) {
          const container_agent = this.prov_obj.document.agent(uuidv4, {
            'prov:type': 'SoftwareAgent',
            'cwlprov:image': img_id,
            'prov:label': `Container execution of image ${img_id}`,
          });
          this.prov_obj.document.wasAssociatedWith(runtimeContext.process_run_id, container_agent);
        }
      } catch (err) {
        const container = runtimeContext.singularity ? 'Singularity' : 'Docker';
        _logger.debug(`${container} error`, err);
        if (docker_is_req) {
          throw new UnsupportedRequirement(`${container} is required to run this tool: ${String(err)}`);
        } else {
          throw new WorkflowException(
            `${container} is not available for this tool, try --no-container to disable ${container}, or install a user space Docker replacement like uDocker with --user-space-docker-cmd.: ${err}`,
          );
        }
      }
    }

    this._setup(runtimeContext);

    stage_files( this.pathmapper, null, {
      ignore_writable: true,
      symlink: true,
      secret_store: runtimeContext.secret_store,
    });
    if (this.generatemapper) {
      stage_files(this.generatemapper, null, {
        ignore_writable: this.inplace_update,
        symlink: true,
        secret_store: runtimeContext.secret_store,
      });
    }

    return this._execute2([], this.environment, runtimeContext);
  }

  _required_env(): { [key: string]: string } {
    const env: { [key: string]: string } = {};
    env['HOME'] = this.outdir;
    env['TMPDIR'] = this.tmpdir;
    env['PATH'] = process.env['PATH'];
    for (const extra of ['SYSTEMROOT', 'QEMU_LD_PREFIX']) {
      if (extra in process.env) {
        env[extra] = process.env[extra];
      }
    }
    return env;
  }
}

async function createOutputBinding(outputs: CommandOutputParameter[], job: CommandLineJob): Promise<OutputBinding[]> {
  const outputBindings: OutputBinding[] = [];
  for (const output of outputs) {
    const outputType = output.type;
    if (isCommandOutputRecordSchema(outputType)) {
      const obs = await createOutputBinding(outputType.fields, job);
      outputBindings.push(...obs);
    }
    if (output.outputBinding) {
      const globpatterns: string[] = [];
      for (const glob of aslist(output.outputBinding.glob)) {
        const gb = await job.do_eval(glob);
        if (gb) {
          if (isStringOrStringArray(gb)) {
            globpatterns.push(...aslist(gb));
          } else {
            throw new WorkflowException(
              'Resolved glob patterns must be strings or list of strings, not ' +
                `${str(gb)} from ${str(output.outputBinding.glob)}`,
            );
          }
        }
      }
      const binding: OutputBinding = {
        streamable: output.streamable,
        name: output.name,
        glob: globpatterns,
        secondaryFiles: aslist(output.secondaryFiles).map(convertSecondaryFiles),
        outputEval: output.outputBinding.outputEval,
        loadListing: output.outputBinding.loadListing,
        loadContents: output.outputBinding.loadContents ?? false,
      };
      outputBindings.push(binding);
    }
  }
  return outputBindings;
}
function convertSecondaryFiles(file: SecondaryFileSchema): OutputSecondaryFile {
  if (typeof file.required === 'string') {
    return { pattern: file.pattern, requiredString: file.required };
  } else if (typeof file.required === 'boolean') {
    return { pattern: file.pattern, requiredBoolean: file.required };
  } else {
    return { pattern: file.pattern };
  }
}
const _IMAGES: Set<string> = new Set();

