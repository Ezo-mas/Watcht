import { getSeason, getShowEpisodes } from './api/catalog';
import type { SeasonDetails, ShowEpisodes } from './api/types';
import { poll } from './poll';

/**
 * Asks for a season again every 3 seconds, for up to 2 minutes, while the
 * server is still fetching its episodes from TMDB (episodes_pending): the
 * season and episode pages use it when their load found it pending.
 * Returns the function that cancels it (leaving the page, another season).
 */
export function pollSeason(
	showId: number,
	number: number,
	handlers: {
		/** Every answer, pending or not. */
		onValue: (season: SeasonDetails) => void;
		onTimeout: () => void;
		onError: (error: unknown) => void;
	}
): () => void {
	return poll<SeasonDetails>({
		load: (signal) =>
			getSeason(showId, number, ((input, init) =>
				fetch(input, { ...init, signal })) as typeof fetch),
		done: (d) => !d.episodes_pending,
		...handlers
	});
}

/**
 * pollSeason for a show's every episode (the All episodes page): asks
 * again while any of its seasons is still being fetched.
 */
export function pollShowEpisodes(
	showId: number,
	handlers: {
		onValue: (episodes: ShowEpisodes) => void;
		onTimeout: () => void;
		onError: (error: unknown) => void;
	}
): () => void {
	return poll<ShowEpisodes>({
		load: (signal) =>
			getShowEpisodes(showId, ((input, init) => fetch(input, { ...init, signal })) as typeof fetch),
		done: (d) => !d.episodes_pending,
		...handlers
	});
}
