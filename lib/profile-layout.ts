import { eq } from "drizzle-orm";
import type { ProfilePageBlock } from "@/components/profile-page";
import { profileLayouts } from "@/db/schema";
import { getDb } from "@/lib/db";

export const PROFILE_BLOCK_IDS = ["about-grid", "featured-list", "favorites-list", "recently-watched", "most-watched", "comments"] as const;

type ProfileBlockId = (typeof PROFILE_BLOCK_IDS)[number];

type ProfileLayoutRecord = {
  blockOrder: ProfileBlockId[];
  hiddenBlockIds: ProfileBlockId[];
};

function sanitizeBlockIds(value: unknown): ProfileBlockId[] {
  if (!Array.isArray(value)) {
    return [];
  }

  const unique = new Set<ProfileBlockId>();

  for (const item of value) {
    if (typeof item === "string" && (PROFILE_BLOCK_IDS as readonly string[]).includes(item)) {
      unique.add(item as ProfileBlockId);
    }
  }

  return Array.from(unique);
}

function normalizeLayout(layout: { blockOrder: unknown; hiddenBlockIds: unknown }): ProfileLayoutRecord {
  const blockOrder = sanitizeBlockIds(layout.blockOrder);
  const hiddenBlockIds = sanitizeBlockIds(layout.hiddenBlockIds);

  return {
    blockOrder: blockOrder.length ? blockOrder : [...PROFILE_BLOCK_IDS],
    hiddenBlockIds,
  };
}

export async function getOrCreateProfileLayout(userId: string): Promise<ProfileLayoutRecord> {
  const db = getDb();
  const [existing] = await db
    .select({
      blockOrder: profileLayouts.blockOrder,
      hiddenBlockIds: profileLayouts.hiddenBlockIds,
    })
    .from(profileLayouts)
    .where(eq(profileLayouts.userId, userId))
    .limit(1);

  if (existing) {
    return normalizeLayout(existing);
  }

  const [created] = await db
    .insert(profileLayouts)
    .values({
      userId,
      blockOrder: [...PROFILE_BLOCK_IDS],
      hiddenBlockIds: [],
      updatedAt: new Date(),
    })
    .returning({
      blockOrder: profileLayouts.blockOrder,
      hiddenBlockIds: profileLayouts.hiddenBlockIds,
    });

  return normalizeLayout(created ?? { blockOrder: PROFILE_BLOCK_IDS, hiddenBlockIds: [] });
}

export async function updateProfileLayout(
  userId: string,
  input: {
    blockOrder?: unknown;
    hiddenBlockIds?: unknown;
  }
): Promise<ProfileLayoutRecord> {
  const current = await getOrCreateProfileLayout(userId);

  const next = normalizeLayout({
    blockOrder: input.blockOrder ?? current.blockOrder,
    hiddenBlockIds: input.hiddenBlockIds ?? current.hiddenBlockIds,
  });

  const db = getDb();
  const [updated] = await db
    .insert(profileLayouts)
    .values({
      userId,
      blockOrder: next.blockOrder,
      hiddenBlockIds: next.hiddenBlockIds,
      updatedAt: new Date(),
    })
    .onConflictDoUpdate({
      target: profileLayouts.userId,
      set: {
        blockOrder: next.blockOrder,
        hiddenBlockIds: next.hiddenBlockIds,
        updatedAt: new Date(),
      },
    })
    .returning({
      blockOrder: profileLayouts.blockOrder,
      hiddenBlockIds: profileLayouts.hiddenBlockIds,
    });

  return normalizeLayout(updated ?? next);
}

export function applyProfileLayout(blocks: ProfilePageBlock[], layout: ProfileLayoutRecord): ProfilePageBlock[] {
  const hidden = new Set(layout.hiddenBlockIds);
  const byId = new Map(blocks.map((block) => [block.id, block]));
  const output: ProfilePageBlock[] = [];

  for (const id of layout.blockOrder) {
    const block = byId.get(id);

    if (block && !hidden.has(id)) {
      output.push(block);
      byId.delete(id);
    }
  }

  for (const block of blocks) {
    if (byId.has(block.id) && !hidden.has(block.id as ProfileBlockId)) {
      output.push(block);
    }
  }

  return output;
}
