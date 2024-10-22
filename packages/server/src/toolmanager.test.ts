import { describe, test, expect, beforeAll, afterAll, beforeEach, afterEach } from 'vitest'
import { Kysely, sql, SqliteDialect  } from 'kysely'
import * as fs from 'fs'
import * as path from 'path'
import * as url from 'url'
import { v4 } from 'uuid'
import SQLite from 'better-sqlite3'
import { Database, db } from './databases'
import { ToolManager } from './toolmanager'
import { Workflow } from './workflow'
import { copyRecursively, deletePathRecursive } from './fileutils'
import * as cwl from '@flowy/cwl-ts-auto'
import { CommandLineTool } from './command_line_tool'
import { createFlowyToolURL,FlowyToolURL } from './flowyurl'
interface ReplaceOption {
  searchValue: string | RegExp;
  replaceValue: string;
}

function pathToUrl(filePath: string): URL {
  return url.pathToFileURL(path.join(process.cwd(),filePath))
}
// データベース接続の設定
describe('Database Tests', () => {
  
    // 各テストの前に実行
    beforeEach(async () => {
      // テーブルのデータをクリア
      await db.deleteFrom('tool').execute()
    })
    afterEach(async () => {
      deletePathRecursive('test_temp')
    })
    // test('import cwlfile', async () => {
    //     const toolmanager = new ToolManager()
    //     const tool = await toolmanager.importTool(pathToUrl('testres/workflow1/count-lines1-wf.cwl'))
    //     expect(tool.name).toEqual('count-lines1-wf.cwl')
    //     const proc = await toolmanager.loadTool(tool.id)

    //     expect(proc instanceof Workflow).toBeTruthy()
    // }, { timeout: 1000000 })
    test('import cwlfile with import', async () => {
      const toolmanager = new ToolManager()
      const tool = await toolmanager.importTool(pathToUrl('testres/bwa-mem-tool.cwl'))
      expect(tool.name).toEqual('bwa-mem-tool.cwl')
      const [proc,status] = await toolmanager.loadTool(createFlowyToolURL(tool.id))
      expect(proc instanceof CommandLineTool).toBeTruthy()
    }, { timeout: 1000000 })
    // test('import invalid cwlfile', async () => {
    //   const toolmanager = new ToolManager() 
    //   expect(async ()=>{h
    //     await toolmanager.importTool(pathToUrl('testres/error.cwl'))
    //   }).rejects.toThrow()
    // })
    // test('"When the same CWL file is imported twice, it returns same tool.', async () => {
    //   const toolmanager = new ToolManager()
    //   const tool1 = await toolmanager.importTool(pathToUrl('testres/workflow1/count-lines1-wf.cwl'))
    //   const tool2 = await toolmanager.importTool(pathToUrl('testres/workflow1/count-lines1-wf.cwl'))
    //   expect(tool2).toEqual(tool1)
    // }, { timeout: 1000000 })
    // test('"When updated CWL file is imported, it returns new tool.', async () => {
    //   const toolmanager = new ToolManager()
    //   const tool1 = await toolmanager.importTool(pathToUrl('testres/test.cwl'))
    //   const tool2 = await toolmanager.importTool(pathToUrl('testres/test_v2.cwl'),"test.cwl")

    //   expect(tool2.id).not.toBe(tool1.id)
    //   expect(tool2.created_at.getTime()).toBeGreaterThan(tool1.created_at.getTime())
    // })
    // test('"When a cwl file referenced within a cwl file is updated, it returns new tool.', async () => {
    //   await copyRecursively('testres/workflow1','test_temp')
    //   const toolmanager = new ToolManager()
    //   const tool1 = await toolmanager.importTool(pathToUrl('test_temp/count-lines1-wf.cwl'))
    //   fs.copyFileSync("test_temp/wc-tool_v2.cwl", "test_temp/wc-tool.cwl");
    //   const tool2 = await toolmanager.importTool(pathToUrl('test_temp/count-lines1-wf.cwl'),"test.cwl")

    //   expect(tool2.id).not.toBe(tool1.id)
    //   expect(tool2.created_at.getTime()).toBeGreaterThan(tool1.created_at.getTime())
    // }, { timeout: 1000000 })

  })