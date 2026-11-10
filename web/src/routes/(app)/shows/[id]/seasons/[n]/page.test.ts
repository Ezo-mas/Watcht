import { describe, expect, it } from 'vitest';
import { isHttpError } from '@sveltejs/kit';
import { render } from 'svelte/server';
import type { Episode, EpisodeWatched, SeasonDetails, Show } from '#lib/api/types.js';
import { load } from './+page';
import Page from './+page.svelte';

type LoadEvent = Parameters<typeof load>[0];

const show: Show = {
	tmdb_id: 1396,
	imdb_id: 'tt0903747',
	tvdb_id: 81189,
	name: 'Breaking Bad',
	original_name: 'Breaking Bad',
	original_language: 'en',
	overview: 'Walter White, a New Mexico chemistry teacher, is diagnosed with cancer.',
	tagline: '',
	status: 'Ended',
	poster_path: '',
	poster_url: 'https://img.test/show.jpg',
	backdrop_path: '',
	backdrop_url: '',
	first_air_date: '2008-01-20',
	last_air_date: '2013-09-29',
	in_production: false,
	genres: [{ tmdb_id: 18, name: 'Drama' }],
	seasons: [0, 1, 2].map((n) => ({
		tmdb_id: 3570 + n,
		season_number: n,
		name: n === 0 ? 'Specials' : `Season ${n}`,
		overview: '',
		poster_path: '',
		poster_url: '',
		air_date: '2008-01-20',
		episode_count: 3,
		episodes_pending: false
	})),
	vote_average: 8.9,
	vote_count: 15000
};

function event(id: string, n: string, fetchFn: typeof fetch): LoadEvent {
	return {
		params: { id, n },
		fetch: fetchFn,
		depends: () => {},
		parent: async () => ({ show })
	} as unknown as LoadEvent;
}

/** Answers each request by its path; anything else is a 404. */
const byPath = (bodies: Record<string, unknown>, asked: string[] = []) =>
	(async (input: RequestInfo | URL) => {
		const path = String(input);
		asked.push(path);
		if (!(path in bodies)) {
			return new Response(JSON.stringify({ error: { code: 'not_found', message: 'x' } }), {
				status: 404,
				headers: { 'Content-Type': 'application/json' }
			});
		}
		return new Response(JSON.stringify(bodies[path]), {
			status: 200,
			headers: { 'Content-Type': 'application/json' }
		});
	}) as unknown as typeof fetch;

const fail = async (p: unknown) => Promise.resolve(p).catch((e: unknown) => e);

const episode = (n: number, air_date: string, extra: Partial<Episode> = {}): Episode => ({
	tmdb_id: 62084 + n,
	season_number: 1,
	episode_number: n,
	name: `Episode ${n}`,
	overview: `What happens in episode ${n}.`,
	still_path: '',
	still_url: '',
	still_large_url: '',
	air_date,
	runtime: 47,
	vote_average: 0,
	...extra
});

const seasonOne: SeasonDetails = {
	show_tmdb_id: 1396,
	season: { ...show.seasons[1], overview: 'The first season.' },
	episodes_pending: false,
	episodes: [
		episode(1, '2008-01-20', { name: 'Pilot', vote_average: 8.3 }),
		episode(2, '2008-01-27'),
		episode(3, '2009-02-10', { runtime: 0 })
	]
};

describe('season page load', () => {
	it('loads the season and the watched episodes, after the show is stored', async () => {
		const asked: string[] = [];
		const episodes = [{ tmdb_id: 62085, season_number: 1, plays: 1 }];
		const fetchFn = byPath(
			{ '/api/v1/shows/1396/seasons/1': seasonOne, '/api/v1/shows/1396/watched': { episodes } },
			asked
		);
		expect(await load(event('1396', '1', fetchFn))).toEqual({
			season: seasonOne,
			watched: episodes,
			watchedError: ''
		});
		expect(asked).toContain('/api/v1/shows/1396/seasons/1');
	});

	it("still shows the season when the watched episodes can't be loaded", async () => {
		const fetchFn = byPath({ '/api/v1/shows/1396/seasons/1': seasonOne });
		expect(await load(event('1396', '1', fetchFn))).toMatchObject({
			season: seasonOne,
			watched: null,
			watchedError: expect.any(String)
		});
	});

	it('is a 404 for a season number that is not a number, without asking the API', async () => {
		for (const n of ['abc', '-1', '1.5']) {
			const asked: string[] = [];
			const err = await fail(load(event('1396', n, byPath({}, asked))));
			expect(isHttpError(err, 404)).toBe(true);
			expect(err).toMatchObject({ body: { missing: 'season' } });
			expect(asked).toEqual([]);
		}
	});

	it("is a 404 for a season the show doesn't list, without asking for it", async () => {
		const asked: string[] = [];
		const err = await fail(load(event('1396', '9', byPath({}, asked))));
		expect(isHttpError(err, 404)).toBe(true);
		expect(err).toMatchObject({
			body: { message: 'TMDB lists no season 9 for Breaking Bad.', missing: 'season' }
		});
		expect(asked).toEqual([]);
	});

	it('is a 404 when TMDB has no such season after all', async () => {
		const err = await fail(load(event('1396', '2', byPath({}))));
		expect(isHttpError(err, 404)).toBe(true);
		expect(err).toMatchObject({
			body: { message: 'TMDB has no season 2 for Breaking Bad.', missing: 'season' }
		});
	});
});

const seen = (tmdb_id: number, plays = 1): EpisodeWatched => ({
	tmdb_id,
	season_number: 1,
	plays,
	last_watched_at: '2026-11-03T18:14:00Z',
	last_watched_on: null
});

/** The season page's HTML, as the server renderer writes it (comments removed). */
function html(
	season: SeasonDetails = seasonOne,
	watched: EpisodeWatched[] | null = [],
	ofShow: Show = show
): string {
	const props = {
		data: { show: ofShow, season, watched, watchedError: watched ? '' : 'Server error' }
	};
	return render(Page, { props: props as never }).body.replace(/<!--[^]*?-->/g, '');
}

const buttons = (body: string) =>
	[...body.matchAll(/(<button\b[^>]*>)([^]*?)<\/button>/g)].map(([, tag, inner]) => ({
		tag,
		text: inner
			.replace(/<[^>]*>/g, ' ')
			.replace(/\s+/g, ' ')
			.trim()
	}));

describe('season page', () => {
	it("titles it with the season's name and the years its episodes aired", () => {
		const body = html();
		expect(body).toMatch(/<h1[^>]*>\s*Season 1(&nbsp;|\u00a0)<span[^>]*>2008–2009<\/span>/);
		const head = render(Page, {
			props: { data: { show, season: seasonOne, watched: [], watchedError: '' } } as never
		}).head;
		expect(head).toContain('<title>Season 1 · Breaking Bad · watcht</title>');
	});

	it('links back to the show in a breadcrumb on the hero', () => {
		const body = html();
		const crumbs = body.slice(body.indexOf('aria-label="Breadcrumb"'));
		expect(crumbs).toMatch(/^[^]*?<a href="\/shows\/1396"[^>]*>Breaking Bad<\/a>/);
		expect(body.indexOf('aria-label="Breadcrumb"')).toBeLessThan(body.indexOf('<h1'));
	});

	it('picks between the seasons in a strip, specials last, this one current', () => {
		const body = html();
		const strip = body.slice(body.indexOf('aria-label="Seasons of Breaking Bad"'));
		const links = [...strip.matchAll(/<a href="(\/shows\/1396\/seasons\/\d)"([^>]*)>/g)];
		expect(links.map((m) => m[1])).toEqual([
			'/shows/1396/seasons/1',
			'/shows/1396/seasons/2',
			'/shows/1396/seasons/0'
		]);
		expect(links[0][2]).toContain('aria-current="page"');
		expect(links[1][2]).not.toContain('aria-current');
		expect(strip).toContain('<span class="sr-only">Season\u00a0</span>2');
	});

	it('ends the strip with All, the every-episode page', () => {
		const body = html();
		const strip = body.slice(body.indexOf('aria-label="Seasons of Breaking Bad"'));
		expect(strip).toMatch(/<a href="\/shows\/1396\/seasons\/all"[^>]*aria-label="All episodes"/);
	});

	it('has arrows to the seasons either side on the hero, none past the ends', () => {
		const body = html();
		const hero = body.slice(0, body.indexOf('</header>'));
		expect(hero).not.toContain('aria-label="Previous');
		expect(hero).toMatch(
			/<a href="\/shows\/1396\/seasons\/2"[^>]*aria-label="Next season: Season 2"/
		);
		const two = { ...seasonOne, season: { ...show.seasons[2] } };
		const second = html(two);
		const hero2 = second.slice(0, second.indexOf('</header>'));
		expect(hero2).toMatch(/aria-label="Previous season: Season 1"/);
		expect(hero2).toMatch(/aria-label="Next season: Specials"/);
	});

	it('counts the episodes first in the section list and the heading', () => {
		const body = html();
		expect(body).toMatch(/href="#episodes"[^]*?>\s*3 Episodes\s*<\/span>\s*<svg/);
		expect(body).toMatch(/<h2[^>]*id="episodes-title"[^>]*>\s*3 Episodes\s*<\/h2>/);
		const one = { ...seasonOne, episodes: [episode(1, '2008-01-20')] };
		expect(html(one)).toMatch(/<h2[^>]*id="episodes-title"[^>]*>\s*1 Episode\s*<\/h2>/);
	});

	it('puts People before Episodes in the nav and body, with Videos full width last', () => {
		const body = html();
		expect(body.indexOf('href="#people"')).toBeLessThan(body.indexOf('href="#episodes"'));
		expect(body.indexOf('id="people"')).toBeLessThan(body.indexOf('id="episodes"'));
		expect(body).toMatch(
			/<div class="col-span-full min-w-0 row-start-6">\s*<section[^>]*id="videos"/
		);
	});

	it('shows the mean of positive episode scores with one decimal, or no band', () => {
		const rated = {
			...seasonOne,
			episodes: [
				episode(1, '', { vote_average: 8.2 }),
				episode(2, '', { vote_average: 9.3 }),
				episode(3, '')
			]
		};
		expect(html(rated)).toContain('TMDB rating 8.8 out of 10');
		expect(html({ ...seasonOne, episodes: [episode(1, '')] })).not.toContain('TMDB rating');
	});

	it('offers remove-or-add options only for watched episode checks', () => {
		const body = html(seasonOne, [seen(62085, 2)]);
		for (const label of [
			'Remove from history?',
			'Only this play',
			'Keep it',
			'Add another play',
			'Coming in a later update'
		])
			expect(body).toContain(label);
		const list = html().slice(html().indexOf('id="episodes"'));
		expect(list).not.toContain('Only this play');
	});

	it('has sort, filter, and hide-watched placeholders over the episodes', () => {
		const list = html().slice(html().indexOf('id="episodes"'));
		expect(list).toContain('aria-label="Sort and filter episodes"');
		expect(list).toMatch(
			/<input[^>]*readonly[^>]*aria-disabled="true"[^>]*placeholder="Filter by title"/
		);
		const hide = buttons(list).find((b) => b.tag.includes('aria-label="Hide watched episodes"'));
		expect(hide?.tag).toContain('aria-disabled="true"');
	});

	it('has its facts, overview, and sections', () => {
		const body = html();
		for (const label of ['Premiered', 'Episodes', 'Runtime', 'Genres']) {
			expect(body).toMatch(new RegExp(`<dt[^>]*>\\s*${label}\\s*</dt>`));
		}
		// The runtimes it knows: 47 + 47.
		expect(body).toMatch(/<dd[^>]*>\s*1h 34m\s*<\/dd>/);
		expect(body).toContain('The first season.');
		for (const id of ['overview', 'episodes', 'people', 'videos']) {
			expect(body).toContain(`href="#${id}"`);
		}
		// The TMDB link is the season's own page.
		expect(body).toContain('href="https://www.themoviedb.org/tv/1396/season/1"');
		// Falls back to the show's poster and overview.
		expect(body).toContain('src="https://img.test/show.jpg"');
		const noOverview = { ...seasonOne, season: { ...seasonOne.season, overview: '' } };
		expect(html(noOverview)).toContain('Walter White');
	});

	it('lists the episodes, each linking to its page, with its check, badges, and score', () => {
		const body = html(seasonOne, [seen(62085, 2)]);
		const list = body.slice(body.indexOf('id="episodes"'), body.indexOf('id="videos"'));
		const links = [...list.matchAll(/href="(\/shows\/1396\/seasons\/1\/episodes\/\d)"/g)].map(
			(m) => m[1]
		);
		// The still and the title each link to it.
		expect(new Set(links)).toEqual(
			new Set([1, 2, 3].map((n) => `/shows/1396/seasons/1/episodes/${n}`))
		);
		expect(list).toMatch(/<h3[^>]*>[^]*?S01E01<\/span>\s*Pilot/);
		expect(list).toContain('aria-label="Remove or add plays of S01E01 “Pilot”…"');
		// Watched, the sage check offers removal or another play.
		const another = list.match(/<button[^>]*aria-label="Remove or add plays of S01E01[^>]*>/)?.[0];
		expect(another).toContain('bg-watched ');
		expect(another).toContain('hover:bg-watched-hover');
		expect(list).toContain('2 plays');
		// The check opens the when-choice itself: no separate chevron.
		expect(list).toMatch(
			/<button[^>]*aria-label="Mark S01E02 “Episode 2” watched…"[^>]*aria-haspopup="dialog"/
		);
		expect(list).not.toContain('Choose when you watched');
		expect(list).toContain('aria-label="Add S01E02 to watchlist (coming in a later update)"');
		expect(list).toContain('83%');
		// Season 1's first episode opens the series; its last closes the season
		// (season 2 exists).
		expect(list).toContain('Series premiere');
		expect(list).toContain('Season finale');
		expect(list).not.toContain('Series finale');
	});

	it('shows the progress button once an episode of it is watched', () => {
		const body = html(seasonOne, [seen(62085, 2)]);
		const progress = buttons(body).find((b) => b.tag.includes('id="season-progress"'));
		expect(progress?.text).toBe('33% watched 1 of 3 episodes · 2 plays');
		expect(body).toContain(
			'aria-label="Mark the rest of Breaking Bad Season 1 watched, choosing when"'
		);
		expect(body).not.toContain('Mark season watched');
	});

	it('counts progress over the episodes it lists once they are in, not the season count', () => {
		// TMDB lists two of the season's three episodes. The user also watched one
		// it no longer lists: the season grid's count takes that in, the page doesn't.
		const twoListed: SeasonDetails = {
			...seasonOne,
			episodes: [episode(1, '2008-01-20', { name: 'Pilot' }), episode(2, '2008-01-27')]
		};
		const body = html(twoListed, [seen(62085), seen(62099)]);
		const progress = buttons(body).find((b) => b.tag.includes('id="season-progress"'));
		expect(progress?.text).toBe('50% watched 1 of 2 episodes · 1 play');
	});

	it('offers "Mark season watched" with a + until then', () => {
		const body = html();
		const mark = buttons(body).find((b) => b.text === 'Mark season watched');
		expect(mark?.tag).toContain('id="mark-season"');
		expect(mark?.tag).not.toMatch(/\sdisabled[\s=>]/);
	});

	it('says why the mark is off when nothing in it has aired', () => {
		const upcoming = { ...seasonOne, episodes: [episode(1, '2099-01-01'), episode(2, '')] };
		const body = html(upcoming);
		expect(body).toContain('Nothing in this season has aired yet.');
		expect(buttons(body).find((b) => b.text === 'Mark season watched')?.tag).toMatch(
			/\sdisabled[\s=>]/
		);
	});

	it('shows the fetching state while the episodes are pending', () => {
		const pending = { ...seasonOne, episodes_pending: true, episodes: null };
		const body = html(pending, [seen(62085)]);
		expect(body).toContain('Fetching episodes from TMDB…');
		expect(body).toContain("This season's episodes are still being fetched from TMDB.");
		// Progress from the show's count while the episodes aren't in.
		expect(body).toContain('1 of 3 episodes');
	});

	it("has no checks or progress when the watched episodes can't be loaded", () => {
		const body = html(seasonOne, null);
		expect(body).toContain("Couldn't load which episodes you've watched: Server error");
		expect(body).not.toContain('data-check');
		expect(body).not.toMatch(/% watched/);
	});
});
