import RatingsPanel from "@/components/ratings-panel";
import WatchlistPanel from "@/components/watchlist-panel";
import { and, count, desc, eq, gte, sql } from "drizzle-orm";
import { auth } from "@/auth";
import type { RatingsSummary } from "@/components/ratings-panel";
import type { WatchlistPanelItem } from "@/components/watchlist-panel";
import { episodes, movies, ratings, shows, watchEvents, watchlistItems } from "@/db/schema";
import { getDb } from "@/lib/db";
import { getRatingsSummary } from "@/lib/ratings";

type Widget = {
  label: string;
  value: string;
};

type RecentActivityItem = {
  id: number;
  mediaType: "movie" | "episode" | "show";
  title: string;
  subtitle: string;
  watchedAt: string;
};

function buildFallbackWidgets(): Widget[] {
  return [
    { label: "Watching", value: "0 series" },
    { label: "Completed this month", value: "0 titles" },
    { label: "Avg rating", value: "N/A" },
    { label: "Watch streak", value: "Soon" },
  ];
}

async function getWidgetsForUser(appUserId: string): Promise<Widget[]> {
  try {
    const db = getDb();
    const monthStart = new Date();
    monthStart.setUTCDate(1);
    monthStart.setUTCHours(0, 0, 0, 0);

    const [watchingResult] = await db
      .select({ total: count() })
      .from(watchlistItems)
      .where(and(eq(watchlistItems.userId, appUserId), eq(watchlistItems.itemType, "show")));

    const [completedResult] = await db
      .select({ total: count() })
      .from(watchEvents)
      .where(and(eq(watchEvents.userId, appUserId), gte(watchEvents.watchedAt, monthStart)));

    const [ratingResult] = await db
      .select({ average: sql<number | null>`avg(${ratings.ratingValue})` })
      .from(ratings)
      .where(eq(ratings.userId, appUserId));

    const watching = watchingResult?.total ?? 0;
    const completed = completedResult?.total ?? 0;
    const average = ratingResult?.average;

    return [
      { label: "Watching", value: `${watching} series` },
      { label: "Completed this month", value: `${completed} titles` },
      { label: "Avg rating", value: typeof average === "number" ? `${average.toFixed(1)} / 10` : "N/A" },
      { label: "Watch streak", value: "Soon" },
    ];
  } catch {
    return buildFallbackWidgets();
  }
}

async function getInitialWatchlistItems(appUserId: string): Promise<WatchlistPanelItem[]> {
  try {
    const db = getDb();
    const rows = await db
      .select({
        id: watchlistItems.id,
        itemType: watchlistItems.itemType,
        movieId: watchlistItems.movieId,
        showId: watchlistItems.showId,
        listedAt: watchlistItems.listedAt,
        movieTitle: movies.title,
        showTitle: shows.title,
      })
      .from(watchlistItems)
      .leftJoin(movies, eq(watchlistItems.movieId, movies.id))
      .leftJoin(shows, eq(watchlistItems.showId, shows.id))
      .where(eq(watchlistItems.userId, appUserId));

    return rows.map((row) => ({
      id: row.id,
      itemType: row.itemType,
      movieId: row.movieId,
      showId: row.showId,
      listedAt: row.listedAt ? row.listedAt.toISOString() : null,
      title: row.itemType === "movie" ? row.movieTitle : row.showTitle,
      userRatingValue: null,
      averageRatingValue: null,
    }));
  } catch {
    return [];
  }
}

async function getRecentActivity(appUserId: string): Promise<RecentActivityItem[]> {
  try {
    const db = getDb();
    const rows = await db
      .select({
        id: watchEvents.id,
        mediaType: watchEvents.mediaType,
        watchedAt: watchEvents.watchedAt,
        movieTitle: movies.title,
        showTitle: shows.title,
        seasonNumber: episodes.seasonNumber,
        episodeNumber: episodes.episodeNumber,
        episodeTitle: episodes.title,
      })
      .from(watchEvents)
      .leftJoin(movies, eq(watchEvents.movieId, movies.id))
      .leftJoin(episodes, eq(watchEvents.episodeId, episodes.id))
      .leftJoin(shows, eq(episodes.showId, shows.id))
      .where(eq(watchEvents.userId, appUserId))
      .orderBy(desc(watchEvents.watchedAt))
      .limit(8);

    return rows.map((row) => {
      if (row.mediaType === "movie") {
        return {
          id: row.id,
          mediaType: "movie",
          title: row.movieTitle ?? "Untitled movie",
          subtitle: "Movie",
          watchedAt: row.watchedAt.toISOString(),
        };
      }

      const seasonNumber = row.seasonNumber ?? 0;
      const episodeNumber = row.episodeNumber ?? 0;
      const episodeCode = `S${String(seasonNumber).padStart(2, "0")}E${String(episodeNumber).padStart(2, "0")}`;

      return {
        id: row.id,
        mediaType: "episode",
        title: row.showTitle ?? "Untitled show",
        subtitle: `${episodeCode}${row.episodeTitle ? ` - ${row.episodeTitle}` : ""}`,
        watchedAt: row.watchedAt.toISOString(),
      };
    });
  } catch {
    return [];
  }
}

function buildEmptyRatingsSummary(): RatingsSummary {
  return {
    total: 0,
    average: null,
    distribution: Array.from({ length: 10 }, (_, index) => ({ rating: index + 1, count: 0 })),
  };
}

export default async function DashboardPage() {
  const session = await auth();
  const appUserId = session?.user?.appUserId;
  const widgets = appUserId ? await getWidgetsForUser(appUserId) : buildFallbackWidgets();
  const initialWatchlistItems = appUserId ? await getInitialWatchlistItems(appUserId) : [];
  const recentActivity = appUserId ? await getRecentActivity(appUserId) : [];
  const ratingsSummary = appUserId ? await getRatingsSummary(appUserId) : buildEmptyRatingsSummary();

  return (
    <section className="space-y-6">
      <header>
        <p className="text-xs uppercase tracking-[0.18em] text-amber-300">dashboard</p>
        <h1 className="mt-2 text-3xl font-bold text-white">Your tracking cockpit</h1>
      </header>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {widgets.map((widget) => (
          <article key={widget.label} className="card rounded-xl p-4">
            <p className="text-xs uppercase tracking-wide text-slate-400">{widget.label}</p>
            <p className="mt-2 text-2xl font-semibold text-white">{widget.value}</p>
          </article>
        ))}
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <WatchlistPanel initialItems={initialWatchlistItems} />
        <section className="card rounded-xl p-5">
          <h2 className="text-lg font-semibold text-white">Recent watched activity</h2>
          {recentActivity.length === 0 ? (
            <p className="mt-3 text-sm text-slate-300">Mark movies or show episodes as watched to populate activity.</p>
          ) : (
            <ul className="mt-3 space-y-2">
              {recentActivity.map((item) => (
                <li key={item.id} className="rounded-md border border-white/10 bg-slate-900/50 px-3 py-2">
                  <p className="text-sm font-medium text-white">{item.title}</p>
                  <p className="text-xs uppercase tracking-wide text-slate-400">{item.subtitle}</p>
                  <p className="mt-1 text-xs text-slate-400">{new Date(item.watchedAt).toLocaleString()}</p>
                </li>
              ))}
            </ul>
          )}
        </section>
      </div>

      <RatingsPanel initialSummary={ratingsSummary} />
    </section>
  );
}
