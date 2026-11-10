import type { Episode, EpisodeWatched, Season } from './api/types';
import { hasAired, type Progress } from './watched';

// Shows, seasons, and episodes on the title pages: which seasons to list,
// how much of a show or season is watched, and which episodes open or
// close a season or the series.

/**
 * A show's seasons as the pages list them: one per number (the lowest
 * TMDB ID when two share it, as the server's marks do), regular seasons in
 * order, specials last.
 */
export function seasonList<T extends Pick<Season, 'tmdb_id' | 'season_number'>>(seasons: T[]): T[] {
	const byNumber = new Map<number, T>();
	for (const s of seasons) {
		const kept = byNumber.get(s.season_number);
		if (!kept || s.tmdb_id < kept.tmdb_id) byNumber.set(s.season_number, s);
	}
	const order = (n: number) => (n === 0 ? Infinity : n);
	return [...byNumber.values()].sort((a, b) => order(a.season_number) - order(b.season_number));
}

/**
 * Watched episodes and plays per season number, from the show's watched
 * list. Each season's total is its episode count as TMDB lists it, and the
 * watched count is capped at that total, so it never passes 100%. The cap
 * is the only limit: watches of episodes TMDB has since dropped still count
 * here (until slice 4's server-side progress), so the season grid can read
 * 100% while the season page, which counts only the episodes it lists,
 * shows less.
 */
export function progressBySeason(
	seasons: Pick<Season, 'tmdb_id' | 'season_number' | 'episode_count'>[],
	watched: Pick<EpisodeWatched, 'season_number' | 'plays'>[]
): Map<number, Progress> {
	const counts = new Map<number, { watched: number; plays: number }>();
	for (const w of watched) {
		const c = counts.get(w.season_number) ?? { watched: 0, plays: 0 };
		c.watched++;
		c.plays += w.plays;
		counts.set(w.season_number, c);
	}
	const result = new Map<number, Progress>();
	for (const s of seasonList(seasons)) {
		const c = counts.get(s.season_number) ?? { watched: 0, plays: 0 };
		const total = Math.max(s.episode_count, 0);
		result.set(s.season_number, { watched: Math.min(c.watched, total), total, plays: c.plays });
	}
	return result;
}

/**
 * A show's progress: its regular seasons only (specials are marked on
 * their own and don't count towards the show), summed.
 */
export function showProgress(
	seasons: Pick<Season, 'tmdb_id' | 'season_number' | 'episode_count'>[],
	watched: Pick<EpisodeWatched, 'season_number' | 'plays'>[]
): Progress {
	const sum: Progress = { watched: 0, total: 0, plays: 0 };
	for (const [number, p] of progressBySeason(seasons, watched)) {
		if (number === 0) continue;
		sum.watched += p.watched;
		sum.total += p.total;
		sum.plays += p.plays;
	}
	return sum;
}

/**
 * Progress as a whole percentage, rounded down, so it only says 100 when
 * everything is watched; anything watched shows at least 1.
 */
export function progressPercent(p: Pick<Progress, 'watched' | 'total'>): number {
	if (p.total <= 0 || p.watched <= 0) return 0;
	if (p.watched >= p.total) return 100;
	return Math.min(99, Math.max(1, Math.floor((p.watched / p.total) * 100)));
}

/** "76 of 133 episodes · 78 plays", the progress button's second line. */
export function progressSummary(p: Progress): string {
	const n = (v: number) => v.toLocaleString('en');
	const episodes = p.total === 1 ? 'episode' : 'episodes';
	const plays = p.plays === 1 ? '1 play' : `${n(p.plays)} plays`;
	return `${n(p.watched)} of ${n(p.total)} ${episodes} · ${plays}`;
}

export type Milestone = 'Series premiere' | 'Season premiere' | 'Season finale' | 'Series finale';

/**
 * Whether an episode opens or closes its season or the series, from what
 * TMDB lists. Kept conservative, so a badge is never wrong:
 * - Series premiere: the first episode of season 1.
 * - Season premiere: the first episode of any later season.
 * - Season finale: the last listed episode of a season, once every
 *   episode of it has aired, and only if the season can't still grow
 *   (a later season exists, or the show has ended).
 * - Series finale: that, for the last season of a show no longer in
 *   production.
 * Specials have none. A one-episode season is its premiere, unless it's
 * the series finale; a one-episode season 1 is the series premiere, even
 * when the show has ended.
 */
export function episodeMilestone(
	episode: Pick<Episode, 'season_number' | 'episode_number'>,
	season: Pick<Episode, 'episode_number' | 'air_date'>[],
	show: { lastSeason: number; inProduction: boolean },
	now: Date
): Milestone | '' {
	return milestoneIn(episode, seasonEnds(season, now), show);
}

/**
 * What episodeMilestone needs to know about a season, worked out once for
 * all its episodes (a long list asks for every row's milestone): its first
 * and last listed episode numbers, and whether all of it has aired. Null
 * for a season with no episodes.
 */
export interface SeasonEnds {
	first: number;
	last: number;
	finished: boolean;
}

export function seasonEnds(
	season: Pick<Episode, 'episode_number' | 'air_date'>[],
	now: Date
): SeasonEnds | null {
	if (season.length === 0) return null;
	let first = Infinity;
	let last = -Infinity;
	let finished = true;
	for (const e of season) {
		first = Math.min(first, e.episode_number);
		last = Math.max(last, e.episode_number);
		if (finished && !hasAired(e.air_date, now)) finished = false;
	}
	return { first, last, finished };
}

/** episodeMilestone, from its season's ends (seasonEnds): constant time per episode. */
export function milestoneIn(
	episode: Pick<Episode, 'season_number' | 'episode_number'>,
	ends: SeasonEnds | null,
	show: { lastSeason: number; inProduction: boolean }
): Milestone | '' {
	const n = episode.season_number;
	if (n <= 0 || !ends) return '';
	const isFirst = episode.episode_number === ends.first;
	const isLast = episode.episode_number === ends.last;
	const lastSeason = n >= show.lastSeason;

	if (isFirst && n === 1) return 'Series premiere';
	if (isLast && ends.finished && lastSeason && !show.inProduction) return 'Series finale';
	if (isFirst) return 'Season premiere';
	if (isLast && ends.finished && (!lastSeason || !show.inProduction)) return 'Season finale';
	return '';
}

/** The highest regular season number a show lists (0 when it has none). */
export function lastSeasonNumber(seasons: Pick<Season, 'season_number'>[]): number {
	return seasons.reduce((max, s) => Math.max(max, s.season_number), 0);
}

/**
 * The years a season aired, from its episodes' air dates: "2008", or
 * "2008–2009" across a new year; `fallback` (the season's own air date)
 * when no episode has a date yet.
 */
export function airYears(episodes: Pick<Episode, 'air_date'>[], fallback = ''): string {
	const years = episodes
		.map((e) => /^\d{4}/.exec(e.air_date)?.[0])
		.filter((y): y is string => y !== undefined)
		.sort();
	if (years.length === 0) return /^\d{4}/.exec(fallback)?.[0] ?? '';
	const first = years[0];
	const last = years[years.length - 1];
	return first === last ? first : `${first}–${last}`;
}

/** The sum of the episodes' known runtimes, in minutes (unknown ones are 0). */
export function totalRuntime(episodes: Pick<Episode, 'runtime'>[]): number {
	return episodes.reduce((sum, e) => sum + (e.runtime > 0 ? e.runtime : 0), 0);
}

/** "7 Seasons", "1 Episode": the count first, singular for one. */
export function countLabel(n: number, singular: string, plural = `${singular}s`): string {
	return `${n.toLocaleString('en')} ${n === 1 ? singular : plural}`;
}

/**
 * The seasons heading and section-list label: "7 Seasons", counting
 * regular seasons only (specials aren't a season of the show); just
 * "Seasons" when there are none but specials.
 */
export function seasonsLabel(seasons: Pick<Season, 'tmdb_id' | 'season_number'>[]): string {
	const regular = seasonList(seasons).filter((s) => s.season_number > 0).length;
	return regular > 0 ? countLabel(regular, 'Season') : 'Seasons';
}

/**
 * The items before and after the current one in a list (the hero's
 * previous and next arrows), without wrapping round; undefined at the
 * ends, or for both when none is current.
 */
export function neighbours<T>(items: T[], current: (item: T) => boolean): { prev?: T; next?: T } {
	const at = items.findIndex(current);
	if (at === -1) return {};
	return { prev: items[at - 1], next: items[at + 1] };
}

/**
 * What a season card's check offers (the show page's season grid):
 * - done: every episode is watched (a sage mark, not a button);
 * - mark: marks the aired episodes not watched yet ("rest" once some are);
 * - off: can't be used now, with the reason (its episodes are still being
 *   fetched, which the server refuses, or it hasn't started airing).
 * A season without an air date can still be marked: the server adds
 * whatever has aired, and says so when that's nothing.
 */
export type SeasonMark =
	{ kind: 'done' } | { kind: 'mark'; rest: boolean } | { kind: 'off'; reason: string };

export function seasonMark(
	season: Pick<Season, 'air_date' | 'episode_count' | 'episodes_pending'>,
	progress: Pick<Progress, 'watched' | 'total'>,
	now: Date
): SeasonMark {
	if (progress.total > 0 && progress.watched >= progress.total) return { kind: 'done' };
	if (season.episodes_pending) {
		return { kind: 'off', reason: "This season's episodes are still being fetched from TMDB." };
	}
	if (season.episode_count === 0 || (season.air_date !== '' && !hasAired(season.air_date, now))) {
		return { kind: 'off', reason: 'Nothing in this season has aired yet.' };
	}
	return { kind: 'mark', rest: progress.watched > 0 };
}

/**
 * A show's every episode in the season strip's order: regular seasons in
 * order, specials last, each by episode number (the All episodes page).
 */
export function showOrder<T extends Pick<Episode, 'season_number' | 'episode_number'>>(
	episodes: T[]
): T[] {
	const order = (n: number) => (n === 0 ? Infinity : n);
	return [...episodes].sort(
		(a, b) => order(a.season_number) - order(b.season_number) || a.episode_number - b.episode_number
	);
}

/** Episodes by their season number, in the order given. */
export function bySeason<T extends Pick<Episode, 'season_number'>>(
	episodes: T[]
): Map<number, T[]> {
	const groups = new Map<number, T[]>();
	for (const e of episodes) {
		const list = groups.get(e.season_number);
		if (list) list.push(e);
		else groups.set(e.season_number, [e]);
	}
	return groups;
}

/** Mean TMDB score of rated episodes; 0 when none have a score. */
export function seasonRating(episodes: Pick<Episode, 'vote_average'>[]): number {
	const rated = episodes.filter((e) => e.vote_average > 0);
	return rated.length ? rated.reduce((sum, e) => sum + e.vote_average, 0) / rated.length : 0;
}
