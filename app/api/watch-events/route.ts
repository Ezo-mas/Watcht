import { and, eq, sql } from "drizzle-orm";
import { NextResponse } from "next/server";
import { z } from "zod";
import { auth } from "@/auth";
import { episodes, progressEpisodes, progressMovies, progressShows, watchEvents, watchlistItems } from "@/db/schema";
import { getDb } from "@/lib/db";

export const runtime = "nodejs";

const markWatchedSchema = z.object({
  watchlistItemId: z.number().int().positive(),
  watchedAt: z.string().datetime().optional(),
  removeFromWatchlist: z.boolean().default(false),
  seasonNumber: z.number().int().positive().optional(),
  episodeNumber: z.number().int().positive().optional(),
  episodeTitle: z.string().trim().min(1).max(240).optional(),
});

async function requireAppUserId() {
  const session = await auth();
  const appUserId = session?.user?.appUserId;

  if (!appUserId) {
    return null;
  }

  return appUserId;
}

export async function POST(request: Request) {
  const db = getDb();
  const appUserId = await requireAppUserId();

  if (!appUserId) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const body = await request.json().catch(() => null);
  const parsed = markWatchedSchema.safeParse(body);

  if (!parsed.success) {
    return NextResponse.json({ error: "Invalid payload", details: parsed.error.flatten() }, { status: 400 });
  }

  const [watchlistItem] = await db
    .select({
      id: watchlistItems.id,
      itemType: watchlistItems.itemType,
      movieId: watchlistItems.movieId,
      showId: watchlistItems.showId,
    })
    .from(watchlistItems)
    .where(and(eq(watchlistItems.userId, appUserId), eq(watchlistItems.id, parsed.data.watchlistItemId)))
    .limit(1);

  if (!watchlistItem) {
    return NextResponse.json({ error: "Watchlist item not found" }, { status: 404 });
  }

  const watchedAt = parsed.data.watchedAt ? new Date(parsed.data.watchedAt) : new Date();

  if (watchlistItem.itemType === "movie") {
    if (!watchlistItem.movieId) {
      return NextResponse.json({ error: "Movie id is missing for watchlist item" }, { status: 400 });
    }

    await db.insert(watchEvents).values({
      userId: appUserId,
      mediaType: "movie",
      movieId: watchlistItem.movieId,
      watchedAt,
    });

    await db
      .insert(progressMovies)
      .values({
        userId: appUserId,
        movieId: watchlistItem.movieId,
        plays: 1,
        totalCount: 1,
        lastWatchedAt: watchedAt,
        lastUpdatedAt: new Date(),
        updatedAt: new Date(),
      })
      .onConflictDoUpdate({
        target: [progressMovies.userId, progressMovies.movieId],
        set: {
          plays: sql`${progressMovies.plays} + 1`,
          totalCount: sql`coalesce(${progressMovies.totalCount}, 0) + 1`,
          lastWatchedAt: watchedAt,
          lastUpdatedAt: new Date(),
          updatedAt: new Date(),
        },
      });

    if (parsed.data.removeFromWatchlist) {
      await db.delete(watchlistItems).where(and(eq(watchlistItems.userId, appUserId), eq(watchlistItems.id, watchlistItem.id)));
    }

    return NextResponse.json({
      ok: true,
      mediaType: "movie",
      watchlistItemId: watchlistItem.id,
      watchedAt,
      removedFromWatchlist: parsed.data.removeFromWatchlist,
    });
  }

  if (!watchlistItem.showId) {
    return NextResponse.json({ error: "Show id is missing for watchlist item" }, { status: 400 });
  }

  if (!parsed.data.seasonNumber || !parsed.data.episodeNumber) {
    return NextResponse.json(
      {
        error: "seasonNumber and episodeNumber are required when marking a show as watched",
      },
      { status: 400 }
    );
  }

  const [episode] = await db
    .insert(episodes)
    .values({
      showId: watchlistItem.showId,
      seasonNumber: parsed.data.seasonNumber,
      episodeNumber: parsed.data.episodeNumber,
      title: parsed.data.episodeTitle ?? null,
    })
    .onConflictDoUpdate({
      target: [episodes.showId, episodes.seasonNumber, episodes.episodeNumber],
      set: {
        title: sql`coalesce(excluded.title, episodes.title)`,
        updatedAt: new Date(),
      },
    })
    .returning({
      id: episodes.id,
    });

  if (!episode) {
    return NextResponse.json({ error: "Unable to resolve episode" }, { status: 500 });
  }

  await db.insert(watchEvents).values({
    userId: appUserId,
    mediaType: "episode",
    episodeId: episode.id,
    watchedAt,
  });

  await db
    .insert(progressShows)
    .values({
      userId: appUserId,
      showId: watchlistItem.showId,
      plays: 1,
      lastWatchedAt: watchedAt,
      lastUpdatedAt: new Date(),
      updatedAt: new Date(),
    })
    .onConflictDoUpdate({
      target: [progressShows.userId, progressShows.showId],
      set: {
        plays: sql`${progressShows.plays} + 1`,
        lastWatchedAt: watchedAt,
        lastUpdatedAt: new Date(),
        updatedAt: new Date(),
      },
    });

  await db
    .insert(progressEpisodes)
    .values({
      userId: appUserId,
      episodeId: episode.id,
      plays: 1,
      lastWatchedAt: watchedAt,
      updatedAt: new Date(),
    })
    .onConflictDoUpdate({
      target: [progressEpisodes.userId, progressEpisodes.episodeId],
      set: {
        plays: sql`${progressEpisodes.plays} + 1`,
        lastWatchedAt: watchedAt,
        updatedAt: new Date(),
      },
    });

  if (parsed.data.removeFromWatchlist) {
    await db.delete(watchlistItems).where(and(eq(watchlistItems.userId, appUserId), eq(watchlistItems.id, watchlistItem.id)));
  }

  return NextResponse.json({
    ok: true,
    mediaType: "episode",
    episodeId: episode.id,
    watchlistItemId: watchlistItem.id,
    watchedAt,
    removedFromWatchlist: parsed.data.removeFromWatchlist,
  });
}