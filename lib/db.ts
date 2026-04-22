import { drizzle } from "drizzle-orm/node-postgres";
import { Pool } from "pg";

const globalForDb = globalThis as unknown as {
  pool?: Pool;
  db?: ReturnType<typeof drizzle>;
};

function ensureDb() {
  const connectionString = process.env.DATABASE_URL;

  if (!connectionString) {
    throw new Error("DATABASE_URL is required to initialize the database client.");
  }

  const pool = globalForDb.pool ?? new Pool({ connectionString });
  const db = globalForDb.db ?? drizzle(pool);

  if (process.env.NODE_ENV !== "production") {
    globalForDb.pool = pool;
    globalForDb.db = db;
  }

  return { db, pool };
}

export function getDb() {
  return ensureDb().db;
}

export function getPool() {
  return ensureDb().pool;
}