import { resolve } from '$app/paths';
import type { Episode, Show } from './api/types';
import type { PickerItem } from './components/PickerStrip.svelte';
import type { HeroLink } from './components/TitlePage.svelte';
import { episodeCode, seasonLabel } from './media';
import { neighbours, seasonList } from './seasons';

// Moving between a show's seasons and episodes: the season strip (with
// "All" at its end) and the arrows on the hero of season, All episodes,
// and episode pages.

/** The All episodes page's key in the season strip (season numbers are 0 or more). */
export const ALL = -1;

/**
 * The season strip: 1, 2, …, Specials, then All (every episode), with
 * `current` (a season number, or ALL) marked.
 */
export function seasonPicker(
	show: Pick<Show, 'tmdb_id' | 'seasons'>,
	current: number
): PickerItem[] {
	const id = String(show.tmdb_id);
	const items: PickerItem[] = seasonList(show.seasons).map((s) => ({
		key: s.season_number,
		href: resolve('/(app)/shows/[id]/seasons/[n]', { id, n: String(s.season_number) }),
		label: s.season_number === 0 ? 'Specials' : String(s.season_number),
		prefix: s.season_number === 0 ? undefined : 'Season',
		title: s.episodes_pending
			? 'Episodes still loading from TMDB'
			: `${s.episode_count} ${s.episode_count === 1 ? 'episode' : 'episodes'}`,
		current: s.season_number === current,
		pending: s.episodes_pending
	}));
	items.push({
		key: ALL,
		href: resolve('/(app)/shows/[id]/seasons/all', { id }),
		label: 'All',
		name: 'All episodes',
		title: 'Every episode, specials included',
		current: current === ALL
	});
	return items;
}

/**
 * The hero's arrows on a season or the All episodes page: the seasons
 * before and after it in the strip's order (specials after the last
 * season, then All), with no wrapping round.
 */
export function seasonArrows(
	show: Pick<Show, 'tmdb_id' | 'seasons'>,
	current: number
): { prev?: HeroLink; next?: HeroLink } {
	const names = new Map(
		seasonList(show.seasons).map((s) => [s.season_number, s.name || seasonLabel(s.season_number)])
	);
	const { prev, next } = neighbours(seasonPicker(show, current), (i) => i.current);
	const link = (item: PickerItem | undefined, dir: 'Previous' | 'Next') => {
		if (!item) return undefined;
		const label =
			item.key === ALL ? `${dir}: All episodes` : `${dir} season: ${names.get(item.key) ?? ''}`;
		return { href: item.href, label };
	};
	return { prev: link(prev, 'Previous'), next: link(next, 'Next') };
}

/**
 * The hero's arrows on an episode page: the episodes before and after it
 * in its season, by the season's list order. They stop at the season's
 * ends: the next season's episode numbers aren't known without asking
 * for that season.
 */
export function episodeArrows(
	showId: number,
	episodes: Pick<Episode, 'tmdb_id' | 'season_number' | 'episode_number' | 'name'>[],
	currentId: number
): { prev?: HeroLink; next?: HeroLink } {
	const { prev, next } = neighbours(episodes, (e) => e.tmdb_id === currentId);
	const link = (e: (typeof episodes)[number] | undefined, dir: 'Previous' | 'Next') =>
		e && {
			href: resolve('/(app)/shows/[id]/seasons/[n]/episodes/[e]', {
				id: String(showId),
				n: String(e.season_number),
				e: String(e.episode_number)
			}),
			label: `${dir} episode: ${episodeCode(e.season_number, e.episode_number)} ${e.name}`.trim()
		};
	return { prev: link(prev, 'Previous'), next: link(next, 'Next') };
}
