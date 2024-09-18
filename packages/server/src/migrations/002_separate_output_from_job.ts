import { Kysely, sql } from 'kysely'

export async function up(db: Kysely<any>): Promise<void> {
    await db.schema
    .createTable('job_output')
    .addColumn('id', 'text', (col) => col.primaryKey())
    .addColumn('job_id', 'text', (col) => col.references('job.id'))
    .addColumn('name', 'text', (col) => col.notNull())
    .addColumn('type', 'varchar(10)')
    .addColumn('value', 'text')
    .execute()
    await db.schema
    .alterTable('job')
    .dropColumn('outputs')
    .execute()
}

export async function down(db: Kysely<any>): Promise<void> {
    await db.schema.dropTable("job_output").execute()
    await db.schema.alterTable("job_output").addColumn('output', 'text').execute()
  // Migration code
}
