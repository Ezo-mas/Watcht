import { describe, expect, it } from 'vitest';
import { ALL, episodeArrows, seasonArrows, seasonPicker } from './season-nav';

const season = (season_number: number, extra: Record<string, unknown> = {}) => ({
	tmdb_id: 100 + season_number,
	season_number,
	name: season_number === 0 ? 'Specials' : `Season ${season_number}`,
	overview: '',
	poster_path: '',
	poster_url: '',
	air_date: '2008-01-20',
	episode_count: 3,
	episodes_pending: false,
	...extra
});

const show = { tmdb_id: 1396, seasons: [season(0), season(2), season(1)] };

describe('seasonPicker', () => {
	it('lists the seasons, specials, then All, with the current one', () => {
		const items = seasonPicker(show, 2);
		expect(items.map((i) => i.label)).toEqual(['1', '2', 'Specials', 'All']);
		expect(items.map((i) => i.href)).toEqual([
			'/shows/1396/seasons/1',
			'/shows/1396/seasons/2',
			'/shows/1396/seasons/0',
			'/shows/1396/seasons/all'
		]);
		expect(items.filter((i) => i.current).map((i) => i.key)).toEqual([2]);
		expect(items[3]).toMatchObject({ key: ALL, name: 'All episodes', current: false });
		expect(seasonPicker(show, ALL)[3].current).toBe(true);
	});
});

describe('seasonArrows', () => {
	it('goes to the seasons either side, without wrapping', () => {
		expect(seasonArrows(show, 1)).toEqual({
			prev: undefined,
			next: { href: '/shows/1396/seasons/2', label: 'Next season: Season 2' }
		});
		expect(seasonArrows(show, 2)).toEqual({
			prev: { href: '/shows/1396/seasons/1', label: 'Previous season: Season 1' },
			next: { href: '/shows/1396/seasons/0', label: 'Next season: Specials' }
		});
	});

	it('ends with specials, then All episodes', () => {
		expect(seasonArrows(show, 0).next).toEqual({
			href: '/shows/1396/seasons/all',
			label: 'Next: All episodes'
		});
		expect(seasonArrows(show, ALL)).toEqual({
			prev: { href: '/shows/1396/seasons/0', label: 'Previous season: Specials' },
			next: undefined
		});
	});

	it("uses a season's own name", () => {
		const named = { ...show, seasons: [season(1), season(2, { name: 'Book Two: Earth' })] };
		expect(seasonArrows(named, 1).next?.label).toBe('Next season: Book Two: Earth');
	});
});

describe('episodeArrows', () => {
	const episodes = [1, 2, 3].map((n) => ({
		tmdb_id: 60 + n,
		season_number: 1,
		episode_number: n,
		name: `Episode ${n}`
	}));

	it('goes to the episodes either side within the season', () => {
		expect(episodeArrows(1396, episodes, 62)).toEqual({
			prev: {
				href: '/shows/1396/seasons/1/episodes/1',
				label: 'Previous episode: S01E01 Episode 1'
			},
			next: { href: '/shows/1396/seasons/1/episodes/3', label: 'Next episode: S01E03 Episode 3' }
		});
	});

	it("stops at the season's ends", () => {
		expect(episodeArrows(1396, episodes, 61).prev).toBeUndefined();
		expect(episodeArrows(1396, episodes, 63).next).toBeUndefined();
	});
});
