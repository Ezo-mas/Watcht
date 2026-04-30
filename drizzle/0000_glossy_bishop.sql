CREATE TYPE "public"."list_item_type" AS ENUM('movie', 'show');--> statement-breakpoint
CREATE TYPE "public"."media_type" AS ENUM('movie', 'show', 'episode');--> statement-breakpoint
CREATE TABLE "app_users" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"handle" text NOT NULL,
	"display_name" text,
	"is_private" boolean DEFAULT true NOT NULL,
	"location" text,
	"bio" text,
	"gender" text,
	"joined_at" timestamp with time zone,
	"trakt_user_id" bigint,
	"trakt_slug" text,
	"trakt_avatar_url" text,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "app_users_handle_unique" UNIQUE("handle"),
	CONSTRAINT "app_users_trakt_user_id_unique" UNIQUE("trakt_user_id"),
	CONSTRAINT "app_users_trakt_slug_unique" UNIQUE("trakt_slug")
);
--> statement-breakpoint
CREATE TABLE "dashboard_layouts" (
	"user_id" uuid PRIMARY KEY NOT NULL,
	"block_order" jsonb DEFAULT '[]'::jsonb NOT NULL,
	"hidden_block_ids" jsonb DEFAULT '[]'::jsonb NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
CREATE TABLE "episodes" (
	"id" bigserial PRIMARY KEY NOT NULL,
	"show_id" bigint NOT NULL,
	"season_number" integer NOT NULL,
	"episode_number" integer NOT NULL,
	"title" text,
	"trakt_id" bigint,
	"tmdb_id" bigint,
	"tvdb_id" bigint,
	"imdb_id" text,
	"first_aired_at" timestamp with time zone,
	"metadata" jsonb DEFAULT '{}'::jsonb NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "episodes_trakt_id_unique" UNIQUE("trakt_id")
);
--> statement-breakpoint
CREATE TABLE "movies" (
	"id" bigserial PRIMARY KEY NOT NULL,
	"title" text NOT NULL,
	"release_year" integer,
	"released_on" date,
	"trakt_id" bigint,
	"trakt_slug" text,
	"tmdb_id" bigint,
	"imdb_id" text,
	"language_code" text,
	"runtime_minutes" integer,
	"metadata" jsonb DEFAULT '{}'::jsonb NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "movies_trakt_id_unique" UNIQUE("trakt_id"),
	CONSTRAINT "movies_trakt_slug_unique" UNIQUE("trakt_slug")
);
--> statement-breakpoint
CREATE TABLE "profile_layouts" (
	"user_id" uuid PRIMARY KEY NOT NULL,
	"block_order" jsonb DEFAULT '[]'::jsonb NOT NULL,
	"hidden_block_ids" jsonb DEFAULT '[]'::jsonb NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
CREATE TABLE "progress_episodes" (
	"id" bigserial PRIMARY KEY NOT NULL,
	"user_id" uuid NOT NULL,
	"episode_id" bigint NOT NULL,
	"plays" integer DEFAULT 0 NOT NULL,
	"last_watched_at" timestamp with time zone,
	"source_file" text,
	"raw_payload" jsonb DEFAULT '{}'::jsonb NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
CREATE TABLE "progress_movies" (
	"id" bigserial PRIMARY KEY NOT NULL,
	"user_id" uuid NOT NULL,
	"movie_id" bigint NOT NULL,
	"plays" integer DEFAULT 0 NOT NULL,
	"total_count" integer,
	"last_watched_at" timestamp with time zone,
	"last_updated_at" timestamp with time zone,
	"source_file" text,
	"raw_payload" jsonb DEFAULT '{}'::jsonb NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
CREATE TABLE "progress_shows" (
	"id" bigserial PRIMARY KEY NOT NULL,
	"user_id" uuid NOT NULL,
	"show_id" bigint NOT NULL,
	"plays" integer DEFAULT 0 NOT NULL,
	"last_watched_at" timestamp with time zone,
	"last_updated_at" timestamp with time zone,
	"reset_at" timestamp with time zone,
	"source_file" text,
	"raw_payload" jsonb DEFAULT '{}'::jsonb NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
CREATE TABLE "ratings" (
	"id" bigserial PRIMARY KEY NOT NULL,
	"user_id" uuid NOT NULL,
	"media_type" "media_type" NOT NULL,
	"movie_id" bigint,
	"show_id" bigint,
	"episode_id" bigint,
	"rating_value" smallint NOT NULL,
	"rated_at" timestamp with time zone NOT NULL,
	"source_file" text,
	"raw_rating" jsonb DEFAULT '{}'::jsonb NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "ratings_rating_value_check" CHECK ("ratings"."rating_value" between 1 and 10)
);
--> statement-breakpoint
CREATE TABLE "shows" (
	"id" bigserial PRIMARY KEY NOT NULL,
	"title" text NOT NULL,
	"release_year" integer,
	"aired_episodes" integer,
	"trakt_id" bigint,
	"trakt_slug" text,
	"tmdb_id" bigint,
	"tvdb_id" bigint,
	"imdb_id" text,
	"status" text,
	"language_code" text,
	"runtime_minutes" integer,
	"metadata" jsonb DEFAULT '{}'::jsonb NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "shows_trakt_id_unique" UNIQUE("trakt_id"),
	CONSTRAINT "shows_trakt_slug_unique" UNIQUE("trakt_slug")
);
--> statement-breakpoint
CREATE TABLE "stats_snapshots" (
	"id" bigserial PRIMARY KEY NOT NULL,
	"user_id" uuid NOT NULL,
	"snapshot_day" date NOT NULL,
	"watched_count" integer DEFAULT 0 NOT NULL,
	"movies_watched_count" integer DEFAULT 0 NOT NULL,
	"episodes_watched_count" integer DEFAULT 0 NOT NULL,
	"watchlist_count" integer DEFAULT 0 NOT NULL,
	"ratings_count" integer DEFAULT 0 NOT NULL,
	"average_rating" numeric(4, 2),
	"total_minutes_watched" integer,
	"payload" jsonb DEFAULT '{}'::jsonb NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
CREATE TABLE "trakt_import_runs" (
	"id" bigserial PRIMARY KEY NOT NULL,
	"user_id" uuid,
	"source_path" text NOT NULL,
	"selected_files" jsonb DEFAULT '[]'::jsonb NOT NULL,
	"started_at" timestamp with time zone DEFAULT now() NOT NULL,
	"finished_at" timestamp with time zone,
	"status" text DEFAULT 'running' NOT NULL,
	"notes" text
);
--> statement-breakpoint
CREATE TABLE "watch_events" (
	"id" bigserial PRIMARY KEY NOT NULL,
	"user_id" uuid NOT NULL,
	"media_type" "media_type" NOT NULL,
	"movie_id" bigint,
	"episode_id" bigint,
	"watched_at" timestamp with time zone NOT NULL,
	"action" text DEFAULT 'watch' NOT NULL,
	"source_history_id" bigint,
	"source_file" text,
	"raw_event" jsonb DEFAULT '{}'::jsonb NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
CREATE TABLE "watchlist_items" (
	"id" bigserial PRIMARY KEY NOT NULL,
	"user_id" uuid NOT NULL,
	"item_type" "list_item_type" NOT NULL,
	"movie_id" bigint,
	"show_id" bigint,
	"rank" integer,
	"listed_at" timestamp with time zone,
	"source_item_id" bigint,
	"notes" text,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "watchlist_items_item_type_check" CHECK ((
        ("watchlist_items"."item_type" = 'movie' and "watchlist_items"."movie_id" is not null and "watchlist_items"."show_id" is null) or
        ("watchlist_items"."item_type" = 'show' and "watchlist_items"."show_id" is not null and "watchlist_items"."movie_id" is null)
      ))
);
--> statement-breakpoint
ALTER TABLE "dashboard_layouts" ADD CONSTRAINT "dashboard_layouts_user_id_app_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."app_users"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "episodes" ADD CONSTRAINT "episodes_show_id_shows_id_fk" FOREIGN KEY ("show_id") REFERENCES "public"."shows"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "profile_layouts" ADD CONSTRAINT "profile_layouts_user_id_app_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."app_users"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "progress_episodes" ADD CONSTRAINT "progress_episodes_user_id_app_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."app_users"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "progress_episodes" ADD CONSTRAINT "progress_episodes_episode_id_episodes_id_fk" FOREIGN KEY ("episode_id") REFERENCES "public"."episodes"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "progress_movies" ADD CONSTRAINT "progress_movies_user_id_app_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."app_users"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "progress_movies" ADD CONSTRAINT "progress_movies_movie_id_movies_id_fk" FOREIGN KEY ("movie_id") REFERENCES "public"."movies"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "progress_shows" ADD CONSTRAINT "progress_shows_user_id_app_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."app_users"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "progress_shows" ADD CONSTRAINT "progress_shows_show_id_shows_id_fk" FOREIGN KEY ("show_id") REFERENCES "public"."shows"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "ratings" ADD CONSTRAINT "ratings_user_id_app_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."app_users"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "ratings" ADD CONSTRAINT "ratings_movie_id_movies_id_fk" FOREIGN KEY ("movie_id") REFERENCES "public"."movies"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "ratings" ADD CONSTRAINT "ratings_show_id_shows_id_fk" FOREIGN KEY ("show_id") REFERENCES "public"."shows"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "stats_snapshots" ADD CONSTRAINT "stats_snapshots_user_id_app_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."app_users"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "trakt_import_runs" ADD CONSTRAINT "trakt_import_runs_user_id_app_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."app_users"("id") ON DELETE set null ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "watch_events" ADD CONSTRAINT "watch_events_user_id_app_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."app_users"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "watch_events" ADD CONSTRAINT "watch_events_movie_id_movies_id_fk" FOREIGN KEY ("movie_id") REFERENCES "public"."movies"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "watch_events" ADD CONSTRAINT "watch_events_episode_id_episodes_id_fk" FOREIGN KEY ("episode_id") REFERENCES "public"."episodes"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "watchlist_items" ADD CONSTRAINT "watchlist_items_user_id_app_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."app_users"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "watchlist_items" ADD CONSTRAINT "watchlist_items_movie_id_movies_id_fk" FOREIGN KEY ("movie_id") REFERENCES "public"."movies"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "watchlist_items" ADD CONSTRAINT "watchlist_items_show_id_shows_id_fk" FOREIGN KEY ("show_id") REFERENCES "public"."shows"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
CREATE UNIQUE INDEX "episodes_show_season_episode_uq" ON "episodes" USING btree ("show_id","season_number","episode_number");--> statement-breakpoint
CREATE UNIQUE INDEX "episodes_tmdb_id_uq" ON "episodes" USING btree ("tmdb_id");--> statement-breakpoint
CREATE UNIQUE INDEX "episodes_tvdb_id_uq" ON "episodes" USING btree ("tvdb_id");--> statement-breakpoint
CREATE UNIQUE INDEX "episodes_imdb_id_uq" ON "episodes" USING btree ("imdb_id");--> statement-breakpoint
CREATE UNIQUE INDEX "movies_tmdb_id_uq" ON "movies" USING btree ("tmdb_id");--> statement-breakpoint
CREATE UNIQUE INDEX "movies_imdb_id_uq" ON "movies" USING btree ("imdb_id");--> statement-breakpoint
CREATE UNIQUE INDEX "progress_episodes_user_episode_uq" ON "progress_episodes" USING btree ("user_id","episode_id");--> statement-breakpoint
CREATE UNIQUE INDEX "progress_movies_user_movie_uq" ON "progress_movies" USING btree ("user_id","movie_id");--> statement-breakpoint
CREATE UNIQUE INDEX "progress_shows_user_show_uq" ON "progress_shows" USING btree ("user_id","show_id");--> statement-breakpoint
CREATE UNIQUE INDEX "shows_tmdb_id_uq" ON "shows" USING btree ("tmdb_id");--> statement-breakpoint
CREATE UNIQUE INDEX "shows_tvdb_id_uq" ON "shows" USING btree ("tvdb_id");--> statement-breakpoint
CREATE UNIQUE INDEX "shows_imdb_id_uq" ON "shows" USING btree ("imdb_id");--> statement-breakpoint
CREATE UNIQUE INDEX "stats_snapshots_user_day_uq" ON "stats_snapshots" USING btree ("user_id","snapshot_day");--> statement-breakpoint
CREATE INDEX "trakt_import_runs_started_idx" ON "trakt_import_runs" USING btree ("started_at");--> statement-breakpoint
CREATE UNIQUE INDEX "watchlist_movie_user_uq" ON "watchlist_items" USING btree ("user_id","movie_id") WHERE "watchlist_items"."item_type" = 'movie';--> statement-breakpoint
CREATE UNIQUE INDEX "watchlist_show_user_uq" ON "watchlist_items" USING btree ("user_id","show_id") WHERE "watchlist_items"."item_type" = 'show';