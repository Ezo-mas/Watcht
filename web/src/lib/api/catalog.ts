import { api } from './client';
import type { Movie, SearchResults, SeasonDetails, Show, ShowEpisodes } from './types';

export function search(q: string, page?: number, fetchFn?: typeof fetch): Promise<SearchResults> {
	const query = new URLSearchParams({ q });
	if (page !== undefined) query.set('page', String(page));
	return api<SearchResults>('GET', `/api/v1/search?${query}`, undefined, fetchFn);
}

export async function getMovie(id: number, fetchFn?: typeof fetch): Promise<Movie> {
	return (await api<{ movie: Movie }>('GET', `/api/v1/movies/${id}`, undefined, fetchFn)).movie;
}

export async function getShow(id: number, fetchFn?: typeof fetch): Promise<Show> {
	return (await api<{ show: Show }>('GET', `/api/v1/shows/${id}`, undefined, fetchFn)).show;
}

export function getSeason(
	showId: number,
	number: number,
	fetchFn?: typeof fetch
): Promise<SeasonDetails> {
	return api<SeasonDetails>('GET', `/api/v1/shows/${showId}/seasons/${number}`, undefined, fetchFn);
}

/** Every stored episode of a show, specials included, by season then number. */
export function getShowEpisodes(showId: number, fetchFn?: typeof fetch): Promise<ShowEpisodes> {
	return api<ShowEpisodes>('GET', `/api/v1/shows/${showId}/episodes`, undefined, fetchFn);
}
