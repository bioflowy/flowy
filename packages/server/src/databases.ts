import {
    ColumnType,
    Generated,
    Insertable,
    JSONColumnType,
    Selectable,
    Updateable,
  } from 'kysely'
  import SQLite from 'better-sqlite3'
  import { Kysely, SqliteDialect } from 'kysely'
import * as path from 'path'
  
  export interface Database {
    job: JobTable
  }
  
  // This interface describes the `person` table to Kysely. Table
  // interfaces should only be used in the `Database` type above
  // and never as a result type of a query!. See the `Person`,
  // `NewPerson` and `PersonUpdate` types below.
  export interface JobTable {
    id: string
    type: 'Workflow' | 'CommandLine' | 'Expression'
    status: 'Created' | 'Started' | 'Finished' | 'Failed'
    exitCode: number | undefined
    inputs: JSONColumnType<Record<string,any>>
    outputs: JSONColumnType<Record<string,any>>
    name: string
    parent_id: string
  }

export type Job = Selectable<JobTable>
export type NewJob = Insertable<JobTable>
export type JobUpdate = Updateable<JobTable>

const dbPath = path.join(process.cwd(),'flowy.db')
console.log(`dbPath=${dbPath}`)
const dialect = new SqliteDialect({
  database: new SQLite(dbPath),
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