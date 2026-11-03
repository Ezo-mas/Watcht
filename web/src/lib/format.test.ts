import { describe, expect, it } from 'vitest';
import {
	dayKey,
	formatDate,
	formatDateTime,
	formatDay,
	formatTime,
	formatWatchDate,
	previousDay
} from './format';

describe('formatDate', () => {
	it('formats the date in UTC', () => {
		expect(formatDate('2026-10-27T10:00:00Z', 'en-GB')).toBe('27 Oct 2026');
		// Late on the 27th UTC is still the 27th, whatever the machine's zone.
		expect(formatDate('2026-10-27T23:59:59Z', 'en-GB')).toBe('27 Oct 2026');
	});

	it('returns anything unparseable as is', () => {
		expect(formatDate('not a date')).toBe('not a date');
	});
});

describe('formatDay', () => {
	it('formats a calendar date as it is', () => {
		expect(formatDay('2008-01-20', 'en-GB')).toBe('20 Jan 2008');
		expect(formatDay('1999-12-31', 'en-GB')).toBe('31 Dec 1999');
	});

	it('returns empty and invalid dates as they are', () => {
		expect(formatDay('')).toBe('');
		expect(formatDay('2008-02-30')).toBe('2008-02-30');
		expect(formatDay('20 Jan 2008')).toBe('20 Jan 2008');
	});
});

describe('watch times', () => {
	it('formats a time in the given zone', () => {
		expect(formatDateTime('2026-11-03T18:14:00Z', 'Europe/Vilnius', 'en-GB')).toBe(
			'3 Nov 2026, 20:14'
		);
		expect(formatTime('2026-11-03T18:14:00Z', 'Europe/Vilnius', 'en-GB')).toBe('20:14');
		expect(formatTime('2026-11-03T18:14:00Z', 'UTC', 'en-GB')).toBe('18:14');
	});

	it("gives a watch's calendar day in the zone", () => {
		expect(formatWatchDate('2026-08-19T20:00:00Z', 'UTC', 'en-GB')).toBe('19 Aug 2026');
		// 23:30 UTC is already the next day in Vilnius.
		expect(formatWatchDate('2026-08-19T23:30:00Z', 'Europe/Vilnius', 'en-GB')).toBe('20 Aug 2026');
	});

	it('returns anything unparseable as is', () => {
		expect(formatDateTime('nope')).toBe('nope');
		expect(formatTime('nope')).toBe('nope');
		expect(formatWatchDate('nope')).toBe('nope');
	});
});

describe('dayKey', () => {
	it('is the calendar day in the zone', () => {
		const late = new Date('2026-11-03T23:30:00Z');
		expect(dayKey(late, 'UTC')).toBe('2026-11-03');
		expect(dayKey(late, 'Europe/Vilnius')).toBe('2026-11-04');
		expect(dayKey(late, 'America/Los_Angeles')).toBe('2026-11-03');
		expect(dayKey(new Date('2026-11-03T05:00:00Z'), 'America/Los_Angeles')).toBe('2026-11-02');
	});
});

describe('dayKey across a daylight saving change', () => {
	it('New York, where clocks went back at 06:00 UTC on 1 Nov 2026', () => {
		const zone = 'America/New_York';
		// 23:30 EDT on the 31st.
		expect(dayKey(new Date('2026-11-01T03:30:00Z'), zone)).toBe('2026-10-31');
		// 00:30 EDT, before the change, and 01:30 EST, after it: both the 1st.
		expect(dayKey(new Date('2026-11-01T04:30:00Z'), zone)).toBe('2026-11-01');
		expect(dayKey(new Date('2026-11-01T06:30:00Z'), zone)).toBe('2026-11-01');
		// 23:59 EST still the 1st; 00:00 EST the 2nd (UTC-5 now).
		expect(dayKey(new Date('2026-11-02T04:59:00Z'), zone)).toBe('2026-11-01');
		expect(dayKey(new Date('2026-11-02T05:00:00Z'), zone)).toBe('2026-11-02');
	});

	it('Vilnius, where clocks went back at 01:00 UTC on 25 Oct 2026', () => {
		const zone = 'Europe/Vilnius';
		// 23:59 EEST on the 24th, then 00:00 EEST on the 25th.
		expect(dayKey(new Date('2026-10-24T20:59:00Z'), zone)).toBe('2026-10-24');
		expect(dayKey(new Date('2026-10-24T21:00:00Z'), zone)).toBe('2026-10-25');
		// After the change it's UTC+2: 23:59 EET is still the 25th.
		expect(dayKey(new Date('2026-10-25T21:59:00Z'), zone)).toBe('2026-10-25');
		expect(dayKey(new Date('2026-10-25T22:00:00Z'), zone)).toBe('2026-10-26');
	});
});

describe('previousDay', () => {
	it('steps back across months, years, and leap days', () => {
		expect(previousDay('2026-11-03')).toBe('2026-11-02');
		expect(previousDay('2026-03-01')).toBe('2026-02-28');
		expect(previousDay('2024-03-01')).toBe('2024-02-29');
		expect(previousDay('2026-01-01')).toBe('2025-12-31');
	});

	it('keeps years below 100', () => {
		expect(previousDay('0050-01-01')).toBe('0049-12-31');
	});
});
