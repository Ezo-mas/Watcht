import { eq } from "drizzle-orm";
import { dashboardLayouts } from "@/db/schema";
import { getDb } from "@/lib/db";

export const DASHBOARD_BLOCK_IDS = [
  "month-review",
  "up-next",
  "watchlist",
  "recently-watched",
  "last-30-days",
  "recommendations",
  "social-feed",
  "ratings",
] as const;

type DashboardBlockId = (typeof DASHBOARD_BLOCK_IDS)[number];

type DashboardLayoutRecord = {
  blockOrder: DashboardBlockId[];
  hiddenBlockIds: DashboardBlockId[];
};

function sanitizeBlockIds(value: unknown): DashboardBlockId[] {
  if (!Array.isArray(value)) {
    return [];
  }

  const unique = new Set<DashboardBlockId>();

  for (const item of value) {
    if (typeof item === "string" && (DASHBOARD_BLOCK_IDS as readonly string[]).includes(item)) {
      unique.add(item as DashboardBlockId);
    }
  }

  return Array.from(unique);
}

function normalizeLayout(layout: { blockOrder: unknown; hiddenBlockIds: unknown }): DashboardLayoutRecord {
  const blockOrder = sanitizeBlockIds(layout.blockOrder);
  const hiddenBlockIds = sanitizeBlockIds(layout.hiddenBlockIds);

  return {
    blockOrder: blockOrder.length ? blockOrder : [...DASHBOARD_BLOCK_IDS],
    hiddenBlockIds,
  };
}

export async function getOrCreateDashboardLayout(userId: string): Promise<DashboardLayoutRecord> {
  const db = getDb();
  const [existing] = await db
    .select({
      blockOrder: dashboardLayouts.blockOrder,
      hiddenBlockIds: dashboardLayouts.hiddenBlockIds,
    })
    .from(dashboardLayouts)
    .where(eq(dashboardLayouts.userId, userId))
    .limit(1);

  if (existing) {
    return normalizeLayout(existing);
  }

  const [created] = await db
    .insert(dashboardLayouts)
    .values({
      userId,
      blockOrder: [...DASHBOARD_BLOCK_IDS],
      hiddenBlockIds: [],
      updatedAt: new Date(),
    })
    .returning({
      blockOrder: dashboardLayouts.blockOrder,
      hiddenBlockIds: dashboardLayouts.hiddenBlockIds,
    });

  return normalizeLayout(created ?? { blockOrder: DASHBOARD_BLOCK_IDS, hiddenBlockIds: [] });
}

export async function updateDashboardLayout(
  userId: string,
  input: {
    blockOrder?: unknown;
    hiddenBlockIds?: unknown;
  }
): Promise<DashboardLayoutRecord> {
  const current = await getOrCreateDashboardLayout(userId);

  const next = normalizeLayout({
    blockOrder: input.blockOrder ?? current.blockOrder,
    hiddenBlockIds: input.hiddenBlockIds ?? current.hiddenBlockIds,
  });

  const db = getDb();
  const [updated] = await db
    .insert(dashboardLayouts)
    .values({
      userId,
      blockOrder: next.blockOrder,
      hiddenBlockIds: next.hiddenBlockIds,
      updatedAt: new Date(),
    })
    .onConflictDoUpdate({
      target: dashboardLayouts.userId,
      set: {
        blockOrder: next.blockOrder,
        hiddenBlockIds: next.hiddenBlockIds,
        updatedAt: new Date(),
      },
    })
    .returning({
      blockOrder: dashboardLayouts.blockOrder,
      hiddenBlockIds: dashboardLayouts.hiddenBlockIds,
    });

  return normalizeLayout(updated ?? next);
}

export function orderDashboardBlockIds(layout: DashboardLayoutRecord): DashboardBlockId[] {
  const hidden = new Set(layout.hiddenBlockIds);
  const ordered = layout.blockOrder.filter((id) => !hidden.has(id));

  for (const id of DASHBOARD_BLOCK_IDS) {
    if (!ordered.includes(id) && !hidden.has(id)) {
      ordered.push(id);
    }
  }

  return ordered;
}
