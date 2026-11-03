import { describe, expect, it } from 'vitest';
import {
	addedMessage,
	anyAired,
	hasAired,
	playsLabel,
	releaseTime,
	seasonProgress,
	sourceLabel,
	watchedById
} from './watched';

describe('releaseTime', () => {
	it('is noon UTC on the date, as the server stores it', () => {
		expect(releaseTime('2008-01-20')?.toISOString()).toBe('2008-01-20T12:00:00.000Z');
		expect(releaseTime('1895-12-28')?.toISOString()).toBe('1895-12-28T12:00:00.000Z');
	});

	it('is null for no date or an invalid one', () => {
		expect(releaseTime('')).toBeNull();
		expect(releaseTime('2008-02-30')).toBeNull();
		expect(releaseTime('2008')).toBeNull();
	});
});

describe('hasAired', () => {
	const now = new Date('2026-11-03T15:00:00Z');

	it('counts a date from noon UTC', () => {
		expect(hasAired('2026-11-02', now)).toBe(true);
		expect(hasAired('2026-11-03', now)).toBe(true);
		expect(hasAired('2026-11-03', new Date('2026-11-03T11:59:59Z'))).toBe(false);
		expect(hasAired('2026-11-04', now)).toBe(false);
	});

	it('is false without a date', () => {
		expect(hasAired('', now)).toBe(false);
	});
});

describe('seasonProgress', () => {
	const watched = watchedById([
		{ tmdb_id: 1, plays: 2, last_watched_at: '2026-11-01T20:00:00Z' },
		{ tmdb_id: 3, plays: 1, last_watched_at: '2026-11-02T20:00:00Z' },
		// An episode TMDB no longer lists for the season.
		{ tmdb_id: 99, plays: 1, last_watched_at: '2026-11-02T21:00:00Z' }
	]);

	it('counts only the listed episodes', () => {
		const episodes = [{ tmdb_id: 1 }, { tmdb_id: 2 }, { tmdb_id: 3 }, { tmdb_id: 4 }];
		expect(seasonProgress(episodes, watched)).toEqual({ watched: 2, total: 4 });
	});

	it('is 0 of 0 for an empty season', () => {
		expect(seasonProgress([], watched)).toEqual({ watched: 0, total: 0 });
	});
});

describe('anyAired', () => {
	const now = new Date('2026-11-03T15:00:00Z');

	it('is true when one episode has aired', () => {
		expect(anyAired([{ air_date: '' }, { air_date: '2026-11-01' }], now)).toBe(true);
	});

	it('is false when none has', () => {
		expect(anyAired([{ air_date: '' }, { air_date: '2026-12-01' }], now)).toBe(false);
		expect(anyAired([], now)).toBe(false);
	});
});

describe('labels', () => {
	it('names sources, and leaves manual marks unlabelled', () => {
		expect(sourceLabel('manual')).toBe('');
		expect(sourceLabel('plex')).toBe('Plex');
		expect(sourceLabel('trakt_import')).toBe('Trakt');
	});

	it('counts plays', () => {
		expect(playsLabel(1)).toBe('1 play');
		expect(playsLabel(3)).toBe('3 plays');
	});

	it('reports a season or show mark', () => {
		expect(addedMessage(8, true)).toBe('Added 8 episodes to your history.');
		expect(addedMessage(1, true)).toBe('Added 1 episode to your history.');
	});

	it('says why nothing was added', () => {
		expect(addedMessage(0, true)).toBe('Nothing added: every aired episode is already watched.');
		expect(addedMessage(0, false)).toBe('Nothing added: nothing has aired yet.');
	});
});
