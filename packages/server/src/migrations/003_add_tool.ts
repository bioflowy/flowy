import { Kysely, sql } from 'kysely'

export async function up(db: Kysely<any>): Promise<void> {
    await db.schema
    .createTable('tool')
    .addColumn('id', 'text', (col) => col.primaryKey())
    .addColumn('name', 'text', (col) => col.notNull())
    .addColumn('version', 'text')
    .addColumn('hash', 'text', (col) => col.notNull())
    .addColumn('created_at', 'timestamp', (col) => col.notNull())
    .addColumn('comefrom', 'text')
    .addColumn('references', 'text')
    .addColumn('content', 'text')
    .execute()
}

export async function down(db: Kysely<any>): Promise<void> {
    await db.schema.dropTable("tool").execute()
  // Migration code
}
