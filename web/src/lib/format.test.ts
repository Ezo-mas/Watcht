import { describe, expect, it } from 'vitest';
import { formatDate, formatDay } from './format';

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
