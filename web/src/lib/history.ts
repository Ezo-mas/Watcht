import type { HistoryEntry } from './api/types';
import { watchDay } from './format';
import { formatRuntime } from './media';

// The /history page: entries grouped under day headings, each with the
// day's watch time.

export interface HistoryDay {
	/** The calendar day in the time zone, YYYY-MM-DD. */
	key: string;
	entries: HistoryEntry[];
}

/**
 * Groups entries (newest first, as the API sends them) by their day: the
 * date of a watch with no time, else the calendar day of its watch time in
 * `timeZone` (the browser's when undefined). Days are newest first, entries
 * keep their order, and a day split across two pages becomes one group.
 *
 * Days are merged by key, not only when next to each other: a date-only
 * watch sorts at noon UTC, so east of UTC+12 a timed watch just before it
 * can fall on the next local day, and the same day can come up twice.
 */
export function groupByDay(entries: HistoryEntry[], timeZone?: string): HistoryDay[] {
	const byKey = new Map<string, HistoryDay>();
	for (const entry of entries) {
		const key = watchDay(entry, timeZone);
		const day = byKey.get(key);
		if (day) day.entries.push(entry);
		else byKey.set(key, { key, entries: [entry] });
	}
	return [...byKey.values()].sort((a, b) => (a.key < b.key ? 1 : a.key > b.key ? -1 : 0));
}

/**
 * A day heading's two parts, the weekday and the full date with the year
 * ("Tuesday", "3 November 2026" in en-GB), shown bold then regular. Always
 * the plain date, never "Today", as classic Trakt's history does.
 */
export function dayHeading(key: string, locale?: string): { weekday: string; date: string } {
	const [y, m, d] = key.split('-').map(Number);
	const date = new Date(0);
	date.setUTCFullYear(y, m - 1, d);
	const format = (options: Intl.DateTimeFormatOptions) =>
		new Intl.DateTimeFormat(locale, { ...options, timeZone: 'UTC' }).format(date);
	return {
		weekday: format({ weekday: 'long' }),
		date: format({ day: 'numeric', month: 'long', year: 'numeric' })
	};
}

/**
 * A day's total watch time, from the runtimes of its loaded entries.
 * `minutes` sums the known runtimes. `text` is what the heading shows: the
 * time ("45m", "1h 45m", "2h"), or the number of watches when no runtime
 * is known. `label` says the same in
 * full, including how many have no runtime, for the tooltip and screen
 * readers ("1h 45m watched, 3 watches, runtime unknown for 1").
 */
export function dayTotal(entries: HistoryEntry[]): {
	minutes: number;
	text: string;
	label: string;
} {
	let minutes = 0;
	let unknown = 0;
	for (const e of entries) {
		const runtime = e.episode?.runtime ?? e.movie?.runtime ?? 0;
		if (runtime > 0) minutes += runtime;
		else unknown++;
	}
	const watches = `${entries.length} ${entries.length === 1 ? 'watch' : 'watches'}`;
	if (minutes === 0) {
		return { minutes, text: watches, label: `${watches}, runtime unknown` };
	}
	const time = formatRuntime(minutes);
	const parts = [`${time} watched`, watches];
	if (unknown > 0) parts.push(`runtime unknown for ${unknown}`);
	return { minutes, text: time, label: parts.join(', ') };
}

/** Whether `a` comes before `b` in history order: newer first, then the higher id. */
function newer(a: HistoryEntry, b: HistoryEntry): boolean {
	const ta = Date.parse(a.watched_at);
	const tb = Date.parse(b.watched_at);
	return ta !== tb ? ta > tb : a.id > b.id;
}

/**
 * Puts a new watch into the loaded entries at its place in history order
 * (the API's: watched_at, then id, newest first). One older than
 * everything loaded is left out while more pages remain (`more`), since it
 * arrives with them; `inserted` says whether it's in the list.
 */
export function insertEntry(
	entries: HistoryEntry[],
	entry: HistoryEntry,
	more: boolean
): { entries: HistoryEntry[]; inserted: boolean } {
	if (entries.some((e) => e.id === entry.id)) return { entries, inserted: true };
	const at = entries.findIndex((e) => newer(entry, e));
	if (at === -1) {
		return more ? { entries, inserted: false } : { entries: [...entries, entry], inserted: true };
	}
	return { entries: [...entries.slice(0, at), entry, ...entries.slice(at)], inserted: true };
}
