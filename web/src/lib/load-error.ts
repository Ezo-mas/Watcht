import { error } from '@sveltejs/kit';
import { ApiError } from './api/client';
import { catalogMessages, errorMessage } from './errors';

/**
 * Turns a failed API call in a page load into the error page's status and
 * message ((app)/+error.svelte). Without this an ApiError reaches the
 * error page as a bare 500 "Internal Error". A server that can't be
 * reached (status 0) is a 503; anything else keeps its status, with the
 * message from `messages` or the server's own.
 */
export function loadError(e: unknown, messages: Partial<Record<string, string>> = {}): never {
	if (e instanceof ApiError) {
		const status = e.status >= 400 && e.status <= 599 ? e.status : 503;
		error(status, errorMessage(e, messages));
	}
	throw e;
}

/** loadError for the catalog: 404 says what's missing (`notFound`). */
export function catalogLoadError(e: unknown, notFound: string): never {
	if (e instanceof ApiError && e.status === 404) error(404, notFound);
	loadError(e, catalogMessages);
}

/**
 * For a part of a page that can fail on its own (the user's watches on a
 * title page): the value, or null and the message to show in its place,
 * so the rest of the page still loads.
 */
export async function optionalLoad<T>(
	request: Promise<T>
): Promise<{ value: T; error: '' } | { value: null; error: string }> {
	try {
		return { value: await request, error: '' };
	} catch (e) {
		return { value: null, error: errorMessage(e) };
	}
}
