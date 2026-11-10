import { api } from './client';
import type { EpisodeWatched, HistoryPage, MarkResult, MarkWhen, Watch } from './types';

/** The request body for a mark (internal/httpapi/tracking.go). */
function markBody(when: MarkWhen): Record<string, unknown> {
	switch (when.at) {
		case 'now':
			return {};
		case 'time':
			return { watched_at: when.time.toISOString() };
		case 'release':
			return { release_date: true };
	}
}

export function markMovie(
	tmdbId: number,
	when: MarkWhen,
	fetchFn?: typeof fetch
): Promise<MarkResult> {
	return api<MarkResult>('POST', `/api/v1/movies/${tmdbId}/watches`, markBody(when), fetchFn);
}

/** Marks an episode by its TMDB episode ID. */
export function markEpisode(
	tmdbId: number,
	when: MarkWhen,
	fetchFn?: typeof fetch
): Promise<MarkResult> {
	return api<MarkResult>('POST', `/api/v1/episodes/${tmdbId}/watches`, markBody(when), fetchFn);
}

/** Marks every aired, unwatched episode of a season; resolves with how many. */
export async function markSeason(
	showId: number,
	seasonNumber: number,
	when: MarkWhen,
	fetchFn?: typeof fetch
): Promise<number> {
	const path = `/api/v1/shows/${showId}/seasons/${seasonNumber}/watches`;
	return (await api<{ added: number }>('POST', path, markBody(when), fetchFn)).added;
}

/** Marks every aired, unwatched episode of a show except specials. */
export async function markShow(
	showId: number,
	when: MarkWhen,
	fetchFn?: typeof fetch
): Promise<number> {
	const path = `/api/v1/shows/${showId}/watches`;
	return (await api<{ added: number }>('POST', path, markBody(when), fetchFn)).added;
}

export function removeMovieWatches(
	tmdbId: number,
	fetchFn?: typeof fetch
): Promise<{ removed: number }> {
	return api('DELETE', `/api/v1/movies/${tmdbId}/watches`, undefined, fetchFn);
}

export function removeEpisodeWatches(
	tmdbId: number,
	fetchFn?: typeof fetch
): Promise<{ removed: number }> {
	return api('DELETE', `/api/v1/episodes/${tmdbId}/watches`, undefined, fetchFn);
}

export function removeSeasonWatches(
	showId: number,
	seasonNumber: number,
	fetchFn?: typeof fetch
): Promise<{ removed: number }> {
	return api(
		'DELETE',
		`/api/v1/shows/${showId}/seasons/${seasonNumber}/watches`,
		undefined,
		fetchFn
	);
}

export function removeShowWatches(
	showId: number,
	fetchFn?: typeof fetch
): Promise<{ removed: number }> {
	return api('DELETE', `/api/v1/shows/${showId}/watches`, undefined, fetchFn);
}

/** Removes a watch, and every repeat merged into it. */
export function removeWatch(id: number, fetchFn?: typeof fetch): Promise<void> {
	return api<void>('DELETE', `/api/v1/watches/${id}`, undefined, fetchFn);
}

export async function movieWatches(tmdbId: number, fetchFn?: typeof fetch): Promise<Watch[]> {
	return (
		await api<{ watches: Watch[] }>('GET', `/api/v1/movies/${tmdbId}/watches`, undefined, fetchFn)
	).watches;
}

export async function episodeWatches(tmdbId: number, fetchFn?: typeof fetch): Promise<Watch[]> {
	const path = `/api/v1/episodes/${tmdbId}/watches`;
	return (await api<{ watches: Watch[] }>('GET', path, undefined, fetchFn)).watches;
}

export async function showWatched(
	showId: number,
	fetchFn?: typeof fetch
): Promise<EpisodeWatched[]> {
	const path = `/api/v1/shows/${showId}/watched`;
	return (await api<{ episodes: EpisodeWatched[] }>('GET', path, undefined, fetchFn)).episodes;
}

/** A page of history, newest first. Omit cursor for the first page. */
export function historyPage(
	cursor?: string,
	limit?: number,
	fetchFn?: typeof fetch
): Promise<HistoryPage> {
	const query = new URLSearchParams();
	if (cursor !== undefined) query.set('cursor', cursor);
	if (limit !== undefined) query.set('limit', String(limit));
	const qs = query.size ? `?${query}` : '';
	return api<HistoryPage>('GET', `/api/v1/history${qs}`, undefined, fetchFn);
}
