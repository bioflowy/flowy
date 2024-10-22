import { v4 } from "uuid";
import * as fs from "node:fs";
import * as crypto from "node:crypto";
import { type NewToolInfo, Dataset, db } from "./databases";
import { createFlowyToolURL,FlowyDatasetURL,FlowyToolURL } from "./flowyurl";
import { URL,fileURLToPath } from "url";
interface FileInfo {
  size: number;
  created: Date;
  modified: Date;
  sha1: string;
}

async function getFileInfo(filePath: string): Promise<FileInfo> {
  return new Promise((resolve, reject) => {
    fs.stat(filePath, (err, stats) => {
      if (err) {
        reject(err);
        return;
      }

      const fileStream = fs.createReadStream(filePath);
      const hash = crypto.createHash('sha1');

      fileStream.on('data', (data:Buffer) => {
        hash.update(data);
      });

      fileStream.on('end', () => {
        const sha1 = hash.digest('hex');

        resolve({
          size: stats.size,
          created: stats.birthtime,
          modified: stats.mtime,
          sha1: sha1
        });
      });

      fileStream.on('error', (err) => {
        reject(err);
      });
    });
  });
}
export class DatasetManager {
  async getDataset(datasetUrl: FlowyDatasetURL): Promise<Dataset> {
    return await db
      .selectFrom("dataset")
      .selectAll()
      .where("id", "=", datasetUrl.getId())
      .executeTakeFirst();
  }
  async getDatasets(fileURL: URL): Promise<Dataset[]> {
    return await db
      .selectFrom("dataset")
      .selectAll()
      .where("location", "=", fileURL.toString())
      .execute();
  }
  async importDataset(fileURL: URL): Promise<Dataset> {
    const filePath = fileURLToPath(fileURL)
    
    const fileInfo = await getFileInfo(filePath);
    const datasets = await this.getDatasets(fileURL);
    for(const dataset of datasets){
      if(dataset.size === fileInfo.size && dataset.modified_at.getTime() === fileInfo.modified.getTime() && 
        dataset.checksum === fileInfo.sha1){
        return dataset;
      }
    }
    const dataset = {
      id: v4(),
      name: fileURL.pathname,
      location: fileURL.toString(),
      checksum: fileInfo.sha1,
      size: fileInfo.size,
      created_at: fileInfo.created,
      modified_at: fileInfo.modified,
      type: "File",
    };
    await db.insertInto("dataset").values(dataset as NewToolInfo).execute();
    return dataset;
  }
}
const manager = new DatasetManager();

export function getDatasetManager(): DatasetManager {
  return manager;
}
