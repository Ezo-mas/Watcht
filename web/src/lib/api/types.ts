// Types for /api/v1 responses. They mirror the Go handlers in
// internal/httpapi; change both together.

/** The standard error body: {"error": {"code": "...", "message": "..."}}. */
export interface ErrorBody {
	error: { code: string; message: string };
}

/** A signed-in user (internal/httpapi userJSON). */
export interface User {
	id: number;
	username: string;
	is_admin: boolean;
}

/** GET /api/v1/auth/session. */
export interface SessionStatus {
	setup_required: boolean;
	user: User | null;
}

/** An account in admin user management (internal/httpapi accountJSON). */
export interface Account extends User {
	/** UTC, RFC 3339. */
	created_at: string;
}

/** An API token's description; the secret is only in CreatedToken. */
export interface APIToken {
	id: number;
	name: string;
	/** UTC, RFC 3339. */
	created_at: string;
}

/** POST /api/v1/tokens: the secret is shown this once and never again. */
export interface CreatedToken {
	token: APIToken;
	secret: string;
}

export interface SearchResult {
	media_type: 'movie' | 'show';
	tmdb_id: number;
	title: string;
	original_title: string;
	overview: string;
	date: string;
	poster_path: string;
	/** 342 px wide, or "" when TMDB has no image. */
	poster_url: string;
	vote_average: number;
}

export interface SearchResults {
	page: number;
	total_pages: number;
	total_results: number;
	results: SearchResult[];
}

export interface Genre {
	tmdb_id: number;
	name: string;
}

export interface Movie {
	tmdb_id: number;
	imdb_id: string;
	title: string;
	original_title: string;
	original_language: string;
	overview: string;
	tagline: string;
	status: string;
	poster_path: string;
	/** 342 px wide, or "" when TMDB has no image. */
	poster_url: string;
	backdrop_path: string;
	/** 1280 px wide, or "" when TMDB has no image. */
	backdrop_url: string;
	release_date: string;
	runtime: number;
	genres: Genre[];
	vote_average: number;
	vote_count: number;
}

export interface Season {
	tmdb_id: number;
	season_number: number;
	name: string;
	overview: string;
	poster_path: string;
	/** 342 px wide, or "" when TMDB has no image. */
	poster_url: string;
	air_date: string;
	episode_count: number;
	episodes_pending: boolean;
}

export interface Show {
	tmdb_id: number;
	imdb_id: string;
	tvdb_id: number;
	name: string;
	original_name: string;
	original_language: string;
	overview: string;
	tagline: string;
	status: string;
	poster_path: string;
	/** 342 px wide, or "" when TMDB has no image. */
	poster_url: string;
	backdrop_path: string;
	/** 1280 px wide, or "" when TMDB has no image. */
	backdrop_url: string;
	first_air_date: string;
	last_air_date: string;
	in_production: boolean;
	genres: Genre[];
	seasons: Season[];
	vote_average: number;
	vote_count: number;
}

export interface Episode {
	tmdb_id: number;
	season_number: number;
	episode_number: number;
	name: string;
	overview: string;
	still_path: string;
	/** 300 px wide, or "" when TMDB has no image. */
	still_url: string;
	air_date: string;
	runtime: number;
	vote_average: number;
}

export interface SeasonDetails {
	show_tmdb_id: number;
	season: Season;
	episodes_pending: boolean;
	episodes: Episode[] | null;
}

/** Where a watch came from. Manual marks are the user's own. */
export type WatchSource = 'manual' | 'jellyfin' | 'plex' | 'trakt_import' | 'letterboxd_import';

/** One watch. Repeats of it merged by the dedupe rule aren't listed. */
export interface Watch {
	id: number;
	/** RFC 3339, UTC. */
	watched_at: string;
	source: WatchSource;
}

/**
 * When a mark is recorded: now (the default), a picked time, or the item's
 * release or air date (stored at noon UTC).
 */
export type MarkWhen = { at: 'now' } | { at: 'time'; time: Date } | { at: 'release' };

export interface MarkResult {
	watch: Watch;
	/** The watch this mark repeats (within 3 hours), or null for a new watch. */
	merged_into: number | null;
}

export interface EpisodeWatched {
	tmdb_id: number;
	plays: number;
	last_watched_at: string;
}

export interface HistoryMovie {
	tmdb_id: number;
	title: string;
	poster_path: string;
	poster_url: string;
	release_date: string;
	/** Minutes; 0 when TMDB has none. */
	runtime: number;
	/** TMDB's rating, 0 to 10; 0 when it has no votes. */
	vote_average: number;
}

export interface HistoryEpisode {
	tmdb_id: number;
	season_number: number;
	episode_number: number;
	name: string;
	still_path: string;
	still_url: string;
	/** YYYY-MM-DD, or empty when TMDB has none. */
	air_date: string;
	/** As for HistoryMovie. */
	runtime: number;
	vote_average: number;
	show: { tmdb_id: number; name: string; poster_path: string; poster_url: string };
}

/** A watch with what was watched: exactly one of movie and episode is set. */
export interface HistoryEntry extends Watch {
	movie: HistoryMovie | null;
	episode: HistoryEpisode | null;
}

export interface HistoryPage {
	entries: HistoryEntry[];
	/** Pass to historyPage for the next page; null on the last page. */
	next_cursor: string | null;
}
