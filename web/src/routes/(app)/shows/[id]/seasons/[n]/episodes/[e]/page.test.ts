import { describe, expect, it } from 'vitest';
import { isHttpError } from '@sveltejs/kit';
import { render } from 'svelte/server';
import type { Episode, SeasonDetails, Show, Watch } from '#lib/api/types.js';
import { formatWatch } from '#lib/format.js';
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
	overview: '',
	tagline: '',
	status: 'Ended',
	poster_path: '',
	poster_url: '',
	backdrop_path: '',
	backdrop_url: '',
	first_air_date: '2008-01-20',
	last_air_date: '2013-09-29',
	in_production: false,
	genres: [],
	seasons: [1, 2].map((n) => ({
		tmdb_id: 3570 + n,
		season_number: n,
		name: `Season ${n}`,
		overview: '',
		poster_path: '',
		poster_url: n === 2 ? 'https://img.test/s2.jpg' : '',
		air_date: '2009-03-08',
		episode_count: 2,
		episodes_pending: false
	})),
	vote_average: 8.9,
	vote_count: 15000
};

const episode = (n: number, extra: Partial<Episode> = {}): Episode => ({
	tmdb_id: 62160 + n,
	season_number: 2,
	episode_number: n,
	name: `Episode ${n}`,
	overview: `What happens in episode ${n}.`,
	still_path: '',
	still_url: '',
	still_large_url: '',
	air_date: '2009-03-08',
	runtime: 47,
	vote_average: 0,
	...extra
});

const seasonTwo: SeasonDetails = {
	show_tmdb_id: 1396,
	season: show.seasons[1],
	episodes_pending: false,
	// TMDB's order isn't relied on: the episode is found by its number.
	episodes: [episode(2, { name: 'ABQ', air_date: '2009-05-31', vote_average: 7.84 }), episode(1)]
};

function event(n: string, e: string, fetchFn: typeof fetch): LoadEvent {
	return {
		params: { id: '1396', n, e },
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

const watches: Watch[] = [
	{ id: 9, watched_at: '2026-11-03T18:14:00Z', watched_on: null, source: 'manual' }
];

describe('episode page load', () => {
	it('finds the episode by its number, then loads its plays by its TMDB ID', async () => {
		const asked: string[] = [];
		const fetchFn = byPath(
			{
				'/api/v1/shows/1396/seasons/2': seasonTwo,
				'/api/v1/episodes/62162/watches': { watches }
			},
			asked
		);
		expect(await load(event('2', '2', fetchFn))).toEqual({
			season: seasonTwo,
			episodeNumber: 2,
			episode: seasonTwo.episodes?.[0],
			watches,
			watchesError: ''
		});
		expect(asked).toEqual(['/api/v1/shows/1396/seasons/2', '/api/v1/episodes/62162/watches']);
	});

	it("still shows the episode when its plays can't be loaded", async () => {
		const fetchFn = byPath({ '/api/v1/shows/1396/seasons/2': seasonTwo });
		expect(await load(event('2', '1', fetchFn))).toMatchObject({
			episode: { tmdb_id: 62161 },
			watches: null,
			watchesError: expect.any(String)
		});
	});

	it('is a 404 for an address without numbers, without asking the API', async () => {
		for (const [n, e] of [
			['x', '1'],
			['2', 'one'],
			['2', '-1']
		]) {
			const asked: string[] = [];
			const err = await fail(load(event(n, e, byPath({}, asked))));
			expect(isHttpError(err, 404)).toBe(true);
			expect(err).toMatchObject({ body: { missing: 'episode' } });
			expect(asked).toEqual([]);
		}
	});

	it("is a 404 for a season the show doesn't list", async () => {
		const err = await fail(load(event('5', '1', byPath({}))));
		expect(isHttpError(err, 404)).toBe(true);
		expect(err).toMatchObject({
			body: { message: 'TMDB lists no season 5 for Breaking Bad.', missing: 'season' }
		});
	});

	it("is a 404 for an episode the season doesn't list, without asking for plays", async () => {
		const asked: string[] = [];
		const fetchFn = byPath({ '/api/v1/shows/1396/seasons/2': seasonTwo }, asked);
		const err = await fail(load(event('2', '9', fetchFn)));
		expect(isHttpError(err, 404)).toBe(true);
		expect(err).toMatchObject({
			body: { message: 'TMDB lists no episode S02E09 for Breaking Bad.', missing: 'episode' }
		});
		expect(asked).toEqual(['/api/v1/shows/1396/seasons/2']);
	});

	it('has no episode yet while the season is still being fetched', async () => {
		const pending = { ...seasonTwo, episodes_pending: true, episodes: null };
		const fetchFn = byPath({ '/api/v1/shows/1396/seasons/2': pending });
		expect(await load(event('2', '1', fetchFn))).toEqual({
			season: pending,
			episodeNumber: 1,
			episode: null,
			watches: null,
			watchesError: ''
		});
	});
});

/** The episode page's HTML, as the server renderer writes it (comments removed). */
function html(
	ep: Episode | null = seasonTwo.episodes?.[0] ?? null,
	plays: Watch[] | null = [],
	season: SeasonDetails = seasonTwo
): string {
	const props = {
		data: {
			show,
			season,
			episodeNumber: ep?.episode_number ?? 1,
			episode: ep,
			watches: plays,
			watchesError: plays ? '' : 'Server error'
		}
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

describe('episode page', () => {
	it('titles it with the code, the name, and the air year', () => {
		const body = html();
		expect(body).toMatch(
			/<h1[^>]*>\s*<span[^>]*>S02E02<\/span>(&nbsp;|\u00a0)ABQ(&nbsp;|\u00a0)<span[^>]*>2009<\/span>/
		);
		const head = render(Page, {
			props: {
				data: {
					show,
					season: seasonTwo,
					episodeNumber: 2,
					episode: seasonTwo.episodes?.[0],
					watches: [],
					watchesError: ''
				}
			} as never
		}).head;
		expect(head).toContain('<title>S02E02 ABQ · Breaking Bad · watcht</title>');
	});

	it('links back to the show and the season, with the finale flag', () => {
		const body = html();
		const crumbs = body.slice(body.indexOf('aria-label="Breadcrumb"'), body.indexOf('<h1'));
		const links = [...crumbs.matchAll(/<a href="([^"]+)"[^>]*>([^<]*)<\/a>/g)].map((m) => [
			m[1],
			m[2]
		]);
		expect(links).toEqual([
			['/shows/1396', 'Breaking Bad'],
			['/shows/1396/seasons/2', 'Season 2']
		]);
		// The show has ended and season 2 is its last: the series finale.
		expect(crumbs).toContain('Series finale');
		expect(html(seasonTwo.episodes?.[1] ?? null)).toContain('Season premiere');
	});

	it("shows TMDB's score, with no vote count to give", () => {
		const body = html();
		expect(body).toContain('TMDB rating 7.8 out of 10');
		expect(body).toContain('title="TMDB rating 7.8"');
		expect(body).not.toMatch(/votes/);
		expect(html(seasonTwo.episodes?.[1] ?? null)).not.toContain('aria-label="Ratings"');
	});

	it('picks between the episodes in a strip, in order, this one current', () => {
		const body = html();
		const strip = body.slice(body.indexOf('aria-label="Episodes of Breaking Bad Season 2"'));
		const links = [
			...strip.matchAll(/<a href="(\/shows\/1396\/seasons\/2\/episodes\/\d)"([^>]*)>/g)
		];
		expect(links.map((m) => m[1])).toEqual([
			'/shows/1396/seasons/2/episodes/2',
			'/shows/1396/seasons/2/episodes/1'
		]);
		expect(links[0][2]).toContain('aria-current="page"');
		expect(links[0][2]).toContain('title="ABQ"');
	});

	it("shows the episode's still on the hero, else the show's backdrop", () => {
		const still = {
			...(seasonTwo.episodes ?? [])[0],
			still_large_url: 'https://img.test/still.jpg'
		};
		const hero = (body: string) => body.slice(0, body.indexOf('</header>'));
		expect(hero(html(still))).toContain('src="https://img.test/still.jpg"');
		const withBackdrop = render(Page, {
			props: {
				data: {
					show: { ...show, backdrop_url: 'https://img.test/backdrop.jpg' },
					season: seasonTwo,
					episodeNumber: 2,
					episode: (seasonTwo.episodes ?? [])[0],
					watches: [],
					watchesError: ''
				}
			} as never
		}).body;
		expect(hero(withBackdrop)).toContain('src="https://img.test/backdrop.jpg"');
	});

	it("has arrows to the episodes either side in the season's order, none past its ends", () => {
		const hero = (body: string) => body.slice(0, body.indexOf('</header>'));
		const first = hero(html());
		expect(first).not.toContain('aria-label="Previous');
		expect(first).toMatch(
			/<a href="\/shows\/1396\/seasons\/2\/episodes\/1"[^>]*aria-label="Next episode: S02E01 Episode 1"/
		);
		const last = hero(html(seasonTwo.episodes?.[1] ?? null));
		expect(last).toMatch(/aria-label="Previous episode: S02E02 ABQ"/);
		expect(last).not.toContain('aria-label="Next');
	});

	it('has its facts, overview, sections, and TMDB link', () => {
		const body = html();
		expect(body).toMatch(/<dt[^>]*>\s*Aired\s*<\/dt>/);
		expect(body).toMatch(/<dt[^>]*>\s*Runtime\s*<\/dt>\s*<dd[^>]*>\s*47m\s*<\/dd>/);
		expect(body).toContain('What happens in episode 2.');
		for (const id of ['overview', 'people', 'videos']) expect(body).toContain(`href="#${id}"`);
		expect(body).toContain('href="https://www.themoviedb.org/tv/1396/season/2/episode/2"');
		expect(body).toContain('Breaking Bad S02E02 on TMDB (opens in a new tab)');
		// The season's poster.
		expect(body).toContain('src="https://img.test/s2.jpg"');
	});

	it('offers "Mark as watched" until it has a play, then the plays button', () => {
		const unwatched = html();
		const mark = buttons(unwatched).find((b) => b.text === 'Mark as watched');
		expect(mark?.tag).toContain('id="mark-episode"');
		expect(unwatched).toContain('aria-label="Choose when you watched Breaking Bad S02E02"');

		const watched = html(seasonTwo.episodes?.[0] ?? null, watches);
		const plays = buttons(watched).find((b) => b.tag.includes('id="episode-plays"'));
		expect(plays?.text).toBe(`1 play ${formatWatch(watches[0])}`);
		expect(watched).toContain('aria-label="Plays of Breaking Bad S02E02"');
		expect(watched).not.toContain('Mark as watched');
	});

	it("still offers the mark when its plays can't be loaded, with the error", () => {
		const body = html(seasonTwo.episodes?.[0] ?? null, null);
		expect(buttons(body).some((b) => b.text === 'Mark as watched')).toBe(true);
		expect(body).toContain("Couldn't load your plays of this episode: Server error");
	});

	it('says the episodes are being fetched while the season is pending', () => {
		const pending = { ...seasonTwo, episodes_pending: true, episodes: null };
		const body = html(null, null, pending);
		expect(body).toContain('Fetching episodes from TMDB…');
		expect(body).toMatch(/<h1[^>]*>\s*<span[^>]*>S02E01<\/span>/);
		expect(body).not.toContain('Mark as watched');
	});
});
