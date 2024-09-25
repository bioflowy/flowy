import { Kysely, sql } from 'kysely'

export async function up(db: Kysely<any>): Promise<void> {
    await db.schema
    .createTable('job')
    .addColumn('id', 'text', (col) => col.primaryKey())
    .addColumn('name', 'text', (col) => col.notNull())
    .addColumn('type', 'varchar(20)')
    .addColumn('status', 'varchar(10)')
    .addColumn('inputs', 'text')
    .addColumn('outputs', 'text')
    .addColumn('exitCode', 'integer')
    .addColumn('parent_id', 'text')
    .execute()
}

export async function down(db: Kysely<any>): Promise<void> {
    await db.schema.dropTable("job").execute()
  // Migration code
}
