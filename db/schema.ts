import {
  bigint,
  bigserial,
  boolean,
  check,
  date,
  index,
  integer,
  jsonb,
  numeric,
  pgEnum,
  pgTable,
  smallint,
  text,
  timestamp,
  uuid,
  uniqueIndex,
} from "drizzle-orm/pg-core";
import { sql } from "drizzle-orm";

export const mediaTypeEnum = pgEnum("media_type", ["movie", "show", "episode"]);
export const listItemTypeEnum = pgEnum("list_item_type", ["movie", "show"]);

export const appUsers = pgTable("app_users", {
  id: uuid("id").primaryKey().defaultRandom(),
  handle: text("handle").notNull().unique(),
  displayName: text("display_name"),
  isPrivate: boolean("is_private").notNull().default(true),
  location: text("location"),
  bio: text("bio"),
  gender: text("gender"),
  joinedAt: timestamp("joined_at", { withTimezone: true }),
  traktUserId: bigint("trakt_user_id", { mode: "number" }).unique(),
  traktSlug: text("trakt_slug").unique(),
  traktAvatarUrl: text("trakt_avatar_url"),
  createdAt: timestamp("created_at", { withTimezone: true }).notNull().defaultNow(),
  updatedAt: timestamp("updated_at", { withTimezone: true }).notNull().defaultNow(),
});

export const movies = pgTable(
  "movies",
  {
    id: bigserial("id", { mode: "number" }).primaryKey(),
    title: text("title").notNull(),
    releaseYear: integer("release_year"),
    releasedOn: date("released_on"),
    traktId: bigint("trakt_id", { mode: "number" }).unique(),
    traktSlug: text("trakt_slug").unique(),
    tmdbId: bigint("tmdb_id", { mode: "number" }),
    imdbId: text("imdb_id"),
    languageCode: text("language_code"),
    runtimeMinutes: integer("runtime_minutes"),
    metadata: jsonb("metadata").notNull().default({}),
    createdAt: timestamp("created_at", { withTimezone: true }).notNull().defaultNow(),
    updatedAt: timestamp("updated_at", { withTimezone: true }).notNull().defaultNow(),
  },
  (table) => ({
    tmdbIdUnique: uniqueIndex("movies_tmdb_id_uq").on(table.tmdbId),
    imdbIdUnique: uniqueIndex("movies_imdb_id_uq").on(table.imdbId),
  })
);

export const shows = pgTable(
  "shows",
  {
    id: bigserial("id", { mode: "number" }).primaryKey(),
    title: text("title").notNull(),
    releaseYear: integer("release_year"),
    airedEpisodes: integer("aired_episodes"),
    traktId: bigint("trakt_id", { mode: "number" }).unique(),
    traktSlug: text("trakt_slug").unique(),
    tmdbId: bigint("tmdb_id", { mode: "number" }),
    tvdbId: bigint("tvdb_id", { mode: "number" }),
    imdbId: text("imdb_id"),
    status: text("status"),
    languageCode: text("language_code"),
    runtimeMinutes: integer("runtime_minutes"),
    metadata: jsonb("metadata").notNull().default({}),
    createdAt: timestamp("created_at", { withTimezone: true }).notNull().defaultNow(),
    updatedAt: timestamp("updated_at", { withTimezone: true }).notNull().defaultNow(),
  },
  (table) => ({
    tmdbIdUnique: uniqueIndex("shows_tmdb_id_uq").on(table.tmdbId),
    tvdbIdUnique: uniqueIndex("shows_tvdb_id_uq").on(table.tvdbId),
    imdbIdUnique: uniqueIndex("shows_imdb_id_uq").on(table.imdbId),
  })
);

export const episodes = pgTable(
  "episodes",
  {
    id: bigserial("id", { mode: "number" }).primaryKey(),
    showId: bigint("show_id", { mode: "number" })
      .notNull()
      .references(() => shows.id, { onDelete: "cascade" }),
    seasonNumber: integer("season_number").notNull(),
    episodeNumber: integer("episode_number").notNull(),
    title: text("title"),
    traktId: bigint("trakt_id", { mode: "number" }).unique(),
    tmdbId: bigint("tmdb_id", { mode: "number" }),
    tvdbId: bigint("tvdb_id", { mode: "number" }),
    imdbId: text("imdb_id"),
    firstAiredAt: timestamp("first_aired_at", { withTimezone: true }),
    metadata: jsonb("metadata").notNull().default({}),
    createdAt: timestamp("created_at", { withTimezone: true }).notNull().defaultNow(),
    updatedAt: timestamp("updated_at", { withTimezone: true }).notNull().defaultNow(),
  },
  (table) => ({
    showSeasonEpisodeUnique: uniqueIndex("episodes_show_season_episode_uq").on(
      table.showId,
      table.seasonNumber,
      table.episodeNumber
    ),
    tmdbIdUnique: uniqueIndex("episodes_tmdb_id_uq").on(table.tmdbId),
    tvdbIdUnique: uniqueIndex("episodes_tvdb_id_uq").on(table.tvdbId),
    imdbIdUnique: uniqueIndex("episodes_imdb_id_uq").on(table.imdbId),
  })
);

export const watchlistItems = pgTable(
  "watchlist_items",
  {
    id: bigserial("id", { mode: "number" }).primaryKey(),
    userId: uuid("user_id")
      .notNull()
      .references(() => appUsers.id, { onDelete: "cascade" }),
    itemType: listItemTypeEnum("item_type").notNull(),
    movieId: bigint("movie_id", { mode: "number" }).references(() => movies.id, { onDelete: "cascade" }),
    showId: bigint("show_id", { mode: "number" }).references(() => shows.id, { onDelete: "cascade" }),
    rank: integer("rank"),
    listedAt: timestamp("listed_at", { withTimezone: true }),
    sourceItemId: bigint("source_item_id", { mode: "number" }),
    notes: text("notes"),
    createdAt: timestamp("created_at", { withTimezone: true }).notNull().defaultNow(),
    updatedAt: timestamp("updated_at", { withTimezone: true }).notNull().defaultNow(),
  },
  (table) => ({
    movieTypeShape: check(
      "watchlist_items_item_type_check",
      sql`(
        (${table.itemType} = 'movie' and ${table.movieId} is not null and ${table.showId} is null) or
        (${table.itemType} = 'show' and ${table.showId} is not null and ${table.movieId} is null)
      )`
    ),
    watchlistMovieUserUnique: uniqueIndex("watchlist_movie_user_uq")
      .on(table.userId, table.movieId)
      .where(sql`${table.itemType} = 'movie'`),
    watchlistShowUserUnique: uniqueIndex("watchlist_show_user_uq")
      .on(table.userId, table.showId)
      .where(sql`${table.itemType} = 'show'`),
  })
);

export const watchEvents = pgTable("watch_events", {
  id: bigserial("id", { mode: "number" }).primaryKey(),
  userId: uuid("user_id")
    .notNull()
    .references(() => appUsers.id, { onDelete: "cascade" }),
  mediaType: mediaTypeEnum("media_type").notNull(),
  movieId: bigint("movie_id", { mode: "number" }).references(() => movies.id, { onDelete: "cascade" }),
  episodeId: bigint("episode_id", { mode: "number" }).references(() => episodes.id, { onDelete: "cascade" }),
  watchedAt: timestamp("watched_at", { withTimezone: true }).notNull(),
  action: text("action").notNull().default("watch"),
  sourceHistoryId: bigint("source_history_id", { mode: "number" }),
  sourceFile: text("source_file"),
  rawEvent: jsonb("raw_event").notNull().default({}),
  createdAt: timestamp("created_at", { withTimezone: true }).notNull().defaultNow(),
});

export const progressMovies = pgTable(
  "progress_movies",
  {
    id: bigserial("id", { mode: "number" }).primaryKey(),
    userId: uuid("user_id")
      .notNull()
      .references(() => appUsers.id, { onDelete: "cascade" }),
    movieId: bigint("movie_id", { mode: "number" })
      .notNull()
      .references(() => movies.id, { onDelete: "cascade" }),
    plays: integer("plays").notNull().default(0),
    totalCount: integer("total_count"),
    lastWatchedAt: timestamp("last_watched_at", { withTimezone: true }),
    lastUpdatedAt: timestamp("last_updated_at", { withTimezone: true }),
    sourceFile: text("source_file"),
    rawPayload: jsonb("raw_payload").notNull().default({}),
    createdAt: timestamp("created_at", { withTimezone: true }).notNull().defaultNow(),
    updatedAt: timestamp("updated_at", { withTimezone: true }).notNull().defaultNow(),
  },
  (table) => ({
    userMovieUnique: uniqueIndex("progress_movies_user_movie_uq").on(table.userId, table.movieId),
  })
);

export const progressShows = pgTable(
  "progress_shows",
  {
    id: bigserial("id", { mode: "number" }).primaryKey(),
    userId: uuid("user_id")
      .notNull()
      .references(() => appUsers.id, { onDelete: "cascade" }),
    showId: bigint("show_id", { mode: "number" })
      .notNull()
      .references(() => shows.id, { onDelete: "cascade" }),
    plays: integer("plays").notNull().default(0),
    lastWatchedAt: timestamp("last_watched_at", { withTimezone: true }),
    lastUpdatedAt: timestamp("last_updated_at", { withTimezone: true }),
    resetAt: timestamp("reset_at", { withTimezone: true }),
    sourceFile: text("source_file"),
    rawPayload: jsonb("raw_payload").notNull().default({}),
    createdAt: timestamp("created_at", { withTimezone: true }).notNull().defaultNow(),
    updatedAt: timestamp("updated_at", { withTimezone: true }).notNull().defaultNow(),
  },
  (table) => ({
    userShowUnique: uniqueIndex("progress_shows_user_show_uq").on(table.userId, table.showId),
  })
);

export const progressEpisodes = pgTable(
  "progress_episodes",
  {
    id: bigserial("id", { mode: "number" }).primaryKey(),
    userId: uuid("user_id")
      .notNull()
      .references(() => appUsers.id, { onDelete: "cascade" }),
    episodeId: bigint("episode_id", { mode: "number" })
      .notNull()
      .references(() => episodes.id, { onDelete: "cascade" }),
    plays: integer("plays").notNull().default(0),
    lastWatchedAt: timestamp("last_watched_at", { withTimezone: true }),
    sourceFile: text("source_file"),
    rawPayload: jsonb("raw_payload").notNull().default({}),
    createdAt: timestamp("created_at", { withTimezone: true }).notNull().defaultNow(),
    updatedAt: timestamp("updated_at", { withTimezone: true }).notNull().defaultNow(),
  },
  (table) => ({
    userEpisodeUnique: uniqueIndex("progress_episodes_user_episode_uq").on(table.userId, table.episodeId),
  })
);

export const ratings = pgTable(
  "ratings",
  {
    id: bigserial("id", { mode: "number" }).primaryKey(),
    userId: uuid("user_id")
      .notNull()
      .references(() => appUsers.id, { onDelete: "cascade" }),
    mediaType: mediaTypeEnum("media_type").notNull(),
    movieId: bigint("movie_id", { mode: "number" }).references(() => movies.id, { onDelete: "cascade" }),
    showId: bigint("show_id", { mode: "number" }).references(() => shows.id, { onDelete: "cascade" }),
    episodeId: bigint("episode_id", { mode: "number" }),
    ratingValue: smallint("rating_value").notNull(),
    ratedAt: timestamp("rated_at", { withTimezone: true }).notNull(),
    sourceFile: text("source_file"),
    rawRating: jsonb("raw_rating").notNull().default({}),
    createdAt: timestamp("created_at", { withTimezone: true }).notNull().defaultNow(),
    updatedAt: timestamp("updated_at", { withTimezone: true }).notNull().defaultNow(),
  },
  (table) => ({
    ratingsValueRange: check("ratings_rating_value_check", sql`${table.ratingValue} between 1 and 10`),
  })
);

export const statsSnapshots = pgTable(
  "stats_snapshots",
  {
    id: bigserial("id", { mode: "number" }).primaryKey(),
    userId: uuid("user_id")
      .notNull()
      .references(() => appUsers.id, { onDelete: "cascade" }),
    snapshotDay: date("snapshot_day").notNull(),
    watchedCount: integer("watched_count").notNull().default(0),
    moviesWatchedCount: integer("movies_watched_count").notNull().default(0),
    episodesWatchedCount: integer("episodes_watched_count").notNull().default(0),
    watchlistCount: integer("watchlist_count").notNull().default(0),
    ratingsCount: integer("ratings_count").notNull().default(0),
    averageRating: numeric("average_rating", { precision: 4, scale: 2 }),
    totalMinutesWatched: integer("total_minutes_watched"),
    payload: jsonb("payload").notNull().default({}),
    createdAt: timestamp("created_at", { withTimezone: true }).notNull().defaultNow(),
  },
  (table) => ({
    userSnapshotUnique: uniqueIndex("stats_snapshots_user_day_uq").on(table.userId, table.snapshotDay),
  })
);

export const traktImportRuns = pgTable(
  "trakt_import_runs",
  {
    id: bigserial("id", { mode: "number" }).primaryKey(),
    userId: uuid("user_id").references(() => appUsers.id, { onDelete: "set null" }),
    sourcePath: text("source_path").notNull(),
    selectedFiles: jsonb("selected_files").notNull().default([]),
    startedAt: timestamp("started_at", { withTimezone: true }).notNull().defaultNow(),
    finishedAt: timestamp("finished_at", { withTimezone: true }),
    status: text("status").notNull().default("running"),
    notes: text("notes"),
  },
  (table) => ({
    startedAtIndex: index("trakt_import_runs_started_idx").on(table.startedAt),
  })
);
