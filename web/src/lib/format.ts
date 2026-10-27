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
