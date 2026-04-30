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
  location: string | null;
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

  // If the caller passed a development-local id (e.g. local-... possibly
  // repeated due to layered fallbacks), resolve it to the DB `handle` that
  // the credentials provider writes (e.g. `credentials_local_dev`). This
  // avoids passing a non-UUID into a query against the `id` PK column.
  if (appUserId.startsWith("local-")) {
    // Strip repeated `local-` prefixes and use the remaining token as the
    // providerAccountId (e.g. `local-dev`). Then compute the handle.
    const providerAccountId = appUserId.replace(/^local-+/, "");
    const handle = buildOAuthHandle("credentials", providerAccountId);

    const [byHandle] = await db
      .select({
        id: appUsers.id,
        handle: appUsers.handle,
        displayName: appUsers.displayName,
        isPrivate: appUsers.isPrivate,
        joinedAt: appUsers.joinedAt,
        location: appUsers.location,
      })
      .from(appUsers)
      .where(eq(appUsers.handle, handle))
      .limit(1);

    if (byHandle) return byHandle;
    return null;
  }
  const [result] = await db
    .select({
      id: appUsers.id,
      handle: appUsers.handle,
      displayName: appUsers.displayName,
      isPrivate: appUsers.isPrivate,
      joinedAt: appUsers.joinedAt,
      location: appUsers.location,
    })
    .from(appUsers)
    .where(eq(appUsers.id, appUserId))
    .limit(1);

  return result ?? null;
}