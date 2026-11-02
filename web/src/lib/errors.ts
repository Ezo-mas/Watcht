import { ApiError } from './api/client';

/**
 * The message to show for a failed API call. `messages` replaces the
 * server's text for the codes a form expects; any other ApiError shows the
 * server's own message, and anything else (a bug) a generic one.
 */
export function errorMessage(e: unknown, messages: Partial<Record<string, string>> = {}): string {
	if (e instanceof ApiError) return messages[e.code] ?? e.message;
	return 'Something went wrong.';
}

/** Messages for the catalog endpoints' error codes (search and title pages). */
export const catalogMessages: Partial<Record<string, string>> = {
	tmdb_not_configured:
		"watcht can't look up titles yet: an admin needs to set WATCHT_TMDB_API_KEY on the server and restart it.",
	provider_unavailable: "TMDB didn't answer. Try again in a moment.",
	not_found: "TMDB doesn't have this title.",
	invalid_query: 'Search for 100 characters or fewer.'
};
