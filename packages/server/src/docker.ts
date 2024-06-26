import * as fs from 'node:fs';
import * as path from 'node:path';
import { DockerRequirement, ResourceRequirement } from '@flowy/cwl-ts-auto';
import { v4 as uuidv4 } from 'uuid';
import { Builder } from './builder.js';
import { RuntimeContext } from './context.js';
import { Tool } from './cwltypes.js';
import { WorkflowException } from './errors.js';
import { ContainerCommandLineJob } from './job.js';
import { _logger } from './loghandler.js';
import { MakePathMapper, MapperEnt } from './pathmapper.js';
import {
  type CWLObjectType,
  type MutableMapping,
  checkOutput,
  copyTree,
  createTmpDir,
  ensureWritable,
  which,
  getRequirement,
} from './utils.js';

const _IMAGES: Set<string> = new Set();

export class DockerCommandLineJob extends ContainerCommandLineJob {
  docker_exec = 'docker';

  // eslint-disable-next-line @typescript-eslint/no-useless-constructor
  constructor(builder: Builder, joborder: CWLObjectType, make_path_mapper: MakePathMapper, tool: Tool, name: string) {
    super(builder, joborder, make_path_mapper, tool, name);
    // this.inplace_update = true;
  }

  async get_image(docker_requirement: DockerRequirement, pull_image: boolean, force_pull: boolean): Promise<boolean> {
    let found = false;

    if (!docker_requirement.dockerImageId && docker_requirement.dockerPull)
      docker_requirement.dockerImageId = docker_requirement.dockerPull;

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

  append_volume(runtime: string[], source: string, target: string, writable = false): void {
    const options = ['type=bind', `source=${source}`, `target=${target}`];

    if (!writable) {
      options.push('readonly');
    }

    const mount_arg = options.join(',');
    runtime.push(`--mount=${mount_arg}`);

    if (source.startsWith('file://') && !fs.existsSync(source)) {
      fs.mkdirSync(source);
    }
  }

  add_file_or_directory_volume(runtime: string[], volume: MapperEnt, _host_outdir_tgt: string | null): void {
    if (volume.resolved.startsWith('s3://')) {
      if (volume.target.startsWith(this.builder.stagedir)) {
        // If the path of the target is under stagedir, it can be directly downloaded to the target.
        const targetDir = path.dirname(volume.target);
        this.staging.mkdirSync(targetDir, true);
        this.staging.symlinkSync(volume.resolved, volume.target);
        this.append_volume(runtime, volume.target, volume.target);
      } else {
        // In other cases, download under stagedir and link it (like in the case of InitialWorkdir).
        const stagedir = path.join(this.builder.stagedir, `stg${uuidv4()}`);
        const stagefile = path.join(stagedir, path.basename(volume.target));
        this.staging.mkdirSync(stagedir, true);
        this.staging.symlinkSync(volume.resolved, stagefile);
        this.append_volume(runtime, stagefile, volume.target);
      }
    } else if (!volume.resolved.startsWith('_:')) {
      this.append_volume(runtime, volume.resolved, volume.target);
    }
  }
  add_writable_file_volume(
    runtime: string[],
    volume: MapperEnt,
    hostOutdirTgt: string | undefined,
    tmpdirPrefix: string,
  ): void {
    let fileCopy = '';
    if (this.inplace_update) {
      const stagedir = path.join(this.builder.stagedir, `stg${uuidv4()}`);
      const stagefile = path.join(stagedir, path.basename(volume.target));
      this.staging.mkdirSync(stagedir, true);
      this.staging.symlinkSync(volume.resolved, stagefile);
      this.append_volume(runtime, stagefile, volume.target, true);
    } else {
      if (hostOutdirTgt) {
        if (!fs.existsSync(path.dirname(hostOutdirTgt))) {
          fs.mkdirSync(path.dirname(hostOutdirTgt), { recursive: true });
        }
        if (volume.resolved.startsWith('s3://')) {
          // if resolved is s3://, add volume
          this.append_volume(runtime, volume.resolved, volume.target, true);
          return;
        }else {
          fs.copyFileSync(volume.resolved, hostOutdirTgt);
        }
      } else {
        const tmpdir = createTmpDir(tmpdirPrefix);
        fileCopy = path.join(tmpdir, path.basename(volume.resolved));
        fs.copyFileSync(volume.resolved, fileCopy);
        this.append_volume(runtime, fileCopy, volume.target, true);
      }
      ensureWritable(hostOutdirTgt || fileCopy);
    }
  }

  add_writable_directory_volume(
    runtime: string[],
    volume: MapperEnt,
    hostOutdirTgt: string | undefined,
    tmpdirPrefix: string,
  ): void {
    let newDir = '';
    if (volume.resolved.startsWith('_:')) {
      if (!hostOutdirTgt) {
        newDir = path.join(createTmpDir(tmpdirPrefix), path.basename(volume.target));
        this.append_volume(runtime, newDir, volume.target, true);
      } else if (!fs.existsSync(hostOutdirTgt)) {
        fs.mkdirSync(hostOutdirTgt, { recursive: true });
      }
    } else {
      if (this.inplace_update) {
        this.append_volume(runtime, volume.resolved, volume.target, true);
      } else {
        if (!hostOutdirTgt) {
          const tmpdir = createTmpDir(tmpdirPrefix);
          newDir = path.join(tmpdir, path.basename(volume.resolved));
          copyTree(volume.resolved, newDir);
          this.append_volume(runtime, newDir, volume.target, true);
        } else {
          copyTree(volume.resolved, hostOutdirTgt);
        }
        ensureWritable(hostOutdirTgt || newDir);
      }
    }
  }

  _required_env(): { [key: string]: string } {
    return {
      TMPDIR: DockerCommandLineJob.CONTAINER_TMPDIR,
      HOME: this.builder.outdir,
    };
  }
  create_runtime(env: MutableMapping<string>, runtimeContext: RuntimeContext): [string[], string | null] {
    const [dockerReq] = getRequirement(this.tool, DockerRequirement);
    const any_path_okay = dockerReq !== undefined;
    const user_space_docker_cmd = runtimeContext.user_space_docker_cmd;
    let runtime: string[] = [];

    if (user_space_docker_cmd) {
      if (user_space_docker_cmd.includes('udocker')) {
        runtime = runtimeContext.debug
          ? [user_space_docker_cmd, 'run', '--nobanner']
          : [user_space_docker_cmd, '--quiet', 'run', '--nobanner'];
      } else {
        runtime = [user_space_docker_cmd, 'run'];
      }
    } else {
      runtime = [this.docker_exec, 'run', '-i'];
    }

    if (runtimeContext.podman) {
      runtime.push('--userns=keep-id');
    }

    this.append_volume(runtime, path.resolve(this.outdir), this.builder.outdir, true);
    this.append_volume(runtime, path.resolve(this.tmpdir), DockerCommandLineJob.CONTAINER_TMPDIR, true);

    this.add_volumes(
      this.pathmapper,
      runtime,
      runtimeContext.tmpdir_prefix,
      runtimeContext.secret_store,
      any_path_okay,
    );

    if (this.generatemapper) {
      this.add_volumes(
        this.generatemapper,
        runtime,
        runtimeContext.tmpdir_prefix,
        runtimeContext.secret_store,
        any_path_okay,
      );
    }

    if (user_space_docker_cmd) {
      runtime = runtime.map((x) => x.replace(':ro', ''));
      runtime = runtime.map((x) => x.replace(':rw', ''));
    }

    runtime.push(`--workdir=${this.builder.outdir}`);

    if (!user_space_docker_cmd) {
      if (!runtimeContext.no_read_only) {
        runtime.push('--read-only=true');
      }

      if (this.networkaccess) {
        if (runtimeContext.custom_net) {
          runtime.push(`--net=${runtimeContext.custom_net}`);
        }
      } else {
        runtime.push('--net=none');
      }

      if (this.stdout) {
        runtime.push('--log-driver=none');
      }

      //    const [euid, egid] = docker_vm_id();
      const finalEuid = process.geteuid ? process.geteuid() : 1;
      const finalEgid = process.getegid ? process.getegid() : 1;

      if (!runtimeContext.no_match_user && finalEuid && finalEgid) {
        runtime.push(`--user=${finalEuid}:${finalEgid}`);
      }
    }

    if (runtimeContext.rm_container) {
      runtime.push('--rm');
    }

    if (this.builder.resources['cudaDeviceCount']) {
      runtime.push(`--gpus=${this.builder.resources['cudaDeviceCount']}`);
    }

    let cidfile_path: string | null = null;

    // Add parameters to docker to write a container ID file
    if (!runtimeContext.user_space_docker_cmd) {
      let cidfile_dir = '';

      if (runtimeContext.cidfile_dir) {
        cidfile_dir = runtimeContext.cidfile_dir;

        if (!fs.existsSync(cidfile_dir)) {
          throw new Error(
            `--cidfile-dir ${cidfile_dir} error:\n${cidfile_dir} is not a directory, please check it first`,
          );
        }

        if (!fs.statSync(cidfile_dir).isDirectory()) {
          throw new Error(
            `--cidfile-dir ${cidfile_dir} error:\n${cidfile_dir} is not a directory, please check it first`,
          );
        }
      } else {
        cidfile_dir = runtimeContext.createTmpdir();
      }

      const cidfile_name = `${getCurrentTimestamp()}.cid`;
      cidfile_path = path.join(
        cidfile_dir,
        runtimeContext.cidfile_prefix ? `${runtimeContext.cidfile_prefix}-${cidfile_name}` : cidfile_name,
      );
      runtime.push(`--cidfile=${cidfile_path}`);
    }

    for (const [key, value] of Object.entries(this.environment)) {
      runtime.push(`--env=${key}=${value}`);
    }

    const [res_req, _] = getRequirement(this.tool, ResourceRequirement);

    if (runtimeContext.strict_memory_limit && !user_space_docker_cmd) {
      const ram = this.builder.resources['ram'];
      runtime.push(`--memory=${ram}m`);
    } else if (!user_space_docker_cmd && res_req && (res_req.ramMin || res_req.ramMax)) {
      _logger.warn(
        `[job ${this.name}] Skipping Docker software container '--memory' limit despite presence of ResourceRequirement with ramMin and/or ramMax setting. Consider running with --strict-memory-limit for increased portability assurance.`,
      );
    }

    if (runtimeContext.strict_cpu_limit && !user_space_docker_cmd) {
      const cpus = Math.ceil(this.builder.resources['cores']);
      runtime.push(`--cpus=${cpus}`);
    } else if (!user_space_docker_cmd && res_req && (res_req.coresMin || res_req.coresMax)) {
      _logger.warn(
        `[job ${this.name}] Skipping Docker software container '--cpus' limit despite presence of ResourceRequirement with coresMin and/or coresMax setting. Consider running with --strict-cpu-limit for increased portability assurance.`,
      );
    }

    return [runtime, cidfile_path];
  }
}
function getCurrentTimestamp(): string {
  const now = new Date();

  const year = now.getFullYear();
  const month = (now.getMonth() + 1).toString().padStart(2, '0'); // Months are 0-based
  const day = now.getDate().toString().padStart(2, '0');
  const hours = now.getHours().toString().padStart(2, '0');
  const minutes = now.getMinutes().toString().padStart(2, '0');
  const seconds = now.getSeconds().toString().padStart(2, '0');
  const milliseconds = now.getMilliseconds().toString().padStart(3, '0');

  return `${year}${month}${day}${hours}${minutes}${seconds}-${milliseconds}`;
}
export class PodmanCommandLineJob extends DockerCommandLineJob {
  constructor(builder: Builder, joborder: CWLObjectType, make_path_mapper: MakePathMapper, tool: Tool, name: string) {
    super(builder, joborder, make_path_mapper, tool, name);
    this.docker_exec = 'podman';
  }
}
