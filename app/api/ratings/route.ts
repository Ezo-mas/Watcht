import { NextResponse } from "next/server";
import { z } from "zod";
import { auth } from "@/auth";
import {
  getRatingsSummary,
  resolveMovieId,
  resolveShowId,
  upsertMovieRating,
  upsertShowRating,
} from "@/lib/ratings";

export const runtime = "nodejs";

const ratingSchema = z.object({
  itemType: z.enum(["movie", "show"]),
  ratingValue: z.number().int().min(1).max(10),
  movieId: z.number().int().positive().optional(),
  showId: z.number().int().positive().optional(),
  tmdbId: z.number().int().positive().optional(),
  title: z.string().trim().min(1).max(240).optional(),
  releaseYear: z.number().int().min(1878).max(3000).optional(),
});

async function requireAppUserId() {
  const session = await auth();
  return session?.user?.appUserId ?? null;
}

export async function GET() {
  const appUserId = await requireAppUserId();

  if (!appUserId) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const summary = await getRatingsSummary(appUserId);
  return NextResponse.json(summary);
}

export async function POST(request: Request) {
  const appUserId = await requireAppUserId();

  if (!appUserId) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const body = await request.json().catch(() => null);
  const parsed = ratingSchema.safeParse(body);

  if (!parsed.success) {
    return NextResponse.json({ error: "Invalid payload", details: parsed.error.flatten() }, { status: 400 });
  }

  const { itemType, ratingValue } = parsed.data;

  if (itemType === "movie") {
    const movieId = await resolveMovieId({
      movieId: parsed.data.movieId,
      tmdbId: parsed.data.tmdbId,
      title: parsed.data.title,
      releaseYear: parsed.data.releaseYear,
    });

    if (!movieId) {
      return NextResponse.json({ error: "movieId, tmdbId, or title is required for movie ratings" }, { status: 400 });
    }

    const result = await upsertMovieRating({ userId: appUserId, movieId, ratingValue });
    return NextResponse.json({ ok: true, id: result.id, updated: result.updated });
  }

  const showId = await resolveShowId({
    showId: parsed.data.showId,
    tmdbId: parsed.data.tmdbId,
    title: parsed.data.title,
    releaseYear: parsed.data.releaseYear,
  });

  if (!showId) {
    return NextResponse.json({ error: "showId, tmdbId, or title is required for show ratings" }, { status: 400 });
  }

  const result = await upsertShowRating({ userId: appUserId, showId, ratingValue });
  return NextResponse.json({ ok: true, id: result.id, updated: result.updated });
}
