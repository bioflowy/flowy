import { Kysely, sql } from 'kysely'

export async function up(db: Kysely<any>): Promise<void> {
    await db.schema
    .createTable('dataset')
    .addColumn('id', 'uuid', (col) => col.primaryKey())
    .addColumn('name', 'text', (col) => col.notNull())
    .addColumn('location', 'text', (col) => col.notNull())
    .addColumn('checksum', 'text')
    .addColumn('size', 'integer')
    .addColumn('created_at', 'timestamp', (col) => col.notNull())
    .addColumn('modified_at', 'timestamp', (col) => col.notNull())
    .addColumn('type', 'text')
    .execute()
}

export async function down(db: Kysely<any>): Promise<void> {
    await db.schema.dropTable("dataset").execute()
  // Migration code
}
