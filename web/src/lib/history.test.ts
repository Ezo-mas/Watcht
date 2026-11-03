import { describe, expect, it } from 'vitest';
import type { HistoryEntry } from './api/types';
import { dayHeading, dayTotal, groupByDay, insertEntry } from './history';

const entry = (id: number, watched_at: string): HistoryEntry => ({
	id,
	watched_at,
	source: 'manual',
	movie: null,
	episode: null
});

describe('groupByDay', () => {
	// Newest first, as the API sends them.
	const entries = [
		entry(4, '2026-11-03T23:30:00Z'),
		entry(3, '2026-11-03T20:00:00Z'),
		entry(2, '2026-11-02T22:30:00Z'),
		entry(1, '2026-11-01T10:00:00Z')
	];

	it('groups by the calendar day in the time zone, keeping the order', () => {
		const days = groupByDay(entries, 'UTC');
		expect(days.map((d) => [d.key, d.entries.map((e) => e.id)])).toEqual([
			['2026-11-03', [4, 3]],
			['2026-11-02', [2]],
			['2026-11-01', [1]]
		]);
	});

	it('moves late evening UTC to the next day east of UTC', () => {
		const days = groupByDay(entries, 'Europe/Vilnius');
		expect(days.map((d) => [d.key, d.entries.map((e) => e.id)])).toEqual([
			['2026-11-04', [4]],
			['2026-11-03', [3, 2]],
			['2026-11-01', [1]]
		]);
	});

	it('groups across a daylight saving change by the local day', () => {
		// New York: 1 Nov 2026 runs 04:00Z–05:00Z next day (25 hours).
		const ny = [
			entry(5, '2026-11-02T05:00:00Z'), // 00:00 EST on the 2nd
			entry(4, '2026-11-02T04:59:00Z'), // 23:59 EST on the 1st
			entry(3, '2026-11-01T06:30:00Z'), // 01:30 EST, after the change
			entry(2, '2026-11-01T04:30:00Z'), // 00:30 EDT, before it
			entry(1, '2026-11-01T03:30:00Z') // 23:30 EDT on 31 Oct
		];
		expect(
			groupByDay(ny, 'America/New_York').map((d) => [d.key, d.entries.map((e) => e.id)])
		).toEqual([
			['2026-11-02', [5]],
			['2026-11-01', [4, 3, 2]],
			['2026-10-31', [1]]
		]);
		// Vilnius: 25 Oct 2026 runs 21:00Z on the 24th to 22:00Z on the 25th.
		const vilnius = [
			entry(3, '2026-10-25T22:00:00Z'),
			entry(2, '2026-10-25T21:59:00Z'),
			entry(1, '2026-10-24T21:00:00Z')
		];
		expect(
			groupByDay(vilnius, 'Europe/Vilnius').map((d) => [d.key, d.entries.map((e) => e.id)])
		).toEqual([
			['2026-10-26', [3]],
			['2026-10-25', [2, 1]]
		]);
	});

	it('is empty for no entries', () => {
		expect(groupByDay([], 'UTC')).toEqual([]);
	});
});

describe('dayHeading', () => {
	it('splits the weekday from the full date, always with the year', () => {
		expect(dayHeading('2026-11-03', 'en-GB')).toEqual({
			weekday: 'Tuesday',
			date: '3 November 2026'
		});
		expect(dayHeading('2025-12-31', 'en-GB')).toEqual({
			weekday: 'Wednesday',
			date: '31 December 2025'
		});
	});

	it('names the day itself, never "Today"', () => {
		// The key is a calendar day, not an instant: no zone shifts it.
		expect(dayHeading('2026-01-01', 'en-GB').weekday).toBe('Thursday');
		expect(dayHeading('2026-11-03', 'en-US')).toEqual({
			weekday: 'Tuesday',
			date: 'November 3, 2026'
		});
	});
});

describe('dayTotal', () => {
	const movie = (id: number, runtime: number): HistoryEntry => ({
		...entry(id, '2026-11-03T20:00:00Z'),
		movie: {
			tmdb_id: id,
			title: 'A movie',
			poster_path: '',
			poster_url: '',
			release_date: '1999-03-30',
			runtime,
			vote_average: 8.2
		}
	});
	const episode = (id: number, runtime: number): HistoryEntry => ({
		...entry(id, '2026-11-03T20:00:00Z'),
		episode: {
			tmdb_id: id,
			season_number: 1,
			episode_number: id,
			name: 'An episode',
			still_path: '',
			still_url: '',
			air_date: '',
			runtime,
			vote_average: 0,
			show: { tmdb_id: 1396, name: 'A show', poster_path: '', poster_url: '' }
		}
	});

	it('adds up minutes as 45m, 1h 45m, or 2h', () => {
		expect(dayTotal([episode(1, 45)])).toEqual({
			minutes: 45,
			text: '45m',
			label: '45m watched, 1 watch'
		});
		expect(dayTotal([movie(1, 58), episode(2, 47)])).toEqual({
			minutes: 105,
			text: '1h 45m',
			label: '1h 45m watched, 2 watches'
		});
		expect(dayTotal([episode(1, 60), episode(2, 60)]).text).toBe('2h');
	});

	it('sums the known runtimes and says how many are unknown', () => {
		expect(dayTotal([movie(1, 58), episode(2, 0), episode(3, 47)])).toEqual({
			minutes: 105,
			text: '1h 45m',
			label: '1h 45m watched, 3 watches, runtime unknown for 1'
		});
	});

	it('counts watches when no runtime is known', () => {
		expect(dayTotal([episode(1, 0), movie(2, 0), episode(3, 0)])).toEqual({
			minutes: 0,
			text: '3 watches',
			label: '3 watches, runtime unknown'
		});
		expect(dayTotal([entry(1, '2026-11-03T20:00:00Z')]).text).toBe('1 watch');
	});
});

describe('insertEntry', () => {
	// Newest first, as the API sends them; 3 and 2 share a time.
	const loaded = [
		entry(4, '2026-11-03T20:00:00Z'),
		entry(3, '2026-11-02T20:00:00Z'),
		entry(2, '2026-11-02T20:00:00Z'),
		entry(1, '2026-11-01T20:00:00Z')
	];
	const ids = (r: { entries: HistoryEntry[] }) => r.entries.map((e) => e.id);

	it('puts a new watch at its place by time', () => {
		expect(ids(insertEntry(loaded, entry(9, '2026-11-04T08:00:00Z'), true))).toEqual([
			9, 4, 3, 2, 1
		]);
		expect(ids(insertEntry(loaded, entry(9, '2026-11-02T09:00:00Z'), true))).toEqual([
			4, 3, 2, 9, 1
		]);
	});

	it('orders by id within the same time, the higher first', () => {
		expect(ids(insertEntry(loaded, entry(9, '2026-11-02T20:00:00Z'), true))).toEqual([
			4, 9, 3, 2, 1
		]);
	});

	it('leaves out one older than everything loaded while more pages remain', () => {
		const r = insertEntry(loaded, entry(9, '2026-10-01T20:00:00Z'), true);
		expect(r.inserted).toBe(false);
		expect(r.entries).toBe(loaded);
	});

	it('appends one older than everything at the end of history', () => {
		const r = insertEntry(loaded, entry(9, '2026-10-01T20:00:00Z'), false);
		expect(r.inserted).toBe(true);
		expect(ids(r)).toEqual([4, 3, 2, 1, 9]);
	});

	it("doesn't add one already shown", () => {
		const r = insertEntry(loaded, entry(3, '2026-11-02T20:00:00Z'), true);
		expect(r).toEqual({ entries: loaded, inserted: true });
	});
});
