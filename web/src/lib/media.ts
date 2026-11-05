import type { Season } from './api/types';

// Small helpers for showing catalog data: years, runtimes, episode codes,
// languages, and which season a show page opens on.

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
 * The season a show page opens on: the first regular season (1 or more),
 * or specials when that's all there is; null for a show with no seasons.
 */
export function defaultSeason(seasons: Pick<Season, 'season_number'>[]): number | null {
	let first: number | null = null;
	for (const s of seasons) {
		if (s.season_number >= 1 && (first === null || s.season_number < first)) {
			first = s.season_number;
		}
	}
	if (first !== null) return first;
	return seasons.some((s) => s.season_number === 0) ? 0 : null;
}

/**
 * Reads ?season=N: a whole number 0 or more, or null for anything else
 * (missing, "abc", "-1", "1.5").
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
