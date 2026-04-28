import { and, count, eq, sql } from "drizzle-orm";
import {
  appUsers,
  episodes,
  movies,
  ratings,
  shows,
  statsSnapshots,
  watchEvents,
  watchlistItems,
} from "@/db/schema";
import { getDb } from "@/lib/db";

export type StatsSnapshotResult = {
  userId: string;
  snapshotDay: string;
  watchedCount: number;
  moviesWatchedCount: number;
  episodesWatchedCount: number;
  watchlistCount: number;
  ratingsCount: number;
  averageRating: number | null;
  totalMinutesWatched: number | null;
};

function toSnapshotDay(value: Date = new Date()): string {
  const day = new Date(value);
  day.setUTCHours(0, 0, 0, 0);
  return day.toISOString().slice(0, 10);
}

export async function computeStatsSnapshotForUser(userId: string, snapshotDate?: Date): Promise<StatsSnapshotResult> {
  const db = getDb();
  const snapshotDay = toSnapshotDay(snapshotDate);

  const [watchTotals] = await db
    .select({ total: count() })
    .from(watchEvents)
    .where(eq(watchEvents.userId, userId));

  const [movieTotals] = await db
    .select({ total: count() })
    .from(watchEvents)
    .where(and(eq(watchEvents.userId, userId), eq(watchEvents.mediaType, "movie")));

  const [episodeTotals] = await db
    .select({ total: count() })
    .from(watchEvents)
    .where(and(eq(watchEvents.userId, userId), eq(watchEvents.mediaType, "episode")));

  const [watchlistTotals] = await db
    .select({ total: count() })
    .from(watchlistItems)
    .where(eq(watchlistItems.userId, userId));

  const [ratingsTotals] = await db
    .select({ total: count(), average: sql<number | null>`avg(${ratings.ratingValue})` })
    .from(ratings)
    .where(eq(ratings.userId, userId));

  const [movieMinutesRow] = await db
    .select({ total: sql<number | null>`sum(${movies.runtimeMinutes})` })
    .from(watchEvents)
    .leftJoin(movies, eq(watchEvents.movieId, movies.id))
    .where(and(eq(watchEvents.userId, userId), eq(watchEvents.mediaType, "movie")));

  const [episodeMinutesRow] = await db
    .select({ total: sql<number | null>`sum(${shows.runtimeMinutes})` })
    .from(watchEvents)
    .leftJoin(episodes, eq(watchEvents.episodeId, episodes.id))
    .leftJoin(shows, eq(episodes.showId, shows.id))
    .where(and(eq(watchEvents.userId, userId), eq(watchEvents.mediaType, "episode")));

  const movieMinutes = movieMinutesRow?.total ?? null;
  const episodeMinutes = episodeMinutesRow?.total ?? null;
  const totalMinutesWatched =
    movieMinutes === null && episodeMinutes === null ? null : (movieMinutes ?? 0) + (episodeMinutes ?? 0);

  const watchedCount = watchTotals?.total ?? 0;
  const moviesWatchedCount = movieTotals?.total ?? 0;
  const episodesWatchedCount = episodeTotals?.total ?? 0;
  const watchlistCount = watchlistTotals?.total ?? 0;
  const ratingsCount = ratingsTotals?.total ?? 0;
  const averageRatingValue: string | null =
    typeof ratingsTotals?.average === "number" ? ratingsTotals.average.toFixed(2) : null;

  await db
    .insert(statsSnapshots)
    .values({
      userId,
      snapshotDay,
      watchedCount,
      moviesWatchedCount,
      episodesWatchedCount,
      watchlistCount,
      ratingsCount,
      averageRating: averageRatingValue,
      totalMinutesWatched,
      payload: {
        source: "computed",
        generatedAt: new Date().toISOString(),
        minutes: {
          movies: movieMinutes,
          episodes: episodeMinutes,
        },
      },
    })
    .onConflictDoUpdate({
      target: [statsSnapshots.userId, statsSnapshots.snapshotDay],
      set: {
        watchedCount,
        moviesWatchedCount,
        episodesWatchedCount,
        watchlistCount,
        ratingsCount,
        averageRating: averageRatingValue,
        totalMinutesWatched,
        payload: {
          source: "computed",
          generatedAt: new Date().toISOString(),
          minutes: {
            movies: movieMinutes,
            episodes: episodeMinutes,
          },
        },
      },
    });

  return {
    userId,
    snapshotDay,
    watchedCount,
    moviesWatchedCount,
    episodesWatchedCount,
    watchlistCount,
    ratingsCount,
    averageRating: averageRatingValue ? Number.parseFloat(averageRatingValue) : null,
    totalMinutesWatched,
  };
}

export async function runStatsSnapshotJob({ userId, snapshotDate }: { userId?: string; snapshotDate?: Date } = {}) {
  const db = getDb();

  const userIds = userId
    ? [userId]
    : (await db.select({ id: appUsers.id }).from(appUsers)).map((row) => row.id);

  const results: StatsSnapshotResult[] = [];

  for (const id of userIds) {
    const result = await computeStatsSnapshotForUser(id, snapshotDate);
    results.push(result);
  }

  return results;
}
