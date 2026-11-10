import { describe, expect, it } from 'vitest';
import {
	airYears,
	bySeason,
	countLabel,
	episodeMilestone,
	lastSeasonNumber,
	milestoneIn,
	neighbours,
	progressBySeason,
	progressPercent,
	progressSummary,
	seasonEnds,
	seasonRating,
	seasonList,
	seasonMark,
	seasonsLabel,
	showOrder,
	showProgress,
	totalRuntime
} from './seasons';

const season = (tmdb_id: number, season_number: number, episode_count = 10) => ({
	tmdb_id,
	season_number,
	episode_count
});
const seen = (season_number: number, plays = 1) => ({ season_number, plays });

describe('seasonList', () => {
	it('lists regular seasons in order and specials last', () => {
		const list = seasonList([season(3, 2), season(1, 0), season(2, 1)]);
		expect(list.map((s) => s.season_number)).toEqual([1, 2, 0]);
	});

	it('keeps one season per number, the lowest TMDB ID', () => {
		const list = seasonList([season(9, 1, 5), season(4, 1, 8), season(5, 2)]);
		expect(list).toEqual([season(4, 1, 8), season(5, 2)]);
	});
});

describe('progress', () => {
	const seasons = [season(10, 0, 4), season(11, 1, 7), season(12, 2, 6)];

	it('counts a show over its regular seasons only, with every play', () => {
		const watched = [seen(1, 2), seen(1), seen(2, 3), seen(0, 5)];
		expect(showProgress(seasons, watched)).toEqual({ watched: 3, total: 13, plays: 6 });
	});

	it('is nothing watched of the total when only specials are watched', () => {
		expect(showProgress(seasons, [seen(0)])).toEqual({ watched: 0, total: 13, plays: 0 });
	});

	it('counts each season by its number, specials too', () => {
		const by = progressBySeason(seasons, [seen(1), seen(0, 2)]);
		expect(by.get(0)).toEqual({ watched: 1, total: 4, plays: 2 });
		expect(by.get(1)).toEqual({ watched: 1, total: 7, plays: 1 });
		expect(by.get(2)).toEqual({ watched: 0, total: 6, plays: 0 });
	});

	it('never counts a season past its episodes (watches of episodes TMDB dropped)', () => {
		const by = progressBySeason([season(1, 1, 2)], [seen(1), seen(1), seen(1)]);
		expect(by.get(1)).toEqual({ watched: 2, total: 2, plays: 3 });
	});

	it('counts a duplicated season number once', () => {
		const dup = [season(5, 1, 7), season(4, 1, 3)];
		expect(showProgress(dup, [seen(1)])).toEqual({ watched: 1, total: 3, plays: 1 });
	});

	it('says 100% only when everything is watched, and at least 1% once anything is', () => {
		expect(progressPercent({ watched: 0, total: 10 })).toBe(0);
		expect(progressPercent({ watched: 1, total: 1000 })).toBe(1);
		expect(progressPercent({ watched: 76, total: 133 })).toBe(57);
		expect(progressPercent({ watched: 199, total: 200 })).toBe(99);
		expect(progressPercent({ watched: 200, total: 200 })).toBe(100);
		expect(progressPercent({ watched: 3, total: 0 })).toBe(0);
	});

	it('summarises it for the progress button', () => {
		expect(progressSummary({ watched: 76, total: 133, plays: 78 })).toBe(
			'76 of 133 episodes · 78 plays'
		);
		expect(progressSummary({ watched: 1, total: 1, plays: 1 })).toBe('1 of 1 episode · 1 play');
		expect(progressSummary({ watched: 1200, total: 2000, plays: 1500 })).toBe(
			'1,200 of 2,000 episodes · 1,500 plays'
		);
	});
});

describe('episodeMilestone', () => {
	const now = new Date('2026-11-09T15:00:00Z');
	const aired = (n: number) => ({ episode_number: n, air_date: '2020-01-01' });
	const upcoming = (n: number) => ({ episode_number: n, air_date: '2027-01-01' });
	const ep = (season_number: number, episode_number: number) => ({ season_number, episode_number });
	const ended = { lastSeason: 3, inProduction: false };
	const running = { lastSeason: 3, inProduction: true };
	const s = [aired(1), aired(2), aired(3)];

	it('marks the first episode of season 1 as the series premiere', () => {
		expect(episodeMilestone(ep(1, 1), s, running, now)).toBe('Series premiere');
		expect(episodeMilestone(ep(1, 1), [upcoming(1)], running, now)).toBe('Series premiere');
	});

	it('marks the first episode of a later season as its premiere', () => {
		expect(episodeMilestone(ep(2, 1), s, running, now)).toBe('Season premiere');
		expect(episodeMilestone(ep(3, 1), s, ended, now)).toBe('Season premiere');
	});

	it('marks the last episode of a finished season that cannot grow as its finale', () => {
		expect(episodeMilestone(ep(2, 3), s, running, now)).toBe('Season finale');
		expect(episodeMilestone(ep(1, 3), s, ended, now)).toBe('Season finale');
	});

	it('marks the last episode of an ended show as the series finale', () => {
		expect(episodeMilestone(ep(3, 3), s, ended, now)).toBe('Series finale');
	});

	it('marks no finale while the season is still airing or the newest one may grow', () => {
		expect(episodeMilestone(ep(2, 3), [aired(1), aired(2), upcoming(3)], running, now)).toBe('');
		expect(episodeMilestone(ep(3, 3), s, running, now)).toBe('');
		expect(
			episodeMilestone(ep(3, 3), [aired(1), { episode_number: 3, air_date: '' }], ended, now)
		).toBe('');
	});

	it('marks nothing in the middle, in specials, or without a list', () => {
		expect(episodeMilestone(ep(2, 2), s, ended, now)).toBe('');
		expect(episodeMilestone(ep(0, 1), s, ended, now)).toBe('');
		expect(episodeMilestone(ep(2, 1), [], ended, now)).toBe('');
	});

	it('treats a one-episode season as its premiere unless it ends the series', () => {
		expect(episodeMilestone(ep(2, 1), [aired(1)], ended, now)).toBe('Season premiere');
		expect(episodeMilestone(ep(3, 1), [aired(1)], ended, now)).toBe('Series finale');
	});

	it('works out a season once for every row: its ends and whether it has aired', () => {
		expect(seasonEnds([aired(3), aired(1), aired(2)], now)).toEqual({
			first: 1,
			last: 3,
			finished: true
		});
		expect(seasonEnds([aired(1), upcoming(2)], now)).toEqual({
			first: 1,
			last: 2,
			finished: false
		});
		expect(seasonEnds([], now)).toBeNull();
		// The same answers as episodeMilestone, from the precomputed ends.
		const ends = seasonEnds(s, now);
		for (const e of [ep(1, 1), ep(2, 1), ep(2, 2), ep(2, 3), ep(3, 3), ep(0, 1)]) {
			for (const show of [ended, running]) {
				expect(milestoneIn(e, ends, show)).toBe(episodeMilestone(e, s, show, now));
			}
		}
		expect(milestoneIn(ep(2, 1), null, ended)).toBe('');
	});

	it('finds the last regular season', () => {
		expect(
			lastSeasonNumber([{ season_number: 0 }, { season_number: 7 }, { season_number: 2 }])
		).toBe(7);
		expect(lastSeasonNumber([])).toBe(0);
	});
});

describe('season facts', () => {
	it('gives the years a season aired', () => {
		const dates = (...d: string[]) => d.map((air_date) => ({ air_date }));
		expect(airYears(dates('2008-10-03', '2009-03-20', ''))).toBe('2008–2009');
		expect(airYears(dates('2008-10-03', '2008-12-01'))).toBe('2008');
		expect(airYears(dates(''), '2010-05-01')).toBe('2010');
		expect(airYears([])).toBe('');
	});

	it('sums the known runtimes', () => {
		expect(totalRuntime([{ runtime: 23 }, { runtime: 0 }, { runtime: 24 }])).toBe(47);
	});
});

describe('count labels', () => {
	it('puts the count first, singular for one', () => {
		expect(countLabel(22, 'Episode')).toBe('22 Episodes');
		expect(countLabel(1, 'Episode')).toBe('1 Episode');
		expect(countLabel(0, 'Episode')).toBe('0 Episodes');
		expect(countLabel(1214, 'Episode')).toBe('1,214 Episodes');
	});

	it('counts regular seasons only, one per number', () => {
		expect(seasonsLabel([season(1, 0), season(2, 1), season(3, 2), season(4, 2)])).toBe(
			'2 Seasons'
		);
		expect(seasonsLabel([season(1, 1), season(2, 0)])).toBe('1 Season');
		// Only specials: no count to give.
		expect(seasonsLabel([season(1, 0)])).toBe('Seasons');
	});
});

describe('neighbours', () => {
	const items = ['a', 'b', 'c'];

	it('gives the items on either side, stopping at the ends', () => {
		expect(neighbours(items, (i) => i === 'b')).toEqual({ prev: 'a', next: 'c' });
		expect(neighbours(items, (i) => i === 'a')).toEqual({ prev: undefined, next: 'b' });
		expect(neighbours(items, (i) => i === 'c')).toEqual({ prev: 'b', next: undefined });
	});

	it('gives none when nothing is current', () => {
		expect(neighbours(items, () => false)).toEqual({});
	});
});

describe('seasonMark', () => {
	const now = new Date('2026-11-09T12:00:00Z');
	const s = (
		extra: Partial<{ air_date: string; episode_count: number; episodes_pending: boolean }>
	) => ({
		air_date: '2008-01-20',
		episode_count: 7,
		episodes_pending: false,
		...extra
	});

	it('marks a season nothing of which is watched, or the rest of one', () => {
		expect(seasonMark(s({}), { watched: 0, total: 7 }, now)).toEqual({ kind: 'mark', rest: false });
		expect(seasonMark(s({}), { watched: 3, total: 7 }, now)).toEqual({ kind: 'mark', rest: true });
	});

	it('is done once every episode is watched', () => {
		expect(seasonMark(s({}), { watched: 7, total: 7 }, now)).toEqual({ kind: 'done' });
	});

	it('is off while its episodes are being fetched, or before it airs, and says why', () => {
		expect(seasonMark(s({ episodes_pending: true }), { watched: 0, total: 7 }, now)).toEqual({
			kind: 'off',
			reason: "This season's episodes are still being fetched from TMDB."
		});
		const off = { kind: 'off', reason: 'Nothing in this season has aired yet.' };
		expect(seasonMark(s({ air_date: '2027-01-01' }), { watched: 0, total: 7 }, now)).toEqual(off);
		expect(seasonMark(s({ episode_count: 0 }), { watched: 0, total: 0 }, now)).toEqual(off);
	});

	it('can still mark a season with no air date (the server adds what has aired)', () => {
		expect(seasonMark(s({ air_date: '' }), { watched: 0, total: 7 }, now)).toEqual({
			kind: 'mark',
			rest: false
		});
	});
});

describe('showOrder and bySeason', () => {
	const ep = (season_number: number, episode_number: number) => ({ season_number, episode_number });

	it('orders regular seasons first, specials last, each by number', () => {
		const list = showOrder([ep(0, 2), ep(2, 1), ep(1, 2), ep(0, 1), ep(1, 1)]);
		expect(list).toEqual([ep(1, 1), ep(1, 2), ep(2, 1), ep(0, 1), ep(0, 2)]);
	});

	it('groups episodes by season, keeping their order', () => {
		const groups = bySeason([ep(1, 1), ep(1, 2), ep(0, 1)]);
		expect([...groups.keys()]).toEqual([1, 0]);
		expect(groups.get(1)).toEqual([ep(1, 1), ep(1, 2)]);
	});
});

describe('seasonRating', () => {
	it('averages only positive episode scores', () => {
		expect(seasonRating([8.2, 9.3, 0, -1].map((vote_average) => ({ vote_average })))).toBe(8.75);
	});
	it('has no rating without rated episodes', () => {
		expect(seasonRating([])).toBe(0);
		expect(seasonRating([{ vote_average: 0 }])).toBe(0);
	});
});
