import { eq, sql } from "drizzle-orm";
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

type AppUserSummary = {
  id: string;
  handle: string;
  displayName: string | null;
  isPrivate: boolean;
  joinedAt: Date | null;
};

export async function upsertAppUserFromOAuthIdentity(input: OAuthIdentityInput): Promise<{ id: string; handle: string }> {
  const db = getDb();
  const handle = buildOAuthHandle(input.provider, input.providerAccountId);
  const [result] = await db
    .insert(appUsers)
    .values({
      handle,
      displayName: input.displayName ?? null,
      isPrivate: true,
      joinedAt: new Date(),
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

export async function getAppUserSummaryById(appUserId: string): Promise<AppUserSummary | null> {
  const db = getDb();
  const [result] = await db
    .select({
      id: appUsers.id,
      handle: appUsers.handle,
      displayName: appUsers.displayName,
      isPrivate: appUsers.isPrivate,
      joinedAt: appUsers.joinedAt,
    })
    .from(appUsers)
    .where(eq(appUsers.id, appUserId))
    .limit(1);

  return result ?? null;
}