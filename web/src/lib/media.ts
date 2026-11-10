// Small helpers for showing catalog data: years, runtimes, episode codes,
// languages, and links to other sites.

/** The year of a TMDB date ("2008-01-20" → "2008"), or "" when there's none. */
export function yearOf(date: string): string {
	return /^\d{4}/.test(date) ? date.slice(0, 4) : '';
}

/** A runtime in minutes as "58m" or "2h 16m"; "" when unknown (0). */
export function formatRuntime(minutes: number): string {
	if (!Number.isFinite(minutes) || minutes <= 0) return '';
	const h = Math.floor(minutes / 60);
	const m = Math.round(minutes % 60);
	if (h === 0) return `${m}m`;
	return m === 0 ? `${h}h` : `${h}h ${m}m`;
}

/** The "S01E02" episode code (maintainer decision, 2026-11-03). */
export function episodeCode(season: number, episode: number): string {
	return `S${String(season).padStart(2, '0')}E${String(episode).padStart(2, '0')}`;
}

/** A TMDB vote average (0–10) to one decimal, or "" when there are no votes. */
export function formatScore(average: number, count: number): string {
	if (!(count > 0) || !(average > 0)) return '';
	return average.toFixed(1);
}

/**
 * A TMDB vote average (0–10) as a percentage ("67%"), as the poster cards'
 * heart shows it; "" when there are no votes (0).
 */
export function tmdbPercent(average: number): string {
	if (!Number.isFinite(average) || average <= 0) return '';
	return `${Math.round(average * 10)}%`;
}

/**
 * The English name of an ISO 639-1 code ("en" → "English"); the code itself
 * when the browser doesn't know it, "" for none.
 */
export function languageName(code: string, locale = 'en'): string {
	if (!code) return '';
	try {
		return new Intl.DisplayNames([locale], { type: 'language' }).of(code) ?? code;
	} catch {
		return code;
	}
}

/** "Specials" for season 0, otherwise "Season N". */
export function seasonLabel(number: number): string {
	return number === 0 ? 'Specials' : `Season ${number}`;
}

/**
 * Reads a season or episode number (?season=N, or the number in a season
 * or episode page's address): a whole number 0 or more, or null for
 * anything else (missing, "abc", "-1", "1.5").
 */
export function parseSeasonParam(value: string | null): number | null {
	if (value === null || !/^\d{1,4}$/.test(value)) return null;
	return Number(value);
}

/** A whole TMDB ID (1 or more) from a route parameter, or null. */
export function parseId(value: string): number | null {
	if (!/^[1-9]\d{0,9}$/.test(value)) return null;
	const n = Number(value);
	return n <= 2147483647 ? n : null;
}

/** A page about a title on another site, shown as an icon link (ExternalLinks). */
export interface TitleLink {
	site: 'imdb' | 'tmdb';
	/** The site's name, for the link's accessible name and tooltip. */
	label: string;
	href: string;
}

/**
 * The title's pages on IMDb (when TMDB knows its IMDb ID) and TMDB, in
 * that order.
 */
export function titleLinks(kind: 'movie' | 'tv', tmdbId: number, imdbId: string): TitleLink[] {
	const links: TitleLink[] = [];
	if (imdbId) {
		links.push({
			site: 'imdb',
			label: 'IMDb',
			href: `https://www.imdb.com/title/${encodeURIComponent(imdbId)}/`
		});
	}
	links.push({ site: 'tmdb', label: 'TMDB', href: `https://www.themoviedb.org/${kind}/${tmdbId}` });
	return links;
}

/**
 * A season's or an episode's page on TMDB, as the icon link under its
 * poster. IMDb has no page for a season, and the API carries no IMDb ID
 * for an episode, so TMDB only.
 */
export function tvPartLinks(showId: number, season: number, episode?: number): TitleLink[] {
	let href = `https://www.themoviedb.org/tv/${showId}/season/${season}`;
	if (episode !== undefined) href += `/episode/${episode}`;
	return [{ site: 'tmdb', label: 'TMDB', href }];
}
