import { v4 } from "uuid";
import { type NewToolInfo, ToolInfo, db } from "./databases";
import * as cwlTsAuto from "@flowy/cwl-ts-auto";

import * as path from "path";
import * as fs from "fs";
import * as crypto from "crypto";
import { loadDocument, loadTool } from "./loader";
import { default_make_tool } from "./workflow";
import { LoadingContext } from "./context";
import * as url from "node:url";
import { DefaultFetcher, Fetcher } from "@flowy/cwl-ts-auto/dist/util/Fetcher";
import { Process } from "./process";
import { createFlowyDatasetURL, createFlowyToolURL,FlowyDatasetURL,FlowyToolURL, FlowyURL } from "./flowyurl";
import { URL } from "url";
import { getDatasetManager } from "./datasetmanager";

export class DefaultFetcher2 extends Fetcher {
  constructor(
    private readonly rootUrl: string,
    private readonly toolManager: ToolManager,
    private readonly urlToIdMap = new Map<string, FlowyToolURL>()
  ) {
    super();
  }
  fetcher: DefaultFetcher = new DefaultFetcher();
  async fetchText(url: string, _?: string[]): Promise<string> {

    if (this.rootUrl === url) {
      return this.fetcher.fetchText(url);
    }
    if (this.urlToIdMap.has(url)) {
      const tool = await this.toolManager.getToolInfo(this.urlToIdMap.get(url));
      if (tool) {
        return tool.content;
      }
    }
    const tool = await this.toolManager.importTool(new URL(url));
    this.urlToIdMap.set(url, new FlowyToolURL(tool.id));

    return tool.content;
  }
  getUrlToIdMap(): Map<string, FlowyURL> {
    return this.urlToIdMap;
  }
  checkExists(urlString: string): boolean {
    return this.fetcher.checkExists(urlString);
  }
  urljoin(baseUrlString: string, urlString: string): string {
    const url = this.fetcher.urljoin(baseUrlString, urlString);
    return url;
  }
}
export class ToolInfoFetcher extends Fetcher {
  constructor(
    private readonly urlToContentMap = new Map<string, string>()
  ) {
    super();
  }
  fetcher: DefaultFetcher = new DefaultFetcher();
  async fetchText(url: string, _?: string[]): Promise<string> {
    return this.urlToContentMap.get(url);
  }
  checkExists(urlString: string): boolean {
    return this.urlToContentMap.has(urlString);
  }
  urljoin(baseUrlString: string, urlString: string): string {
    const url = this.fetcher.urljoin(baseUrlString, urlString);
    return url;
  }
}
// 再帰的に探索して特定のクラスのインスタンスを抽出する関数
function extractInstances<T>(
  data: any,
  targetClass: new (...args: any[]) => T
): T[] {
  const results: T[] = [];

  function traverse(item: any) {
    if (item instanceof targetClass) {
      results.push(item);
    } else if (Array.isArray(item)) {
      item.forEach(traverse);
    } else if (typeof item === 'object' && item !== null) {
      Object.values(item).forEach(traverse);
    }
  }

  traverse(data);
  return results;
}

export class ToolManager {
  async getToolInfo(toolUrl: FlowyToolURL): Promise<ToolInfo> {
    return await db
      .selectFrom("tool")
      .selectAll()
      .where("id", "=", toolUrl.getId())
      .executeTakeFirst();
  }
  async importTool(
    toolUrl: URL,
    name?: string,
    version?: string
  ): Promise<ToolInfo> {
    const toolPath = url.fileURLToPath(toolUrl);
    if (name === undefined) {
      name = path.basename(toolPath);
    }
    const loadingOptions = new cwlTsAuto.LoadingOptions({});
    const fetcher = new DefaultFetcher2(toolUrl.toString(), this);
    loadingOptions.fetcher = fetcher;
    const idMap = fetcher.getUrlToIdMap();
    try{
      const doc = await cwlTsAuto.loadDocument(toolUrl.toString(), toolUrl.toString(), loadingOptions);
      const loadingContext = new LoadingContext({});
      loadingContext.construct_tool_object = default_make_tool;
      loadingContext.loadingOptions = loadingOptions;
      const [process,status] =await loadTool(doc,toolUrl.toString(), loadingContext);
      const files = extractInstances(doc, cwlTsAuto.File);
      for(const file of files){
        const dataset =  await getDatasetManager().importDataset(new URL(file.location));
        if(dataset){
          idMap.set(file.location, new FlowyDatasetURL(dataset.id));
        }
      }  
    }catch(e){
      console.error(e);
    }
    const content = await fs.promises.readFile(toolPath, { encoding: "utf-8" });
    const hash = crypto.createHash("sha1");
    hash.update(content);
    hash.update(JSON.stringify(Array.from(idMap.entries())));
    const checksum = hash.digest("hex");
    const tool2 = await db
      .selectFrom("tool")
      .selectAll()
      .where("name", "=", name)
      .where("hash", "=", checksum)
      .executeTakeFirst();
    if (tool2) {
      return tool2;
    }
    const toolId = v4();
    idMap.set(toolUrl.toString(), new FlowyToolURL(toolId));
    const tool = {
      id: v4(),
      name: name,
      hash: checksum,
      content: content,
      comefrom: toolUrl.toString(),
      version: version ? version : null,
      references: JSON.stringify(Array.from(idMap.entries())),
      created_at: new Date(),
    };
    await db.insertInto("tool").values(tool).execute();
    return tool;
  }
  async loadTool(toolUrl: FlowyToolURL): Promise<[Process, string]> {
    // fetch tool files from database
    const t = await this.getToolInfo(toolUrl);
    const urlToDatasetIdtMap = new Map<string, FlowyDatasetURL>();
    const urlToContentMap = new Map<string, string>();
    urlToContentMap.set(t.comefrom, t.content);
    for(const [url, id] of JSON.parse(t.references)){
      if(id.startsWith("flowy://tool/")){
        const toolInfo = await this.getToolInfo(createFlowyToolURL(id));
        if(toolInfo){
          urlToContentMap.set(toolInfo.comefrom, toolInfo.content);
        }
      }else if(id.startsWith("flowy://dataset/")){
        urlToDatasetIdtMap.set(url, createFlowyDatasetURL(id));
      }
    }    // load tool
    const loadingContext = new LoadingContext({});
    loadingContext.construct_tool_object = default_make_tool;
    const loadingOptions = new cwlTsAuto.LoadingOptions({});
    loadingContext.loadingOptions = loadingOptions;
    const fetcher = new ToolInfoFetcher(urlToContentMap);
    loadingOptions.fetcher = fetcher;
    const doc = await loadDocument(t.comefrom, loadingContext);
    const files = extractInstances(doc, cwlTsAuto.File);
    for(const file of files){
      const datasetid = urlToDatasetIdtMap.get(file.location);
      file['flowy_id'] = datasetid;
    }

    return await loadTool(doc,t.comefrom+toolUrl.getFragment(), loadingContext);
  }
}
const toolManager = new ToolManager();

export function getToolManager(): ToolManager {
  return toolManager;
}
