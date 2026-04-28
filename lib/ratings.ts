import { and, count, eq, sql } from "drizzle-orm";
import { movies, ratings, shows } from "@/db/schema";
import { getDb } from "@/lib/db";

export type RatingsSummary = {
  total: number;
  average: number | null;
  distribution: Array<{ rating: number; count: number }>;
};

export async function getRatingsSummary(userId: string): Promise<RatingsSummary> {
  const db = getDb();

  const [summaryRow] = await db
    .select({ total: count(), average: sql<number | null>`avg(${ratings.ratingValue})` })
    .from(ratings)
    .where(eq(ratings.userId, userId));

  const distributionRows = await db
    .select({ ratingValue: ratings.ratingValue, total: count() })
    .from(ratings)
    .where(eq(ratings.userId, userId))
    .groupBy(ratings.ratingValue)
    .orderBy(ratings.ratingValue);

  const distributionMap = new Map<number, number>();
  for (const row of distributionRows) {
    distributionMap.set(row.ratingValue, row.total);
  }

  const distribution = Array.from({ length: 10 }, (_, index) => {
    const rating = index + 1;
    return { rating, count: distributionMap.get(rating) ?? 0 };
  });

  return {
    total: summaryRow?.total ?? 0,
    average: typeof summaryRow?.average === "number" ? summaryRow.average : null,
    distribution,
  };
}

export async function resolveMovieId({
  movieId,
  tmdbId,
  title,
  releaseYear,
}: {
  movieId?: number;
  tmdbId?: number;
  title?: string;
  releaseYear?: number;
}) {
  const db = getDb();

  if (movieId) {
    return movieId;
  }

  if (tmdbId) {
    const [existingMovie] = await db
      .select({ id: movies.id })
      .from(movies)
      .where(eq(movies.tmdbId, tmdbId))
      .limit(1);

    if (existingMovie?.id) {
      return existingMovie.id;
    }
  }

  if (!title) {
    return null;
  }

  const [insertedMovie] = await db
    .insert(movies)
    .values({
      title,
      releaseYear: releaseYear ?? null,
      tmdbId: tmdbId ?? null,
    })
    .returning({ id: movies.id });

  return insertedMovie?.id ?? null;
}

export async function resolveShowId({
  showId,
  tmdbId,
  title,
  releaseYear,
}: {
  showId?: number;
  tmdbId?: number;
  title?: string;
  releaseYear?: number;
}) {
  const db = getDb();

  if (showId) {
    return showId;
  }

  if (tmdbId) {
    const [existingShow] = await db
      .select({ id: shows.id })
      .from(shows)
      .where(eq(shows.tmdbId, tmdbId))
      .limit(1);

    if (existingShow?.id) {
      return existingShow.id;
    }
  }

  if (!title) {
    return null;
  }

  const [insertedShow] = await db
    .insert(shows)
    .values({
      title,
      releaseYear: releaseYear ?? null,
      tmdbId: tmdbId ?? null,
    })
    .returning({ id: shows.id });

  return insertedShow?.id ?? null;
}

export async function upsertMovieRating({
  userId,
  movieId,
  ratingValue,
}: {
  userId: string;
  movieId: number;
  ratingValue: number;
}) {
  const db = getDb();
  const [existingRating] = await db
    .select({ id: ratings.id })
    .from(ratings)
    .where(and(eq(ratings.userId, userId), eq(ratings.movieId, movieId), eq(ratings.mediaType, "movie")))
    .limit(1);

  if (existingRating?.id) {
    await db
      .update(ratings)
      .set({ ratingValue, ratedAt: new Date(), updatedAt: new Date() })
      .where(eq(ratings.id, existingRating.id));

    return { id: existingRating.id, updated: true };
  }

  const [inserted] = await db
    .insert(ratings)
    .values({
      userId,
      mediaType: "movie",
      movieId,
      ratingValue,
      ratedAt: new Date(),
      updatedAt: new Date(),
    })
    .returning({ id: ratings.id });

  return { id: inserted?.id ?? null, updated: false };
}

export async function upsertShowRating({
  userId,
  showId,
  ratingValue,
}: {
  userId: string;
  showId: number;
  ratingValue: number;
}) {
  const db = getDb();
  const [existingRating] = await db
    .select({ id: ratings.id })
    .from(ratings)
    .where(and(eq(ratings.userId, userId), eq(ratings.showId, showId), eq(ratings.mediaType, "show")))
    .limit(1);

  if (existingRating?.id) {
    await db
      .update(ratings)
      .set({ ratingValue, ratedAt: new Date(), updatedAt: new Date() })
      .where(eq(ratings.id, existingRating.id));

    return { id: existingRating.id, updated: true };
  }

  const [inserted] = await db
    .insert(ratings)
    .values({
      userId,
      mediaType: "show",
      showId,
      ratingValue,
      ratedAt: new Date(),
      updatedAt: new Date(),
    })
    .returning({ id: ratings.id });

  return { id: inserted?.id ?? null, updated: false };
}
