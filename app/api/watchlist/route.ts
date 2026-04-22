import { and, desc, eq } from "drizzle-orm";
import { NextResponse } from "next/server";
import { z } from "zod";
import { auth } from "@/auth";
import { movies, shows, watchlistItems } from "@/db/schema";
import { getDb } from "@/lib/db";

export const runtime = "nodejs";

const addWatchlistSchema = z.union([
  z.object({
    itemType: z.literal("movie"),
    movieId: z.number().int().positive().optional(),
    title: z.string().trim().min(1).max(240).optional(),
    releaseYear: z.number().int().min(1878).max(3000).optional(),
  }),
  z.object({
    itemType: z.literal("show"),
    showId: z.number().int().positive().optional(),
    title: z.string().trim().min(1).max(240).optional(),
    releaseYear: z.number().int().min(1878).max(3000).optional(),
  }),
]);

const deleteWatchlistSchema = z.union([
  z.object({ id: z.number().int().positive() }),
  z.object({
    itemType: z.literal("movie"),
    movieId: z.number().int().positive(),
  }),
  z.object({
    itemType: z.literal("show"),
    showId: z.number().int().positive(),
  }),
]);

async function requireAppUserId() {
  const session = await auth();
  const appUserId = session?.user?.appUserId;

  if (!appUserId) {
    return null;
  }

  return appUserId;
}

export async function GET() {
  const db = getDb();
  const appUserId = await requireAppUserId();

  if (!appUserId) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const rows = await db
    .select({
      id: watchlistItems.id,
      itemType: watchlistItems.itemType,
      movieId: watchlistItems.movieId,
      showId: watchlistItems.showId,
      listedAt: watchlistItems.listedAt,
      createdAt: watchlistItems.createdAt,
      movieTitle: movies.title,
      showTitle: shows.title,
    })
    .from(watchlistItems)
    .leftJoin(movies, eq(watchlistItems.movieId, movies.id))
    .leftJoin(shows, eq(watchlistItems.showId, shows.id))
    .where(eq(watchlistItems.userId, appUserId))
    .orderBy(desc(watchlistItems.createdAt));

  const items = rows.map((row) => ({
    id: row.id,
    itemType: row.itemType,
    movieId: row.movieId,
    showId: row.showId,
    listedAt: row.listedAt,
    title: row.itemType === "movie" ? row.movieTitle : row.showTitle,
  }));

  return NextResponse.json({ items });
}

export async function POST(request: Request) {
  const db = getDb();
  const appUserId = await requireAppUserId();

  if (!appUserId) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const body = await request.json().catch(() => null);
  const parsed = addWatchlistSchema.safeParse(body);

  if (!parsed.success) {
    return NextResponse.json({ error: "Invalid payload", details: parsed.error.flatten() }, { status: 400 });
  }

  if (parsed.data.itemType === "movie" && !parsed.data.movieId && !parsed.data.title) {
    return NextResponse.json({ error: "movieId or title is required for movie items" }, { status: 400 });
  }

  if (parsed.data.itemType === "show" && !parsed.data.showId && !parsed.data.title) {
    return NextResponse.json({ error: "showId or title is required for show items" }, { status: 400 });
  }

  let movieId = parsed.data.itemType === "movie" ? (parsed.data.movieId ?? null) : null;
  let showId = parsed.data.itemType === "show" ? (parsed.data.showId ?? null) : null;

  if (parsed.data.itemType === "movie" && !movieId && parsed.data.title) {
    const insertedMovie = await db
      .insert(movies)
      .values({
        title: parsed.data.title,
        releaseYear: parsed.data.releaseYear ?? null,
      })
      .returning({ id: movies.id });

    movieId = insertedMovie[0]?.id ?? null;
  }

  if (parsed.data.itemType === "show" && !showId && parsed.data.title) {
    const insertedShow = await db
      .insert(shows)
      .values({
        title: parsed.data.title,
        releaseYear: parsed.data.releaseYear ?? null,
      })
      .returning({ id: shows.id });

    showId = insertedShow[0]?.id ?? null;
  }

  if (parsed.data.itemType === "movie" && !movieId) {
    return NextResponse.json({ error: "Unable to resolve movie id" }, { status: 400 });
  }

  if (parsed.data.itemType === "show" && !showId) {
    return NextResponse.json({ error: "Unable to resolve show id" }, { status: 400 });
  }

  const values =
    parsed.data.itemType === "movie"
      ? {
          userId: appUserId,
          itemType: parsed.data.itemType,
          movieId,
          listedAt: new Date(),
        }
      : {
          userId: appUserId,
          itemType: parsed.data.itemType,
          showId,
          listedAt: new Date(),
        };

  const inserted = await db.insert(watchlistItems).values(values).onConflictDoNothing().returning({ id: watchlistItems.id });

  if (inserted.length === 0) {
    return NextResponse.json({ ok: true, inserted: false, reason: "Already exists" }, { status: 200 });
  }

  return NextResponse.json({ ok: true, inserted: true, id: inserted[0].id }, { status: 201 });
}

export async function DELETE(request: Request) {
  const db = getDb();
  const appUserId = await requireAppUserId();

  if (!appUserId) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const body = await request.json().catch(() => null);
  const parsed = deleteWatchlistSchema.safeParse(body);

  if (!parsed.success) {
    return NextResponse.json({ error: "Invalid payload", details: parsed.error.flatten() }, { status: 400 });
  }

  const whereClause =
    "id" in parsed.data
      ? and(eq(watchlistItems.userId, appUserId), eq(watchlistItems.id, parsed.data.id))
      : parsed.data.itemType === "movie"
        ? and(eq(watchlistItems.userId, appUserId), eq(watchlistItems.itemType, "movie"), eq(watchlistItems.movieId, parsed.data.movieId))
        : and(eq(watchlistItems.userId, appUserId), eq(watchlistItems.itemType, "show"), eq(watchlistItems.showId, parsed.data.showId));

  const removed = await db.delete(watchlistItems).where(whereClause).returning({ id: watchlistItems.id });

  if (removed.length === 0) {
    return NextResponse.json({ ok: false, error: "Not found" }, { status: 404 });
  }

  return NextResponse.json({ ok: true, removedId: removed[0].id });
}