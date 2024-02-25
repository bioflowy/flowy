import { log } from 'node:console';
import { DefaultFetcher, Fetcher } from '@flowy/cwl-ts-auto/dist/util/Fetcher.js';
import { fetcher } from 'rdflib';
import { getFileContentFromS3 } from './builder.js';
import { getManager } from './server/manager.js';

export function dirnames3(path: string): string {
  let dirname = path.split('/').slice(0, -1).join('/');
  if (!dirname.endsWith('/')) {
    dirname = `${dirname}/`;
  }
  return dirname;
}
export type StringMap = { [key: string]: string };
export class S3Fetcher extends Fetcher {
  fetcher: DefaultFetcher = new DefaultFetcher();
  async fetchText(url: string, _?: string[]): Promise<string> {
    const config = getManager().getServerConfig();
    const content = await getFileContentFromS3(config.sharedFileSystem, url, true);
    return content;
  }
  checkExists(urlString: string): boolean {
    return this.fetcher.checkExists(urlString);
  }
  urljoin(baseUrlString: string, urlString: string): string {
    return this.fetcher.urljoin(baseUrlString, urlString);
  }
  static override schemes = ['s3'];
}
