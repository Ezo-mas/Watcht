import type { MarkWhen } from './api/types';
import { formatDay } from './format';
import { releaseTime } from './watched';

// The "when" choice offered by every mark: just now, the release or air
// date, or another date and time. Picked times are in the browser's time
// zone until slice 4 adds the user's setting.

/** The release-date option of the choice: whether it can be picked, and what it says. */
export interface ReleaseOption {
	/** "Release date", "Air date", "Air dates". */
	label: string;
	/** The date, or why it can't be picked. */
	detail: string;
	enabled: boolean;
}

/**
 * The option for one movie or episode with this release or air date: the
 * date itself when it's out, otherwise the reason it can't be picked.
 */
export function releaseOption(label: string, date: string, now: Date): ReleaseOption {
	const t = releaseTime(date);
	if (t === null) return { label, detail: 'No date on TMDB', enabled: false };
	if (t.getTime() > now.getTime()) return { label, detail: 'Not out yet', enabled: false };
	return { label, detail: formatDay(date), enabled: true };
}

const pad = (n: number, width = 2) => String(n).padStart(width, '0');

/** A local time as a datetime-local value ("2026-11-03T20:14"). */
export function toInputValue(d: Date): string {
	return (
		`${pad(d.getFullYear(), 4)}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}` +
		`T${pad(d.getHours())}:${pad(d.getMinutes())}`
	);
}

/**
 * Reads a datetime-local value (local time, seconds optional) into the
 * mark to send, or the message to show: empty, unreadable, or later than
 * `now`.
 */
export function pickedWhen(value: string, now: Date): { when: MarkWhen } | { error: string } {
	const m = /^(\d{4,})-(\d{2})-(\d{2})T(\d{2}):(\d{2})(?::(\d{2})(?:\.\d+)?)?$/.exec(value);
	if (!m) return { error: 'Pick a date and time.' };
	const [y, mo, d, h, mi, s] = m.slice(1).map((v) => Number(v ?? 0));
	const time = new Date(0);
	time.setFullYear(y, mo - 1, d);
	time.setHours(h, mi, s, 0);
	if (Number.isNaN(time.getTime()) || time.getDate() !== d || time.getMonth() !== mo - 1) {
		return { error: 'Pick a date and time.' };
	}
	// A minute's grace: "now" in the picker is rounded to the minute.
	if (time.getTime() > now.getTime() + 60_000) {
		return { error: "Pick a time that isn't in the future." };
	}
	return { when: { at: 'time', time } };
}

/** The browser's time zone name ("Europe/Vilnius"), shown under the picker. */
export function browserTimeZone(): string {
	return Intl.DateTimeFormat().resolvedOptions().timeZone;
}
