import { sql } from "drizzle-orm";
import { appUsers } from "@/db/schema";
import { getDb } from "@/lib/db";

const HANDLE_MAX_LENGTH = 64;

function buildOAuthHandle(provider: string, providerAccountId: string): string {
  const normalized = `${provider}_${providerAccountId}`.toLowerCase().replace(/[^a-z0-9_]/g, "_");
  return normalized.slice(0, HANDLE_MAX_LENGTH);
}

type OAuthIdentityInput = {
  provider: string;
  providerAccountId: string;
  displayName?: string | null;
};

export async function upsertAppUserFromOAuthIdentity(input: OAuthIdentityInput): Promise<{ id: string; handle: string }> {
  const db = getDb();
  const handle = buildOAuthHandle(input.provider, input.providerAccountId);
  const [result] = await db
    .insert(appUsers)
    .values({
      handle,
      displayName: input.displayName ?? null,
    })
    .onConflictDoUpdate({
      target: appUsers.handle,
      set: {
        displayName: sql`coalesce(excluded.display_name, app_users.display_name)`,
        updatedAt: sql`now()`,
      },
    })
    .returning({
      id: appUsers.id,
      handle: appUsers.handle,
    });

  if (!result) {
    throw new Error("Failed to resolve app user identity.");
  }

  return result;
}