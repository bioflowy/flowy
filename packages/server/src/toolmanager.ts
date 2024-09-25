import { v4 } from "uuid";
import { type NewTool, Tool, db } from "./databases";
import * as cwlTsAuto from '@flowy/cwl-ts-auto';

import * as path from "path";
import * as fs from "fs";
import * as crypto  from "crypto";
import { loadDocument } from "./loader";
import { default_make_tool } from "./workflow";
import { LoadingContext } from "./context";
import * as url from 'node:url';
import { DefaultFetcher, Fetcher } from "@flowy/cwl-ts-auto/dist/util/Fetcher";
import { Process } from "./process";
export class DefaultFetcher2 extends Fetcher {
    constructor(
      private readonly rootUrl: string,
        private readonly toolManager: ToolManager,
        private readonly urlToIdMap: Map<string, string> = new Map<string, string>(),
    ) {
        super();
    }
    fetcher: DefaultFetcher = new DefaultFetcher();
    async fetchText(url: string, _?: string[]): Promise<string> {
      if(this.rootUrl === url){
        return this.fetcher.fetchText(url)
      }
      if(this.urlToIdMap.has(url)){
        const tool = await this.toolManager.getTool(this.urlToIdMap.get(url))
        if(tool){
          return tool.content
        }
      }
      const tool = await this.toolManager.importTool(url)
      this.urlToIdMap.set(url,tool.id)

      return tool.content;
    }
    getUrlToIdMap(): Map<string, string> {
      return this.urlToIdMap;
    }
    checkExists(urlString: string): boolean {
      return this.fetcher.checkExists(urlString);
    }
    urljoin(baseUrlString: string, urlString: string): string {
      const url = this.fetcher.urljoin(baseUrlString, urlString)
      return url;
    }
  }
  export class ToolManager {
    async getTool(toolId: string): Promise<Tool>{
        return await db.selectFrom('tool').selectAll().where("id","=", toolId).executeTakeFirst()
    }
    async importTool(toolUrl: string,name?: string,version?:string): Promise<Tool>{
        const toolPath = url.fileURLToPath(toolUrl)
        if(name === undefined){
            name = path.basename(toolPath)
        }
        const loadingContext = new LoadingContext({});
        loadingContext.construct_tool_object = default_make_tool;
        const loadingOptions = new cwlTsAuto.LoadingOptions({});
        loadingContext.loadingOptions = loadingOptions;
        const fetcher = new DefaultFetcher2(toolUrl,this)
        loadingOptions.fetcher = fetcher
        loadingContext.baseuri = path.dirname(toolPath);
        try{
          await loadDocument(toolUrl.toString(), loadingContext);
        }catch(e){
        }
        const idMap = fetcher.getUrlToIdMap()
        const content = await fs.promises.readFile(toolPath, { encoding: 'utf-8' });
        const hash = crypto.createHash('sha1')
        hash.update(content)
        hash.update(JSON.stringify(Array.from(idMap.entries())))
        const checksum = hash.digest("hex");
        const tool2 = await db.selectFrom('tool').selectAll().where("name","=", name).where("hash","=", checksum).executeTakeFirst()
        if(tool2){
            return tool2
        }
        const toolId = v4()
        idMap.set(toolUrl,toolId)
        const tool = {
            id: v4(),
            name: name,
            hash: checksum,
            content: content,
            comefrom: toolUrl,
            version: version?version:null,
            references: JSON.stringify(Array.from(idMap.entries())),
            created_at: new Date(),
        }
        await db.insertInto('tool').values(tool).execute()
        return tool
    }
    async loadTool(toolId): Promise<[Process,string]>{
      const t = await this.getTool(toolId);
      const urlToIdMap = t.references?new Map<string,string>(JSON.parse(t.references)):new Map<string,string>();
      const loadingContext = new LoadingContext({});
      loadingContext.construct_tool_object = default_make_tool;
      const loadingOptions = new cwlTsAuto.LoadingOptions({});
      loadingContext.loadingOptions = loadingOptions;
      const fetcher = new DefaultFetcher2("",this,urlToIdMap)
      loadingOptions.fetcher = fetcher
      loadingContext.baseuri = path.dirname(t.comefrom);
      try{
        const [tool,status] = await loadDocument(t.comefrom, loadingContext);
        return [tool,t.comefrom]
      }catch(e){
        console.log(e)
      }
  }

}
const toolManager = new ToolManager();

export function getToolManager(): ToolManager {
    return toolManager;
}