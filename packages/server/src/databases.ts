import {
    Insertable,
    JSONColumnType,
    PostgresDialect,
    Selectable,
    Updateable,
  } from 'kysely'
  import pg from 'pg'
  import { Kysely, SqliteDialect } from 'kysely'
import * as path from 'path'
import { JobStatus } from './utils'
  
export interface Database {
  tool: ToolTable
  job: JobTable
  job_output: JobOutputTable
}
export interface ToolTable {
  id: string
  name: string
  version: string
  hash: string
  comefrom: string
  references: string
  content: string
  created_at: Date
}
  
  export interface JobTable {
    id: string
    type: 'Workflow' | 'CommandLine' | 'Expression'
    status: JobStatus
    exitCode: number | undefined
    inputs: JSONColumnType<Record<string,any>>
    outputs: JSONColumnType<Record<string,any>>
    name: string
    parent_id: string
  }
  export interface JobOutputTable {
    id: string
    job_id: string
    type: string
    name: string
    value: JSONColumnType<any>
  }

export type Tool = Selectable<ToolTable>
export type NewTool = Insertable<ToolTable>

export type Job = Selectable<JobTable>
export type NewJob = Insertable<JobTable>
export type JobUpdate = Updateable<JobTable>

export type JobOutput = Selectable<JobOutputTable>
export type NewJobOutput = Insertable<JobTable>

const dialect = new PostgresDialect({
  pool: new pg.Pool({
    host: 'localhost',
    database: 'flowy',
    user: 'flowy',
    password: 'flowy',
  }),
})
// Database interface is passed to Kysely's constructor, and from now on, Kysely 
// knows your database structure.
// Dialect is passed to Kysely's constructor, and from now on, Kysely knows how 
// to communicate with your database.
export const db = new Kysely<Database>({
  dialect,
  log(event) {
    if (event.level === "error") {
        console.error("Query failed : ", {
          durationMs: event.queryDurationMillis,
          error: event.error,
          sql: event.query.sql,
        });
    } else { // `'query'`
      console.log("Query executed : ", {
        durationMs: event.queryDurationMillis,
        sql: event.query.sql,
      });
    }
  }
})