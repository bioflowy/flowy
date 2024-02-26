import { JavascriptException } from "./errors";
import { spawn } from 'child_process';
import { CWLOutputType } from "./utils";
import * as fs from 'fs';
import * as path from 'path';
import * as os from 'os';
import {fileContent} from "./cwlNodeEngine.js";
export function get_js_engine() {
    return new JSEngine1();
  }
  
export interface JSEngine {
    eval(scan: string, jslib: string, rootvars: { [key: string]: unknown }): Promise<CWLOutputType>;
  }
  interface ExecJsProcessResult {
    returnCode: number;
    stdout: string;
    stderr: string;
  }
let workDir : string | undefined;
async function initJsWorkdir(): Promise<string>{
  if(workDir){
    return workDir;
  }
  return new Promise((resolve,reject)=>{
    fs.mkdtemp(path.join(os.tmpdir(), "nodejswork"), (err, folder) => {
    if (err){
      reject(err);
    };
    fs.writeFileSync(path.join(folder, "cwlNodeEngine.js"), fileContent, "utf-8")
    workDir = folder;
    resolve(folder);
  })
});
}
function execProcess(cmd: string,args:string[],cwd:string | undefined,stdin:string | undefined): Promise<ExecJsProcessResult> {
    return new Promise<ExecJsProcessResult>((resolve, reject) => {
        const proc = spawn(cmd,args, {cwd, stdio: [stdin?'pipe':'ignore', 'pipe', 'pipe'] });
        let stdoutData = "";
        let stderrData = "";
  
        proc.stdout.on("data", (data: Buffer) => {
          console.log(`data len=${data.length}`)
            stdoutData += data.toString();
        });
  
        proc.stderr.on("data", (data: Buffer) => {
            stderrData += data.toString();
        });
  
        proc.on("error", (err) => {
            reject(err);
        });
  
        proc.on("exit", (code) => {
            resolve({
                returnCode: code ?? 0,
                stdout: stdoutData,
                stderr: stderrData,
            });
        });
        if(stdin){
          proc.stdin?.write(stdin);
          proc.stdin.end();
        }
    });
  }
  function jshead(engine_config: Array<string>, rootvars: Record<string, any>): string {
    return [
        ...engine_config,
        ...Object.entries(rootvars).map(([key, value]) => 
            `var ${key} = ${JSON.stringify(value, null, 4)};`
        )
    ].join("\n");
}
  class JSEngine1 implements JSEngine {
    code_fragment_to_js(jscript: string, jslib = ''): string {
      let inner_js = '';
      if (jscript.length > 1 && jscript[0] == '{') {
        inner_js = jscript;
      } else {
        inner_js = `{return ${jscript};}`;
      }
  
      return `"use strict";\n${jslib}\n(function()${inner_js})()`;
    }
  
    async eval(expr: string, jslib: string, rootvars: { [key: string]: unknown }): Promise<CWLOutputType> {
      const jslib2 = jshead([jslib],rootvars)
      const jsText = this.code_fragment_to_js(expr,jslib2)
      const jsonText = JSON.stringify(jsText)
      const workDir = await initJsWorkdir();
      const nodeProcess = await execProcess("node",["cwlNodeEngine.js"],workDir,jsonText+"\n")
      if(nodeProcess.returnCode !== 0){
        throw new JavascriptException(nodeProcess.stderr)
      }
      const rslt_str = nodeProcess.stdout
      console.log(rslt_str.length)
      const rslt = JSON.parse(rslt_str);
      return rslt;
    }
  }
  