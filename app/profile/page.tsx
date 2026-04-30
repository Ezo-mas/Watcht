import { and, count, desc, eq } from "drizzle-orm";
import { redirect } from "next/navigation";
import { auth } from "@/auth";
import ProfilePage, { type ProfilePageBlock } from "@/components/profile-page";
import { episodes, movies, progressEpisodes, progressMovies, progressShows, ratings, shows, watchEvents, watchlistItems } from "@/db/schema";
import { applyProfileLayout, getOrCreateProfileLayout } from "../../lib/profile-layout";
import { getDb } from "@/lib/db";
import { getAppUserSummaryById } from "@/lib/app-users";

export const runtime = "nodejs";

type WatchEventRow = {
  title: string;
  subtitle: string;
  watchedAt: string;
};

type RankedItem = {
  title: string;
  subtitle: string;
  meta: string;
};

type FeaturedListItem = {
  title: string;
  subtitle: string;
  meta: string;
};

async function requireProfileUserId() {
  const session = await auth();
  const appUserId = session?.user?.appUserId;

  if (!appUserId) {
    redirect("/auth/sign-in");
  }

  return { session, appUserId };
}

async function getLastWatchedMovie(appUserId: string): Promise<string> {
  const db = getDb();
  const [row] = await db
    .select({ title: movies.title, watchedAt: watchEvents.watchedAt })
    .from(watchEvents)
    .leftJoin(movies, eq(watchEvents.movieId, movies.id))
    .where(and(eq(watchEvents.userId, appUserId), eq(watchEvents.mediaType, "movie")))
    .orderBy(desc(watchEvents.watchedAt))
    .limit(1);

  if (!row) {
    return "No movie watched yet";
  }

  const watchedAt = row.watchedAt ? row.watchedAt.toLocaleDateString() : "Recently";
  return `${row.title ?? "Untitled movie"} · ${watchedAt}`;
}

async function getProfileCounts(appUserId: string) {
  const db = getDb();

  const [watchlistRow] = await db.select({ total: count() }).from(watchlistItems).where(eq(watchlistItems.userId, appUserId));
  const [ratingsRow] = await db.select({ total: count() }).from(ratings).where(eq(ratings.userId, appUserId));
  const [episodesRow] = await db.select({ total: count() }).from(progressEpisodes).where(eq(progressEpisodes.userId, appUserId));
  const [showsRow] = await db.select({ total: count() }).from(progressShows).where(eq(progressShows.userId, appUserId));
  const [moviesRow] = await db.select({ total: count() }).from(progressMovies).where(eq(progressMovies.userId, appUserId));

  return {
    watchlist: watchlistRow?.total ?? 0,
    ratings: ratingsRow?.total ?? 0,
    episodes: episodesRow?.total ?? 0,
    shows: showsRow?.total ?? 0,
    movies: moviesRow?.total ?? 0,
  };
}

async function getFeaturedList(appUserId: string): Promise<FeaturedListItem[]> {
  const db = getDb();
  const rows = await db
    .select({ title: movies.title, showTitle: shows.title, itemType: watchlistItems.itemType, listedAt: watchlistItems.listedAt })
    .from(watchlistItems)
    .leftJoin(movies, eq(watchlistItems.movieId, movies.id))
    .leftJoin(shows, eq(watchlistItems.showId, shows.id))
    .where(eq(watchlistItems.userId, appUserId))
    .orderBy(desc(watchlistItems.createdAt))
    .limit(6);

  return rows.map((row) => ({
    title: row.itemType === "movie" ? row.title ?? "Untitled movie" : row.showTitle ?? "Untitled show",
    subtitle: row.itemType === "movie" ? "Movie watchlist" : "Show watchlist",
    meta: row.listedAt ? row.listedAt.toLocaleDateString() : "Listed recently",
  }));
}

async function getFavorites(appUserId: string): Promise<RankedItem[]> {
  const db = getDb();
  const movieRows = await db
    .select({ title: movies.title, ratingValue: ratings.ratingValue, ratedAt: ratings.ratedAt })
    .from(ratings)
    .leftJoin(movies, eq(ratings.movieId, movies.id))
    .where(and(eq(ratings.userId, appUserId), eq(ratings.mediaType, "movie")))
    .orderBy(desc(ratings.ratingValue), desc(ratings.ratedAt))
    .limit(4);

  const showRows = await db
    .select({ title: shows.title, ratingValue: ratings.ratingValue, ratedAt: ratings.ratedAt })
    .from(ratings)
    .leftJoin(shows, eq(ratings.showId, shows.id))
    .where(and(eq(ratings.userId, appUserId), eq(ratings.mediaType, "show")))
    .orderBy(desc(ratings.ratingValue), desc(ratings.ratedAt))
    .limit(4);

  return [...movieRows, ...showRows]
    .sort((left, right) => (right.ratingValue ?? 0) - (left.ratingValue ?? 0))
    .slice(0, 6)
    .map((row) => ({
      title: row.title ?? "Untitled title",
      subtitle: "Top rated",
      meta: `${(row.ratingValue ?? 0) / 2} stars`,
    }));
}

async function getRecentlyWatched(appUserId: string): Promise<WatchEventRow[]> {
  const db = getDb();
  const rows = await db
    .select({
      mediaType: watchEvents.mediaType,
      movieTitle: movies.title,
      showTitle: shows.title,
      seasonNumber: episodes.seasonNumber,
      episodeNumber: episodes.episodeNumber,
      episodeTitle: episodes.title,
      watchedAt: watchEvents.watchedAt,
    })
    .from(watchEvents)
    .leftJoin(movies, eq(watchEvents.movieId, movies.id))
    .leftJoin(episodes, eq(watchEvents.episodeId, episodes.id))
    .leftJoin(shows, eq(episodes.showId, shows.id))
    .where(eq(watchEvents.userId, appUserId))
    .orderBy(desc(watchEvents.watchedAt))
    .limit(6);

  return rows.map((row) => ({
    title: row.mediaType === "movie" ? row.movieTitle ?? "Untitled movie" : row.showTitle ?? "Untitled show",
    subtitle:
      row.mediaType === "movie"
        ? "Movie"
        : `S${String(row.seasonNumber ?? 0).padStart(2, "0")}E${String(row.episodeNumber ?? 0).padStart(2, "0")}${row.episodeTitle ? ` · ${row.episodeTitle}` : ""}`,
    watchedAt: row.watchedAt ? row.watchedAt.toLocaleDateString() : "Recently",
  }));
}

async function getMostWatched(appUserId: string): Promise<RankedItem[]> {
  const db = getDb();
  const movieRows = await db
    .select({ title: movies.title, plays: progressMovies.plays, lastWatchedAt: progressMovies.lastWatchedAt })
    .from(progressMovies)
    .leftJoin(movies, eq(progressMovies.movieId, movies.id))
    .where(eq(progressMovies.userId, appUserId))
    .orderBy(desc(progressMovies.plays), desc(progressMovies.lastWatchedAt))
    .limit(3);

  const showRows = await db
    .select({ title: shows.title, plays: progressShows.plays, lastWatchedAt: progressShows.lastWatchedAt })
    .from(progressShows)
    .leftJoin(shows, eq(progressShows.showId, shows.id))
    .where(eq(progressShows.userId, appUserId))
    .orderBy(desc(progressShows.plays), desc(progressShows.lastWatchedAt))
    .limit(3);

  return [...movieRows, ...showRows]
    .sort((left, right) => (right.plays ?? 0) - (left.plays ?? 0))
    .slice(0, 6)
    .map((row) => ({
      title: row.title ?? "Untitled title",
      subtitle: `${row.plays ?? 0} plays`,
      meta: row.lastWatchedAt ? row.lastWatchedAt.toLocaleDateString() : "No recent play",
    }));
}

function buildCommentsBlock() {
  return {
    id: "comments",
    kind: "comments" as const,
    title: "Comments",
    subtitle: "Community notes and replies will land here later.",
    timeframe: "Last 30 days",
    items: [],
    emptyText: "Comments are not connected yet. This block is ready for the social layer when you turn it on.",
  };
}

export default async function ProfilePageRoute() {
  const { session, appUserId } = await requireProfileUserId();
  const appUser = await getAppUserSummaryById(appUserId);
  const counts = await getProfileCounts(appUserId);
  const featuredList = await getFeaturedList(appUserId);
  const favorites = await getFavorites(appUserId);
  const recentWatched = await getRecentlyWatched(appUserId);
  const mostWatched = await getMostWatched(appUserId);
  const layout = await getOrCreateProfileLayout(appUserId);

  const blocks: ProfilePageBlock[] = [
    {
      id: "about-grid",
      kind: "cards",
      title: "Profile blocks",
      subtitle: "The first four panels are intentionally structured so the order can be customized later.",
      cards: [
        {
          label: "About me",
          title: appUser?.displayName ?? session.user.name ?? appUser?.handle ?? "Account",
          body: [
            appUser?.handle ? `@${appUser.handle}` : null,
            appUser?.joinedAt ? `Joined ${appUser.joinedAt.toLocaleDateString()}` : null,
            appUser?.location ? appUser.location : null,
            appUser?.isPrivate === false ? "Public profile" : "Private by default",
          ]
            .filter(Boolean)
            .join(" · "),
        },
        {
          label: "Last watched movie",
          title: await getLastWatchedMovie(appUserId),
          body: "Latest film check-in from your watch history.",
        },
        {
          label: "TV watched stats",
          title: `${counts.episodes} episodes / ${counts.shows} shows`,
          body: `${counts.movies} movies logged · ${counts.ratings} ratings submitted`,
        },
        {
          label: "Featured list",
          title: `${counts.watchlist} saved items`,
          body: featuredList[0] ? `${featuredList[0].title} is pinned at the top.` : "Use this slot for a curated list later.",
        },
      ],
    },
    {
      id: "featured-list",
      kind: "list",
      title: "Featured List",
      subtitle: "Current watchlist items rendered as a profile block.",
      items: featuredList,
      emptyText: "Add titles to your watchlist and this block will fill automatically.",
      timeLabel: "Watchlist",
    },
    {
      id: "favorites-list",
      kind: "list",
      title: "Favorites",
      subtitle: "Highest-rated titles in your account.",
      items: favorites,
      emptyText: "Rate a few titles to build your favorites list.",
      timeLabel: "Top rated",
    },
    {
      id: "recently-watched",
      kind: "list",
      title: "Recently Watched",
      subtitle: "Newest movie and episode activity.",
      items: recentWatched,
      emptyText: "Watch a movie or episode to populate recent activity.",
      timeLabel: "Latest",
    },
    {
      id: "most-watched",
      kind: "list",
      title: "Most Watched Stats",
      subtitle: "Titles sorted by play count.",
      items: mostWatched,
      emptyText: "Play counts will show up here once progress grows.",
      timeLabel: "All time",
    },
    buildCommentsBlock(),
  ];

  const hero = {
    imageUrl: session.user.image ?? null,
    name: appUser?.displayName ?? session.user.name ?? appUser?.handle ?? "Account",
    handle: appUser?.handle ?? session.user.handle ?? null,
    badge: "Director",
    followersLabel: "0 Followers",
    followingLabel: "0 Following",
    locationLabel: appUser?.location ?? null,
    privacyLabel: appUser?.isPrivate === false ? "Public profile" : "Private profile",
  };

  return <ProfilePage hero={hero} blocks={applyProfileLayout(blocks, layout)} />;
}
