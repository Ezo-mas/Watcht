-- Core v1 schema for watcht (PostgreSQL)
-- Source-aligned with selected Trakt export files:
-- user-profile.json, lists-watchlist.json, watched-history-*.json,
-- watched-movies.json, watched-shows.json, ratings-*.json

create extension if not exists pgcrypto;

create type media_type as enum ('movie', 'show', 'episode');
create type list_item_type as enum ('movie', 'show');

create table app_users (
  id uuid primary key default gen_random_uuid(),
  handle text not null unique,
  display_name text,
  is_private boolean not null default true,
  location text,
  bio text,
  gender text,
  joined_at timestamptz,
  trakt_user_id bigint unique,
  trakt_slug text unique,
  trakt_avatar_url text,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table movies (
  id bigserial primary key,
  title text not null,
  release_year integer,
  released_on date,
  trakt_id bigint unique,
  trakt_slug text unique,
  tmdb_id bigint,
  imdb_id text,
  language_code text,
  runtime_minutes integer,
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create unique index movies_tmdb_id_uq on movies (tmdb_id) where tmdb_id is not null;
create unique index movies_imdb_id_uq on movies (lower(imdb_id)) where imdb_id is not null;

create table shows (
  id bigserial primary key,
  title text not null,
  release_year integer,
  aired_episodes integer,
  trakt_id bigint unique,
  trakt_slug text unique,
  tmdb_id bigint,
  tvdb_id bigint,
  imdb_id text,
  status text,
  language_code text,
  runtime_minutes integer,
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create unique index shows_tmdb_id_uq on shows (tmdb_id) where tmdb_id is not null;
create unique index shows_tvdb_id_uq on shows (tvdb_id) where tvdb_id is not null;
create unique index shows_imdb_id_uq on shows (lower(imdb_id)) where imdb_id is not null;

create table episodes (
  id bigserial primary key,
  show_id bigint not null references shows(id) on delete cascade,
  season_number integer not null,
  episode_number integer not null,
  title text,
  trakt_id bigint unique,
  tmdb_id bigint,
  tvdb_id bigint,
  imdb_id text,
  first_aired_at timestamptz,
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (show_id, season_number, episode_number)
);

create unique index episodes_tmdb_id_uq on episodes (tmdb_id) where tmdb_id is not null;
create unique index episodes_tvdb_id_uq on episodes (tvdb_id) where tvdb_id is not null;
create unique index episodes_imdb_id_uq on episodes (lower(imdb_id)) where imdb_id is not null;

create table watchlist_items (
  id bigserial primary key,
  user_id uuid not null references app_users(id) on delete cascade,
  item_type list_item_type not null,
  movie_id bigint references movies(id) on delete cascade,
  show_id bigint references shows(id) on delete cascade,
  rank integer,
  listed_at timestamptz,
  source_item_id bigint,
  notes text,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  check (
    (item_type = 'movie' and movie_id is not null and show_id is null) or
    (item_type = 'show' and show_id is not null and movie_id is null)
  )
);

create unique index watchlist_movie_user_uq on watchlist_items (user_id, movie_id) where item_type = 'movie';
create unique index watchlist_show_user_uq on watchlist_items (user_id, show_id) where item_type = 'show';

create table watch_events (
  id bigserial primary key,
  user_id uuid not null references app_users(id) on delete cascade,
  media_type media_type not null,
  movie_id bigint references movies(id) on delete cascade,
  episode_id bigint references episodes(id) on delete cascade,
  watched_at timestamptz not null,
  action text not null default 'watch',
  source_history_id bigint,
  source_file text,
  raw_event jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  check (
    (media_type = 'movie' and movie_id is not null and episode_id is null) or
    (media_type = 'episode' and episode_id is not null and movie_id is null)
  )
);

create unique index watch_events_user_source_history_uq on watch_events (user_id, source_history_id) where source_history_id is not null;
create index watch_events_user_watched_at_idx on watch_events (user_id, watched_at desc);
create index watch_events_episode_idx on watch_events (episode_id) where episode_id is not null;
create index watch_events_movie_idx on watch_events (movie_id) where movie_id is not null;

create table ratings (
  id bigserial primary key,
  user_id uuid not null references app_users(id) on delete cascade,
  media_type media_type not null,
  movie_id bigint references movies(id) on delete cascade,
  show_id bigint references shows(id) on delete cascade,
  episode_id bigint references episodes(id) on delete cascade,
  rating_value smallint not null check (rating_value between 1 and 10),
  rated_at timestamptz not null,
  source_file text,
  raw_rating jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  check (
    (media_type = 'movie' and movie_id is not null and show_id is null and episode_id is null) or
    (media_type = 'show' and show_id is not null and movie_id is null and episode_id is null) or
    (media_type = 'episode' and episode_id is not null and movie_id is null and show_id is null)
  )
);

create unique index ratings_user_movie_uq on ratings (user_id, movie_id) where media_type = 'movie';
create unique index ratings_user_show_uq on ratings (user_id, show_id) where media_type = 'show';
create unique index ratings_user_episode_uq on ratings (user_id, episode_id) where media_type = 'episode';
create index ratings_user_rated_at_idx on ratings (user_id, rated_at desc);

create table progress_movies (
  id bigserial primary key,
  user_id uuid not null references app_users(id) on delete cascade,
  movie_id bigint not null references movies(id) on delete cascade,
  plays integer not null default 0,
  total_count integer,
  last_watched_at timestamptz,
  last_updated_at timestamptz,
  source_file text,
  raw_payload jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (user_id, movie_id)
);

create table progress_shows (
  id bigserial primary key,
  user_id uuid not null references app_users(id) on delete cascade,
  show_id bigint not null references shows(id) on delete cascade,
  plays integer not null default 0,
  last_watched_at timestamptz,
  last_updated_at timestamptz,
  reset_at timestamptz,
  source_file text,
  raw_payload jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (user_id, show_id)
);

create table progress_episodes (
  id bigserial primary key,
  user_id uuid not null references app_users(id) on delete cascade,
  episode_id bigint not null references episodes(id) on delete cascade,
  plays integer not null default 0,
  last_watched_at timestamptz,
  source_file text,
  raw_payload jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (user_id, episode_id)
);

create table trakt_import_runs (
  id bigserial primary key,
  user_id uuid references app_users(id) on delete set null,
  source_path text not null,
  selected_files jsonb not null default '[]'::jsonb,
  started_at timestamptz not null default now(),
  finished_at timestamptz,
  status text not null default 'running',
  notes text
);

create index trakt_import_runs_started_idx on trakt_import_runs (started_at desc);

create table stats_snapshots (
  id bigserial primary key,
  user_id uuid not null references app_users(id) on delete cascade,
  snapshot_day date not null,
  watched_count integer not null default 0,
  movies_watched_count integer not null default 0,
  episodes_watched_count integer not null default 0,
  watchlist_count integer not null default 0,
  ratings_count integer not null default 0,
  average_rating numeric(4, 2),
  total_minutes_watched integer,
  payload jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now()
);

create unique index stats_snapshots_user_day_uq on stats_snapshots (user_id, snapshot_day);
