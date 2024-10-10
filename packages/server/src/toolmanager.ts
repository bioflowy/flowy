import { v4 } from "uuid";
import { type NewToolInfo, ToolInfo, db } from "./databases";
import * as cwlTsAuto from "@flowy/cwl-ts-auto";

import * as path from "path";
import * as fs from "fs";
import * as crypto from "crypto";
import { loadDocument } from "./loader";
import { default_make_tool } from "./workflow";
import { LoadingContext } from "./context";
import * as url from "node:url";
import { DefaultFetcher, Fetcher } from "@flowy/cwl-ts-auto/dist/util/Fetcher";
import { Process } from "./process";
import { createFlowyToolURL,FlowyToolURL } from "./flowyurl";
import { URL } from "url";

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
  getUrlToIdMap(): Map<string, FlowyToolURL> {
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
    const loadingContext = new LoadingContext({});
    loadingContext.construct_tool_object = default_make_tool;
    const loadingOptions = new cwlTsAuto.LoadingOptions({});
    loadingContext.loadingOptions = loadingOptions;
    const fetcher = new DefaultFetcher2(toolUrl.toString(), this);
    loadingOptions.fetcher = fetcher;
    loadingContext.baseuri = path.dirname(toolUrl.toString());
    try {
      await loadDocument(toolUrl.toString(), loadingContext);
    } catch (e) {
      console.log(e);
    }
    const idMap = fetcher.getUrlToIdMap();
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
    const urlToContentMap = new Map<string, string>();
    urlToContentMap.set(t.comefrom, t.content);
    for(const [url, id] of JSON.parse(t.references)){
      const toolInfo = await this.getToolInfo(createFlowyToolURL(id));
      if(toolInfo){
        urlToContentMap.set(toolInfo.comefrom, toolInfo.content);
      }
    }    // load tool
    const loadingContext = new LoadingContext({});
    loadingContext.construct_tool_object = default_make_tool;
    const loadingOptions = new cwlTsAuto.LoadingOptions({});
    loadingContext.loadingOptions = loadingOptions;
    const fetcher = new ToolInfoFetcher(urlToContentMap);
    loadingOptions.fetcher = fetcher;
    loadingContext.baseuri = path.dirname(t.comefrom);
    try {
      return await loadDocument(t.comefrom, loadingContext);
    } catch (e) {
      console.log(e);
    }
  }
}
const toolManager = new ToolManager();

export function getToolManager(): ToolManager {
  return toolManager;
}
