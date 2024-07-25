import * as path from 'path'
import SQLite from 'better-sqlite3'
import { promises as fs } from 'fs'
import {
  Kysely,
  Migrator,
  PostgresDialect,
  FileMigrationProvider,
  SqliteDialect,
} from 'kysely'
import { Database } from './databases'

async function migrateToLatest() {
  const db = new Kysely<Database>({
    dialect: new SqliteDialect({database: new SQLite("flowy.db")}) 
  })
  const migratePath =path.join(process.cwd(),'src','migrations')
  console.log(`migratePath=${migratePath}`)
  const migrator = new Migrator({
    
    db,
    provider: new FileMigrationProvider({
      fs,
      path,
      // This needs to be an absolute path.
      migrationFolder: migratePath,
    }),
  })

  const { error, results } = await migrator.migrateToLatest()

  results?.forEach((it) => {
    if (it.status === 'Success') {
      console.log(`migration "${it.migrationName}" was executed successfully`)
    } else if (it.status === 'Error') {
      console.error(`failed to execute migration "${it.migrationName}"`)
    }
  })

  if (error) {
    console.error('failed to migrate')
    console.error(error)
    process.exit(1)
  }

  await db.destroy()
}

migrateToLatest()