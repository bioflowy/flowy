import yargs from 'yargs';
import { hc } from 'hono/client'
import { hideBin } from 'yargs/helpers';
import { ExecuteJobRoute } from '@flowy/server';

export interface Args {
  tool_path?: string;
  job_path?: string;
  outdir?: string;
  basedir?: string;
  quiet?: boolean;
}
function wait(ms:number) {
  return new Promise(resolve => setTimeout(resolve, ms));
}
export async function main(args: Args): Promise<number> {
  const client = hc<ExecuteJobRoute>('http://localhost:5173/api/')
  const res = await client.executeJob.$post({json:{
    tool_path: args.tool_path,
    job_path: args.job_path,
    clientWorkDir: process.cwd(),
    basedir: args.basedir??'file://'+process.cwd(),
  }});
  const rlst = await res.json()
  while(true){
    const res = await client.getJobInfo.$post({json:{jobId:rlst.jobId}})
    if(res.status != 200){
      console.log(res.statusText)
      return 1;
    }
    const rslt = await res.json()
    if(rslt.status === "finished"){
      const result = rslt.result
      const status  = rslt.resultStatus
      if (status === 'success') {
        process.stdout.write(`${JSON.stringify(result)}\n`);
        return new Promise((resolve) => {
          process.stdout.end(() => {
            resolve(0);
          });
        });
      } else {
        process.stderr.write(result+"\n");
        return 1;
      }
    }else{
      await wait(1000)
    }
  }
}

export async function executeClient() {
  // MEMO: ↓この行に breakpoint を仕掛けて、デバッグ実行してみよう。
  // eslint-disable-next-line no-console
  const arg = yargs(hideBin(process.argv))
    .command('$0 <tool_path> [job_path]', 'execute cwl workflow')
    .positional('tool_path', {
      description: 'Path to cwl file',
      type: 'string',
      required: true,
    })
    .positional('job_path', {
      description: 'job file path',
      type: 'string',
    })
    .option('outdir', {
      alias: 'o',
      description: 'Output directory',
      type: 'string',
    })
    .option('basedir', {
      alias: 'b',
      description: 'base directory for input',
      type: 'string',
    })
    .option('quiet', {
      alias: 'q',
      description: 'supress log output',
      type: 'boolean',
    })
    .help()
    .parseSync();
  return main(arg);
}

executeClient().then((code) => {
    process.exit(code);
    }).catch((e) => {
    console.error(e);
    });