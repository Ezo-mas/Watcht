/**
 * Formats a UTC RFC 3339 timestamp from the API as a date ("27 Oct 2026").
 * There's no per-user time zone setting until M4, so dates are shown in
 * UTC for now; M4 switches this to the user's zone.
 */
export function formatDate(rfc3339: string, locale?: string): string {
	const d = new Date(rfc3339);
	if (Number.isNaN(d.getTime())) return rfc3339;
	return new Intl.DateTimeFormat(locale, {
		day: 'numeric',
		month: 'short',
		year: 'numeric',
		timeZone: 'UTC'
	}).format(d);
}

/**
 * Formats a calendar date from TMDB ("2008-01-20", a release or air date)
 * as "20 Jan 2008". These dates have no time or zone, so they're shown as
 * they are, never shifted into a time zone. Anything else is returned
 * unchanged, and "" stays "".
 */
export function formatDay(date: string, locale?: string): string {
	const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(date);
	if (!m) return date;
	const d = new Date(Date.UTC(Number(m[1]), Number(m[2]) - 1, Number(m[3])));
	if (Number.isNaN(d.getTime()) || d.getUTCDate() !== Number(m[3])) return date;
	return new Intl.DateTimeFormat(locale, {
		day: 'numeric',
		month: 'short',
		year: 'numeric',
		timeZone: 'UTC'
	}).format(d);
}

// Watch times. The tracking pages show them in the browser's time zone
// until slice 4 adds the user's setting; pass `timeZone` then. Undefined
// means the browser's zone.

/** A watch time as date and time ("3 Nov 2026, 20:14"). */
export function formatDateTime(rfc3339: string, timeZone?: string, locale?: string): string {
	const d = new Date(rfc3339);
	if (Number.isNaN(d.getTime())) return rfc3339;
	return new Intl.DateTimeFormat(locale, {
		day: 'numeric',
		month: 'short',
		year: 'numeric',
		hour: '2-digit',
		minute: '2-digit',
		timeZone
	}).format(d);
}

/** The time fields of a watch (see Watch in api/types). */
interface WatchTime {
	watched_at: string;
	watched_on: string | null;
}

/**
 * A watch as the pages show it: the date alone for a watch with no time
 * ("20 Jan 2008", the same in every zone), else date and time in the zone.
 */
export function formatWatch(w: WatchTime, timeZone?: string, locale?: string): string {
	return w.watched_on
		? formatDay(w.watched_on, locale)
		: formatDateTime(w.watched_at, timeZone, locale);
}

/** A watch's day as a date ("19 Aug 2026"): its date, or the day of its time in the zone. */
export function formatWatchDay(w: WatchTime, timeZone?: string, locale?: string): string {
	return w.watched_on
		? formatDay(w.watched_on, locale)
		: formatWatchDate(w.watched_at, timeZone, locale);
}

/** A watch's calendar day, YYYY-MM-DD: its date, or the day of its time in the zone. */
export function watchDay(w: WatchTime, timeZone?: string): string {
	return w.watched_on ?? dayKey(new Date(w.watched_at), timeZone);
}

/** A watch time's calendar day ("19 Aug 2026"). */
export function formatWatchDate(rfc3339: string, timeZone?: string, locale?: string): string {
	const d = new Date(rfc3339);
	if (Number.isNaN(d.getTime())) return rfc3339;
	return new Intl.DateTimeFormat(locale, {
		day: 'numeric',
		month: 'short',
		year: 'numeric',
		timeZone
	}).format(d);
}

/** A watch time's time of day ("20:14"). */
export function formatTime(rfc3339: string, timeZone?: string, locale?: string): string {
	const d = new Date(rfc3339);
	if (Number.isNaN(d.getTime())) return rfc3339;
	return new Intl.DateTimeFormat(locale, { hour: '2-digit', minute: '2-digit', timeZone }).format(
		d
	);
}

/** The calendar day of an instant in a time zone, as YYYY-MM-DD. */
export function dayKey(date: Date, timeZone?: string): string {
	const parts = new Intl.DateTimeFormat('en-US', {
		year: 'numeric',
		month: '2-digit',
		day: '2-digit',
		timeZone
	}).formatToParts(date);
	const get = (type: string) => parts.find((p) => p.type === type)?.value ?? '';
	return `${get('year').padStart(4, '0')}-${get('month')}-${get('day')}`;
}

/** The day before a YYYY-MM-DD day. */
export function previousDay(key: string): string {
	const [y, m, d] = key.split('-').map(Number);
	// setUTCFullYear, unlike Date.UTC, keeps years below 100 as they are.
	const prev = new Date(0);
	prev.setUTCFullYear(y, m - 1, d - 1);
	return prev.toISOString().slice(0, 10);
}
