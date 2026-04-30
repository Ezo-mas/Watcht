import Image from "next/image";
import Link from "next/link";
import RatingsPanel from "@/components/ratings-panel";
import WatchlistPanel from "@/components/watchlist-panel";
import { and, count, desc, eq, gte, sql } from "drizzle-orm";
import { auth } from "@/auth";
import type { RatingsSummary } from "@/components/ratings-panel";
import type { WatchlistPanelItem } from "@/components/watchlist-panel";
import { episodes, movies, progressShows, ratings, shows, watchEvents, watchlistItems } from "@/db/schema";
import { getDb } from "@/lib/db";
import { DASHBOARD_BLOCK_IDS, getOrCreateDashboardLayout, orderDashboardBlockIds } from "@/lib/dashboard-layout";
import { getRatingsSummary } from "@/lib/ratings";

type RecentActivityItem = {
  id: number;
  mediaType: "movie" | "episode" | "show";
  title: string;
  subtitle: string;
  watchedAt: string;
};

type CurrentlyWatchingItem = {
  id: number;
  title: string;
  plays: number;
  lastWatchedAt: string | null;
};

type ProfileSummary = {
  handle: string | null;
  displayName: string | null;
  isPrivate: boolean | null;
  joinedAt: string | null;
  imageUrl: string | null;
};

type MonthReviewStats = {
  plays: number;
  ratings: number;
  hours: number;
  firstPlayLabel: string;
};

type DashboardMediaStats = {
  episodesWatched: number;
  showsActive: number;
  moviesWatched: number;
};

async function getDashboardMediaStats(appUserId: string): Promise<DashboardMediaStats> {
  try {
    const db = getDb();
    const [moviesWatched] = await db
      .select({ total: count() })
      .from(watchEvents)
      .where(and(eq(watchEvents.userId, appUserId), eq(watchEvents.mediaType, "movie")));

    const [episodesWatched] = await db
      .select({ total: count() })
      .from(watchEvents)
      .where(and(eq(watchEvents.userId, appUserId), eq(watchEvents.mediaType, "episode")));

    const [showsActive] = await db.select({ total: count() }).from(progressShows).where(eq(progressShows.userId, appUserId));

    return {
      moviesWatched: moviesWatched?.total ?? 0,
      episodesWatched: episodesWatched?.total ?? 0,
      showsActive: showsActive?.total ?? 0,
    };
  } catch {
    return {
      moviesWatched: 0,
      episodesWatched: 0,
      showsActive: 0,
    };
  }
}

async function getMonthReviewStats(appUserId: string): Promise<MonthReviewStats> {
  try {
    const db = getDb();
    const monthStart = new Date();
    monthStart.setUTCDate(1);
    monthStart.setUTCHours(0, 0, 0, 0);

    const [plays] = await db
      .select({ total: count() })
      .from(watchEvents)
      .where(and(eq(watchEvents.userId, appUserId), gte(watchEvents.watchedAt, monthStart)));

    const [monthRatings] = await db
      .select({ total: count() })
      .from(ratings)
      .where(and(eq(ratings.userId, appUserId), gte(ratings.ratedAt, monthStart)));

    const [firstPlay] = await db
      .select({
        watchedAt: watchEvents.watchedAt,
        movieTitle: movies.title,
        showTitle: shows.title,
      })
      .from(watchEvents)
      .leftJoin(movies, eq(watchEvents.movieId, movies.id))
      .leftJoin(episodes, eq(watchEvents.episodeId, episodes.id))
      .leftJoin(shows, eq(episodes.showId, shows.id))
      .where(and(eq(watchEvents.userId, appUserId), gte(watchEvents.watchedAt, monthStart)))
      .orderBy(watchEvents.watchedAt)
      .limit(1);

    const playCount = plays?.total ?? 0;
    const firstTitle = firstPlay?.movieTitle ?? firstPlay?.showTitle ?? "No plays yet";

    return {
      plays: playCount,
      ratings: monthRatings?.total ?? 0,
      hours: Math.round((playCount * 45) / 60),
      firstPlayLabel: firstPlay ? `${firstTitle} · ${firstPlay.watchedAt.toLocaleDateString()}` : firstTitle,
    };
  } catch {
    return {
      plays: 0,
      ratings: 0,
      hours: 0,
      firstPlayLabel: "No month activity yet",
    };
  }
}

async function getCurrentlyWatching(appUserId: string): Promise<CurrentlyWatchingItem[]> {
  try {
    const db = getDb();
    const rows = await db
      .select({
        id: progressShows.id,
        title: shows.title,
        plays: progressShows.plays,
        lastWatchedAt: progressShows.lastWatchedAt,
      })
      .from(progressShows)
      .leftJoin(shows, eq(progressShows.showId, shows.id))
      .where(eq(progressShows.userId, appUserId))
      .orderBy(desc(progressShows.lastWatchedAt), desc(progressShows.updatedAt))
      .limit(6);

    return rows.map((row) => ({
      id: row.id,
      title: row.title ?? "Untitled show",
      plays: row.plays ?? 0,
      lastWatchedAt: row.lastWatchedAt ? row.lastWatchedAt.toISOString() : null,
    }));
  } catch {
    return [];
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

async function getLast30DayActivity(appUserId: string): Promise<number[]> {
  try {
    const db = getDb();
    const start = new Date();
    start.setUTCDate(start.getUTCDate() - 29);
    start.setUTCHours(0, 0, 0, 0);

    const rows = await db
      .select({
        day: sql<string>`to_char(${watchEvents.watchedAt}::date, 'YYYY-MM-DD')`,
        total: count(),
      })
      .from(watchEvents)
      .where(and(eq(watchEvents.userId, appUserId), gte(watchEvents.watchedAt, start)))
      .groupBy(sql`${watchEvents.watchedAt}::date`)
      .orderBy(sql`${watchEvents.watchedAt}::date`);

    const byDay = new Map<string, number>();

    for (const row of rows) {
      byDay.set(row.day, row.total ?? 0);
    }

    const points: number[] = [];
    const cursor = new Date(start);

    for (let i = 0; i < 30; i += 1) {
      const isoDay = cursor.toISOString().slice(0, 10);
      points.push(byDay.get(isoDay) ?? 0);
      cursor.setUTCDate(cursor.getUTCDate() + 1);
    }

    return points;
  } catch {
    return Array.from({ length: 30 }, () => 0);
  }
}

function formatJoinedAt(joinedAt: string | null): string {
  if (!joinedAt) {
    return "Joined recently";
  }

  return `Joined ${new Date(joinedAt).toLocaleDateString()}`;
}

function Avatar({ profile }: { profile: ProfileSummary }) {
  const initials = (profile.displayName ?? profile.handle ?? "A").charAt(0).toUpperCase();

  if (profile.imageUrl) {
    return <Image src={profile.imageUrl} alt="Profile avatar" width={96} height={96} unoptimized className="h-24 w-24 rounded-full border-4 border-white/90 object-cover" />;
  }

  return (
    <div className="flex h-24 w-24 items-center justify-center rounded-full border-4 border-white/90 bg-gradient-to-br from-violet-500 to-fuchsia-600 text-4xl font-black text-white">
      {initials}
    </div>
  );
}

export default async function DashboardPage() {
  const session = await auth();
  const appUserId = session?.user?.appUserId;
  const mediaStats = appUserId ? await getDashboardMediaStats(appUserId) : { moviesWatched: 0, episodesWatched: 0, showsActive: 0 };
  const monthReview = appUserId ? await getMonthReviewStats(appUserId) : { plays: 0, ratings: 0, hours: 0, firstPlayLabel: "No month activity yet" };
  const initialWatchlistItems = appUserId ? await getInitialWatchlistItems(appUserId) : [];
  const recentActivity = appUserId ? await getRecentActivity(appUserId) : [];
  const currentlyWatching = appUserId ? await getCurrentlyWatching(appUserId) : [];
  const last30Days = appUserId ? await getLast30DayActivity(appUserId) : Array.from({ length: 30 }, () => 0);
  const layout = appUserId ? await getOrCreateDashboardLayout(appUserId) : null;
  const orderedBlockIds = layout ? orderDashboardBlockIds(layout) : [...DASHBOARD_BLOCK_IDS];
  const ratingsSummary = appUserId ? await getRatingsSummary(appUserId) : buildEmptyRatingsSummary();
  const profileSummary: ProfileSummary = {
    handle: session?.user?.handle ?? null,
    displayName: session?.user?.displayName ?? session?.user?.name ?? null,
    isPrivate: session?.user?.isPrivate ?? null,
    joinedAt: session?.user?.joinedAt ?? null,
    imageUrl: session?.user?.image ?? null,
  };

  const maxLast30 = Math.max(...last30Days, 1);

  const blockMap: Record<string, React.ReactNode> = {
    "month-review": (
      <section id="month-review" className="overflow-hidden rounded-3xl border border-white/10 bg-[radial-gradient(circle_at_20%_20%,rgba(122,78,255,0.28),transparent_45%),radial-gradient(circle_at_85%_15%,rgba(255,61,90,0.24),transparent_42%),linear-gradient(120deg,#0b1023_0%,#1a1738_55%,#121212_100%)] p-6 shadow-[0_18px_70px_rgba(0,0,0,0.45)]">
        <div className="grid gap-5 lg:grid-cols-[1fr_1.3fr]">
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-1">
            <article className="rounded-2xl border border-white/10 bg-black/25 p-4">
              <p className="text-xs uppercase tracking-[0.2em] text-slate-300">Plays</p>
              <p className="mt-2 text-4xl font-black text-white">{monthReview.plays}</p>
            </article>
            <article className="rounded-2xl border border-white/10 bg-black/25 p-4">
              <p className="text-xs uppercase tracking-[0.2em] text-slate-300">Hours</p>
              <p className="mt-2 text-4xl font-black text-white">{monthReview.hours}</p>
            </article>
            <article className="rounded-2xl border border-white/10 bg-black/25 p-4">
              <p className="text-xs uppercase tracking-[0.2em] text-slate-300">Ratings</p>
              <p className="mt-2 text-4xl font-black text-white">{monthReview.ratings}</p>
            </article>
          </div>
          <article className="rounded-2xl border border-white/10 bg-black/25 p-6">
            <p className="text-xs uppercase tracking-[0.2em] text-violet-200">First play of the month</p>
            <p className="mt-4 text-3xl font-black text-white">Month in Review</p>
            <p className="mt-3 text-lg text-slate-200">{monthReview.firstPlayLabel}</p>
            <div className="mt-5 flex flex-wrap gap-3">
              <Link href="#" className="rounded-xl bg-red-500 px-4 py-2 font-semibold text-white transition hover:bg-red-400">
                Upgrade Placeholder
              </Link>
              <Link href="#" className="rounded-xl bg-violet-500 px-4 py-2 font-semibold text-white transition hover:bg-violet-400">
                See Example
              </Link>
            </div>
          </article>
        </div>
      </section>
    ),
    "up-next": (
      <section id="shows" className="space-y-3">
        <div className="flex items-center justify-between gap-3">
          <h2 className="text-3xl font-bold text-white">Up Next</h2>
          <span className="rounded-full border border-white/10 bg-black/30 px-3 py-1 text-xs uppercase tracking-[0.18em] text-slate-300">Progress</span>
        </div>
        {currentlyWatching.length === 0 ? (
          <article className="card rounded-2xl p-5 text-sm text-slate-300">No active shows yet. Mark episodes watched to build this row.</article>
        ) : (
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-6">
            {currentlyWatching.map((item) => (
              <article key={item.id} className="card rounded-2xl p-3">
                <div className="aspect-[2/3] rounded-xl border border-white/10 bg-gradient-to-br from-slate-900 to-slate-700" />
                <p className="mt-3 truncate text-sm font-semibold text-white">{item.title}</p>
                <p className="text-xs text-slate-400">{item.plays} plays</p>
              </article>
            ))}
          </div>
        )}
      </section>
    ),
    watchlist: (
      <section id="movies" className="space-y-3">
        <div className="flex items-center justify-between gap-3">
          <h2 className="text-3xl font-bold text-white">Watchlist</h2>
          <span className="rounded-full border border-emerald-500/30 bg-emerald-500/10 px-3 py-1 text-xs uppercase tracking-[0.18em] text-emerald-300">
            Modular block
          </span>
        </div>
        <WatchlistPanel initialItems={initialWatchlistItems} />
      </section>
    ),
    "recently-watched": (
      <section className="space-y-3">
        <div className="flex items-center justify-between gap-3">
          <h2 className="text-3xl font-bold text-white">Recently Watched</h2>
          <span className="rounded-full border border-white/10 bg-black/30 px-3 py-1 text-xs uppercase tracking-[0.18em] text-slate-300">History</span>
        </div>
        {recentActivity.length === 0 ? (
          <article className="card rounded-2xl p-5 text-sm text-slate-300">No recent activity yet.</article>
        ) : (
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
            {recentActivity.slice(0, 6).map((item) => (
              <article key={item.id} className="card rounded-2xl overflow-hidden">
                <div className="aspect-[16/8] bg-gradient-to-r from-slate-800 via-slate-600 to-slate-800" />
                <div className="p-3">
                  <p className="text-sm font-semibold text-white">{item.title}</p>
                  <p className="mt-1 text-xs text-slate-400">{item.subtitle}</p>
                  <p className="mt-2 text-[11px] uppercase tracking-[0.18em] text-violet-300">{new Date(item.watchedAt).toLocaleString()}</p>
                </div>
              </article>
            ))}
          </div>
        )}
      </section>
    ),
    "last-30-days": (
      <section className="rounded-3xl border border-white/10 bg-black/25 p-5">
        <div className="flex items-center justify-between gap-3">
          <div>
            <h2 className="text-3xl font-bold text-white">Last 30 Days</h2>
            <p className="text-sm text-slate-300">
              {monthReview.hours}h watched · {mediaStats.episodesWatched} episodes · {mediaStats.moviesWatched} movies
            </p>
          </div>
          <div className="flex gap-3 text-xs uppercase tracking-[0.18em] text-slate-300">
            <Link href="#" className="transition hover:text-white">
              Yearly stats
            </Link>
            <Link href="#" className="transition hover:text-white">
              All time stats
            </Link>
          </div>
        </div>
        <div className="mt-5 flex h-36 items-end gap-1.5 rounded-xl border border-white/10 bg-slate-900/50 p-3">
          {last30Days.map((value, index) => (
            <div
              key={`last30-${index}`}
              className="flex-1 rounded-t-sm bg-slate-300/70"
              style={{ height: `${Math.max(8, Math.round((value / maxLast30) * 100))}%` }}
              title={`Day ${index + 1}: ${value}`}
            />
          ))}
        </div>
      </section>
    ),
    recommendations: (
      <section id="discover" className="space-y-3">
        <h2 className="text-3xl font-bold text-white">Recommendations</h2>
        <div className="grid gap-3 sm:grid-cols-2 md:grid-cols-3 xl:grid-cols-6">
          {Array.from({ length: 6 }, (_, index) => (
            <article key={`rec-${index}`} className="card rounded-2xl overflow-hidden">
              <div className="aspect-[2/3] bg-gradient-to-br from-emerald-800 via-cyan-700 to-blue-900" />
              <div className="p-3">
                <p className="text-sm font-semibold text-white">Recommendation {index + 1}</p>
                <p className="text-xs text-slate-400">Placeholder card</p>
              </div>
            </article>
          ))}
        </div>
      </section>
    ),
    "social-feed": (
      <section id="forums" className="space-y-3">
        <h2 className="text-3xl font-bold text-white">Social Feed</h2>
        <div className="grid gap-3 sm:grid-cols-2 md:grid-cols-3 xl:grid-cols-5">
          {Array.from({ length: 5 }, (_, index) => (
            <article key={`social-${index}`} className="card rounded-2xl overflow-hidden">
              <div className="aspect-[2/3] bg-gradient-to-br from-slate-800 to-slate-700" />
              <div className="p-3">
                <p className="text-sm font-semibold text-white">Following activity {index + 1}</p>
                <p className="text-xs text-slate-400">Placeholder until social model is enabled</p>
              </div>
            </article>
          ))}
        </div>
      </section>
    ),
    ratings: (
      <section>
        <RatingsPanel initialSummary={ratingsSummary} />
      </section>
    ),
  };

  return (
    <section className="space-y-8">
      <header className="overflow-hidden rounded-3xl border border-white/10 bg-[radial-gradient(circle_at_18%_22%,rgba(255,255,255,0.12),transparent_24%),radial-gradient(circle_at_82%_10%,rgba(221,98,255,0.22),transparent_30%),linear-gradient(120deg,#070b14_0%,#120f1f_48%,#05070f_100%)] shadow-[0_18px_70px_rgba(0,0,0,0.45)]">
        <div className="grid gap-6 p-6 lg:grid-cols-[1.25fr_1fr] lg:items-end">
          <div className="flex items-end gap-4">
            <Avatar profile={profileSummary} />
            <div>
              <div className="flex flex-wrap items-center gap-3">
                <h1 className="text-5xl font-black tracking-tight text-white">Hello, {profileSummary.displayName ?? profileSummary.handle ?? "friend"}</h1>
                <span className="rounded-full border border-red-300/30 bg-red-500/90 px-3 py-1 text-sm font-semibold uppercase tracking-[0.2em] text-white">Director</span>
              </div>
              <p className="mt-2 text-sm uppercase tracking-[0.16em] text-white/80">{formatJoinedAt(profileSummary.joinedAt)}</p>
              <div className="mt-4 grid gap-2 text-sm font-semibold text-white sm:grid-cols-2">
                <Link href="#month-review" className="transition hover:text-violet-300">Month in Review</Link>
                <Link href="#last-30-days" className="transition hover:text-violet-300">Year to Date</Link>
                <Link href="#" className="transition hover:text-violet-300">All Time Stats</Link>
                <Link href="/profile" className="transition hover:text-violet-300">Your Profile</Link>
              </div>
            </div>
          </div>
          <div className="grid gap-3 sm:grid-cols-3 lg:grid-cols-1">
            <article className="rounded-2xl border border-white/10 bg-black/30 p-4">
              <p className="text-xs uppercase tracking-[0.2em] text-slate-300">TV watched</p>
              <p className="mt-2 text-2xl font-black text-white">{mediaStats.episodesWatched} episodes</p>
              <p className="text-sm text-slate-300">{mediaStats.showsActive} active shows</p>
            </article>
            <article className="rounded-2xl border border-white/10 bg-black/30 p-4">
              <p className="text-xs uppercase tracking-[0.2em] text-slate-300">Movie watched</p>
              <p className="mt-2 text-2xl font-black text-white">{mediaStats.moviesWatched} movies</p>
              <p className="text-sm text-slate-300">All tracked plays</p>
            </article>
            <article className="rounded-2xl border border-white/10 bg-black/30 p-4">
              <p className="text-xs uppercase tracking-[0.2em] text-slate-300">Privacy</p>
              <p className="mt-2 text-xl font-black text-white">{profileSummary.isPrivate === false ? "Public" : "Private"}</p>
              <p className="text-sm text-slate-300">Configurable in profile</p>
            </article>
          </div>
        </div>
      </header>

      <div className="space-y-8">
        {orderedBlockIds.map((id) => (
          <div key={id}>{blockMap[id]}</div>
        ))}
      </div>
    </section>
  );
}
