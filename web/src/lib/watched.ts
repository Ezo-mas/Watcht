import type { Episode, EpisodeWatched, WatchSource } from './api/types';

// Watched state on the title pages: what can be marked, and how much of a
// season is watched.

/**
 * The moment a TMDB release or air date ("2008-01-20") counts as out: noon
 * UTC that day, the same rule the server uses (tracking.ReleaseTime), so
 * the page never offers a mark the server refuses. Null for no or an
 * invalid date.
 */
export function releaseTime(date: string): Date | null {
	const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(date);
	if (!m) return null;
	const t = new Date(0);
	t.setUTCFullYear(Number(m[1]), Number(m[2]) - 1, Number(m[3]));
	if (t.getUTCDate() !== Number(m[3])) return null;
	t.setUTCHours(12);
	return t;
}

/** Whether a release or air date has passed (see releaseTime). */
export function hasAired(date: string, now: Date): boolean {
	const t = releaseTime(date);
	return t !== null && t.getTime() <= now.getTime();
}

/** The user's watched episodes of a show, by TMDB episode ID. */
export function watchedById(list: EpisodeWatched[]): Map<number, EpisodeWatched> {
	return new Map(list.map((e) => [e.tmdb_id, e]));
}

/**
 * How many of a season's listed episodes are watched. Watches of episodes
 * TMDB no longer lists for it don't count.
 */
export function seasonProgress(
	episodes: Pick<Episode, 'tmdb_id'>[],
	watched: Map<number, EpisodeWatched>
): { watched: number; total: number } {
	return {
		watched: episodes.filter((e) => watched.has(e.tmdb_id)).length,
		total: episodes.length
	};
}

/** Whether a season has any aired episode, so "Mark season watched" can add something. */
export function anyAired(episodes: Pick<Episode, 'air_date'>[], now: Date): boolean {
	return episodes.some((e) => hasAired(e.air_date, now));
}

const sources: Record<WatchSource, string> = {
	manual: '',
	jellyfin: 'Jellyfin',
	plex: 'Plex',
	trakt_import: 'Trakt',
	letterboxd_import: 'Letterboxd'
};

/** The badge for where a watch came from; "" for the user's own marks. */
export function sourceLabel(source: WatchSource): string {
	return sources[source] ?? source;
}

/** "1 play", "3 plays". */
export function playsLabel(n: number): string {
	return n === 1 ? '1 play' : `${n} plays`;
}

/**
 * The result of a season or show mark: "Added 8 episodes to your
 * history.", or why nothing was added. `anyAired` says whether anything
 * had aired, as far as the page knows; season and show marks skip unaired
 * episodes.
 */
export function addedMessage(added: number, anyAired: boolean): string {
	if (added === 0) {
		return anyAired
			? 'Nothing added: every aired episode is already watched.'
			: 'Nothing added: nothing has aired yet.';
	}
	return `Added ${added} ${added === 1 ? 'episode' : 'episodes'} to your history.`;
}
