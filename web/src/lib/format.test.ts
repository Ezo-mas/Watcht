import { describe, expect, it } from 'vitest';
import { formatDate } from './format';

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
