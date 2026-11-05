import { describe, expect, it } from 'vitest';
import { pickedWhen, releaseOption, toInputValue } from './when';

describe('releaseOption', () => {
	const now = new Date('2026-11-03T15:00:00Z');

	it('offers a past date', () => {
		expect(releaseOption('Release date', '1999-03-31', now)).toMatchObject({
			label: 'Release date',
			enabled: true,
			detail: expect.stringContaining('1999')
		});
	});

	it('is off without a date, or before it', () => {
		expect(releaseOption('Air date', '', now)).toMatchObject({
			enabled: false,
			detail: 'No date on TMDB'
		});
		expect(releaseOption('Release date', '2027-05-01', now)).toMatchObject({
			enabled: false,
			detail: 'Not out yet'
		});
		// Release day itself counts from noon UTC, as on the server.
		expect(releaseOption('Air date', '2026-11-03', new Date('2026-11-03T09:00:00Z')).enabled).toBe(
			false
		);
	});
});

describe('toInputValue', () => {
	it('is the local time to the minute', () => {
		expect(toInputValue(new Date(2026, 10, 3, 20, 14, 59))).toBe('2026-11-03T20:14');
		expect(toInputValue(new Date(2026, 0, 5, 7, 3))).toBe('2026-01-05T07:03');
	});
});

describe('pickedWhen', () => {
	const now = new Date(2026, 10, 3, 20, 14, 30);

	it('reads a local time', () => {
		expect(pickedWhen('2026-11-02T21:30', now)).toEqual({
			when: { at: 'time', time: new Date(2026, 10, 2, 21, 30) }
		});
		expect(pickedWhen('2026-11-02T21:30:15', now)).toEqual({
			when: { at: 'time', time: new Date(2026, 10, 2, 21, 30, 15) }
		});
	});

	it('accepts the current minute, which the picker rounds down to', () => {
		expect(pickedWhen(toInputValue(now), now)).toHaveProperty('when');
	});

	it('refuses a time in the future', () => {
		expect(pickedWhen('2026-11-03T20:16', now)).toEqual({
			error: "Pick a time that isn't in the future."
		});
	});

	it('asks for a time when the value is empty or invalid', () => {
		expect(pickedWhen('', now)).toEqual({ error: 'Pick a date and time.' });
		expect(pickedWhen('2026-02-30T10:00', now)).toEqual({ error: 'Pick a date and time.' });
		expect(pickedWhen('yesterday', now)).toEqual({ error: 'Pick a date and time.' });
	});
});
