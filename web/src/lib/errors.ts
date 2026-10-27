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
