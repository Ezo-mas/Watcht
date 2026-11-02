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
