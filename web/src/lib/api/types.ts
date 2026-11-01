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
	backdrop_path: string;
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
	backdrop_path: string;
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
