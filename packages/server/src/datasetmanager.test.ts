import { describe, test, expect, beforeEach, afterEach } from 'vitest'
import * as fs from 'fs'
import * as path from 'path'
import * as url from 'url'
import { db } from './databases'
import { deletePathRecursive } from './fileutils'
import { DatasetManager } from './datasetmanager'

function pathToUrl(filePath: string): URL {
  return url.pathToFileURL(path.join(process.cwd(),filePath))
}
describe('DatasetManager test', () => {
  
    // 各テストの前に実行
    beforeEach(async () => {
      // テーブルのデータをクリア
      await db.deleteFrom('dataset').execute()
    })
    afterEach(async () => {
      deletePathRecursive('test_temp')
    })
    test('import file to dataset', async () => {
      const filePath = 'testres/schemadef-tool.cwl'
      const fileUrl = pathToUrl(filePath)
      const manager = new DatasetManager()
      const dataset1 = await manager.importDataset(fileUrl)
      const dataset2 = await manager.importDataset(fileUrl)
      expect(dataset1.id).toEqual(dataset2.id)
      const now = new Date();
      await fs.promises.utimes(filePath, now, now);
      const dataset3 = await manager.importDataset(fileUrl)
      expect(dataset2.id).not.toEqual(dataset3.id)
    }, { timeout: 1000000 })

  })