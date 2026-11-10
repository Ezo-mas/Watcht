import { describe, expect, it } from 'vitest';
import { isHttpError } from '@sveltejs/kit';
import { render } from 'svelte/server';
import type { Episode, EpisodeWatched, Show, ShowEpisodes } from '#lib/api/types.js';
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
	backdrop_url: 'https://img.test/backdrop.jpg',
	first_air_date: '2008-01-20',
	last_air_date: '2013-09-29',
	in_production: false,
	genres: [],
	seasons: [0, 1, 2].map((n) => ({
		tmdb_id: 3570 + n,
		season_number: n,
		name: n === 0 ? 'Specials' : `Season ${n}`,
		overview: '',
		poster_path: '',
		poster_url: '',
		air_date: '2008-01-20',
		episode_count: 2,
		episodes_pending: false
	})),
	vote_average: 8.9,
	vote_count: 15000
};

const episode = (season: number, n: number, extra: Partial<Episode> = {}): Episode => ({
	tmdb_id: 60000 + season * 100 + n,
	season_number: season,
	episode_number: n,
	name: `Episode ${season}.${n}`,
	overview: '',
	still_path: '',
	still_url: '',
	still_large_url: '',
	air_date: '2008-01-20',
	runtime: 45,
	vote_average: 0,
	...extra
});

// As the server sends them: by season (specials first), then number.
const all: ShowEpisodes = {
	show_tmdb_id: 1396,
	episodes_pending: false,
	episodes: [episode(0, 1), episode(1, 1), episode(1, 2), episode(2, 1, { runtime: 0 })]
};

function event(fetchFn: typeof fetch): LoadEvent {
	return {
		params: { id: '1396' },
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

describe('all episodes page load', () => {
	it("loads every episode in one request, with the user's watched episodes", async () => {
		const asked: string[] = [];
		const watched = [{ tmdb_id: 60101, season_number: 1, plays: 1 }];
		const fetchFn = byPath(
			{ '/api/v1/shows/1396/episodes': all, '/api/v1/shows/1396/watched': { episodes: watched } },
			asked
		);
		expect(await load(event(fetchFn))).toEqual({
			episodes: all,
			watched,
			watchedError: ''
		});
		expect(asked.filter((p) => p.includes('/seasons/'))).toEqual([]);
	});

	it("still shows the episodes when the watched ones can't be loaded", async () => {
		const fetchFn = byPath({ '/api/v1/shows/1396/episodes': all });
		expect(await load(event(fetchFn))).toMatchObject({
			episodes: all,
			watched: null,
			watchedError: expect.any(String)
		});
	});

	it("is a 404 for the show when the server doesn't have it", async () => {
		const err = await fail(load(event(byPath({}))));
		expect(isHttpError(err, 404)).toBe(true);
		expect(err).toMatchObject({ body: { missing: 'show' } });
	});
});

/** The page's HTML, as the server renderer writes it (comments removed). */
function html(episodes: ShowEpisodes = all, watched: EpisodeWatched[] | null = []): string {
	const props = { data: { show, episodes, watched, watchedError: watched ? '' : 'Server error' } };
	return render(Page, { props: props as never }).body.replace(/<!--[^]*?-->/g, '');
}

describe('all episodes page', () => {
	it("titles it with the show's years, on its backdrop and poster", () => {
		const body = html();
		expect(body).toMatch(/<h1[^>]*>\s*All episodes(&nbsp;|\u00a0)<span[^>]*>2008–2013<\/span>/);
		expect(body).toContain('src="https://img.test/backdrop.jpg"');
		expect(body).toContain('src="https://img.test/show.jpg"');
		const head = render(Page, {
			props: { data: { show, episodes: all, watched: [], watchedError: '' } } as never
		}).head;
		expect(head).toContain('<title>All episodes · Breaking Bad · watcht</title>');
	});

	it('is the current All chip at the end of the season strip, with an arrow back to specials', () => {
		const body = html();
		expect(body).toMatch(/<a href="\/shows\/1396\/seasons\/all"[^>]*aria-current="page"/);
		const hero = body.slice(0, body.indexOf('</header>'));
		expect(hero).toMatch(
			/<a href="\/shows\/1396\/seasons\/0"[^>]*aria-label="Previous season: Specials"/
		);
		expect(hero).not.toContain('aria-label="Next');
	});

	it('lists every episode, specials last, each linking to its own season', () => {
		const body = html();
		const list = body.slice(body.indexOf('id="episodes"'));
		const codes = [...list.matchAll(/<h3[^>]*>[^]*?(S\d\dE\d\d)<\/span>/g)].map((m) => m[1]);
		expect(codes).toEqual(['S01E01', 'S01E02', 'S02E01', 'S00E01']);
		expect(list).toContain('href="/shows/1396/seasons/0/episodes/1"');
		expect(list).toContain('href="/shows/1396/seasons/2/episodes/1"');
	});

	it('renders a long list a chunk at a time, keeping the full count', () => {
		// A long-running show: 30 seasons of 50, so 1,500 episodes.
		const many: ShowEpisodes = {
			show_tmdb_id: 1396,
			episodes_pending: false,
			episodes: Array.from({ length: 1500 }, (_, i) =>
				episode(Math.floor(i / 50) + 1, (i % 50) + 1)
			)
		};
		const body = html(many);
		const list = body.slice(body.indexOf('id="episodes"'));
		expect(list.match(/<h3[\s>]/g)).toHaveLength(100);
		expect(body).toMatch(/<h2[^>]*id="episodes-title"[^>]*>\s*1,500 Episodes\s*<\/h2>/);
		expect(body).toMatch(/href="#episodes"[^]*?>\s*1,500 Episodes\s*<\/span>/);
		expect(list).toMatch(/<button[^>]*>\s*Show 100 more\s*<\/button>/);
		expect(list).toContain('Showing 100 of 1,500 episodes');
		// Milestones still come out per season.
		expect(list).toContain('Series premiere');
		expect(list).toContain('Season premiere');
		expect(list).toContain('Season finale');
		// A short list has no button.
		expect(html()).not.toContain('Show 100 more');
	});

	it('keeps the episode list beside the sticky poster, outside the full-width last cell', () => {
		const body = html();
		expect(body).toContain('sm:sticky');
		expect(body).toMatch(
			/<div class="col-span-2 min-w-0 sm:col-span-1 sm:col-start-2 lg:col-span-2 lg:col-start-2 row-start-5">[^]*?<section[^>]*id="episodes"/
		);
		expect(body).not.toContain('col-span-full min-w-0 row-start-6');
	});

	it('counts the episodes first, with their total runtime', () => {
		const body = html();
		expect(body).toMatch(/href="#episodes"[^]*?>\s*4 Episodes\s*<\/span>/);
		expect(body).toMatch(/<h2[^>]*id="episodes-title"[^>]*>\s*4 Episodes\s*<\/h2>/);
		expect(body).toMatch(/<dt[^>]*>\s*Runtime\s*<\/dt>\s*<dd[^>]*>\s*2h 15m\s*<\/dd>/);
	});

	it('tracks the whole show, as the show page does', () => {
		const unwatched = html();
		expect(unwatched).toContain('id="mark-show"');
		const watched = html(all, [
			{
				tmdb_id: 60101,
				season_number: 1,
				plays: 1,
				last_watched_at: '2026-11-03T18:14:00Z',
				last_watched_on: null
			}
		]);
		expect(watched).toContain('id="show-progress"');
		expect(watched).toContain('aria-label="Remove or add plays of S01E01 “Episode 1.1”…"');
	});

	it('shows the fetching state while some season is still pending', () => {
		const body = html({ ...all, episodes_pending: true, episodes: null });
		expect(body).toContain('Fetching episodes from TMDB…');
		// The show's own count until they're in.
		expect(body).toMatch(/<h2[^>]*id="episodes-title"[^>]*>\s*6 Episodes\s*<\/h2>/);
	});
});
