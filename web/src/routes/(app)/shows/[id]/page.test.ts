import { describe, expect, it } from 'vitest';
import { isHttpError, isRedirect } from '@sveltejs/kit';
import { render } from 'svelte/server';
import type { EpisodeWatched, Show } from '#lib/api/types.js';
import { load as layoutLoad } from './+layout';
import { load } from './+page';
import Page from './+page.svelte';

type LayoutEvent = Parameters<typeof layoutLoad>[0];
type LoadEvent = Parameters<typeof load>[0];

function layoutEvent(id: string, fetchFn: typeof fetch): LayoutEvent {
	return { params: { id }, fetch: fetchFn, depends: () => {} } as unknown as LayoutEvent;
}

function event(id: string, fetchFn: typeof fetch, search = ''): LoadEvent {
	return {
		params: { id },
		url: new URL(`http://watcht.test/shows/${id}${search}`),
		fetch: fetchFn,
		depends: () => {}
	} as unknown as LoadEvent;
}

const answer = (status: number, body: unknown) =>
	(async () =>
		new Response(JSON.stringify(body), {
			status,
			headers: { 'Content-Type': 'application/json' }
		})) as unknown as typeof fetch;

/** Answers each request by its path. */
const byPath = (bodies: Record<string, unknown>) =>
	(async (input: RequestInfo | URL) => {
		const path = String(input);
		if (!(path in bodies)) return new Response('{}', { status: 500 });
		return new Response(JSON.stringify(bodies[path]), {
			status: 200,
			headers: { 'Content-Type': 'application/json' }
		});
	}) as unknown as typeof fetch;

const fail = async (p: unknown) => Promise.resolve(p).catch((e: unknown) => e);

describe('show layout load', () => {
	it('loads the show by TMDB ID, for the show, season, and episode pages', async () => {
		const show = { tmdb_id: 1396, name: 'Breaking Bad', seasons: [] };
		const fetchFn = byPath({ '/api/v1/shows/1396': { show } });
		expect(await layoutLoad(layoutEvent('1396', fetchFn))).toEqual({ show });
	});

	it('is a 404 for an ID that is not a number, without asking the API', async () => {
		let called = false;
		const fetchFn = (async () => {
			called = true;
			return new Response('{}');
		}) as unknown as typeof fetch;
		const err = await fail(layoutLoad(layoutEvent('abc', fetchFn)));
		expect(isHttpError(err, 404)).toBe(true);
		expect(called).toBe(false);
	});

	it('is a 404 that names the ID when TMDB has no such show', async () => {
		const err = await fail(
			layoutLoad(layoutEvent('999', answer(404, { error: { code: 'not_found', message: 'x' } })))
		);
		expect(isHttpError(err, 404)).toBe(true);
		expect(err).toMatchObject({ body: { message: 'TMDB has no show with ID 999.' } });
	});

	it('explains a missing TMDB key', async () => {
		const body = { error: { code: 'tmdb_not_configured', message: 'x' } };
		const err = await fail(layoutLoad(layoutEvent('1396', answer(503, body))));
		expect(isHttpError(err, 503)).toBe(true);
		expect(err).toMatchObject({
			body: { message: expect.stringContaining('WATCHT_TMDB_API_KEY') }
		});
	});
});

describe('show page load', () => {
	it("loads the user's watched episodes by TMDB ID", async () => {
		const episodes = [
			{ tmdb_id: 62085, season_number: 1, plays: 2, last_watched_at: '2026-11-03T18:14:00Z' }
		];
		const fetchFn = byPath({ '/api/v1/shows/1396/watched': { episodes } });
		expect(await load(event('1396', fetchFn))).toEqual({ watched: episodes, watchedError: '' });
	});

	it("still shows the show when the watched episodes can't be loaded", async () => {
		expect(await load(event('1396', byPath({})))).toMatchObject({
			watched: null,
			watchedError: expect.any(String)
		});
	});

	it('is a 404 for an ID that is not a number, without asking the API', async () => {
		let called = false;
		const fetchFn = (async () => {
			called = true;
			return new Response('{}');
		}) as unknown as typeof fetch;
		const err = await fail(load(event('abc', fetchFn)));
		expect(isHttpError(err, 404)).toBe(true);
		expect(called).toBe(false);
	});

	it('sends an old ?season=N link to the season page', async () => {
		let called = false;
		const fetchFn = (async () => {
			called = true;
			return new Response('{}');
		}) as unknown as typeof fetch;
		const err = await fail(load(event('1396', fetchFn, '?season=2')));
		expect(isRedirect(err)).toBe(true);
		expect(err).toMatchObject({ status: 301, location: '/shows/1396/seasons/2' });
		expect(called).toBe(false);
		const specials = await fail(load(event('1396', fetchFn, '?season=0')));
		expect(specials).toMatchObject({ location: '/shows/1396/seasons/0' });
	});

	it('ignores a ?season= that is not a season number', async () => {
		const fetchFn = byPath({ '/api/v1/shows/1396/watched': { episodes: [] } });
		expect(await load(event('1396', fetchFn, '?season=abc'))).toEqual({
			watched: [],
			watchedError: ''
		});
	});
});

const breakingBad: Show = {
	tmdb_id: 1396,
	imdb_id: 'tt0903747',
	tvdb_id: 81189,
	name: 'Breaking Bad',
	original_name: 'Breaking Bad',
	original_language: 'en',
	overview: 'Walter White, a New Mexico chemistry teacher, is diagnosed with cancer.',
	tagline: 'Change the equation.',
	status: 'Ended',
	poster_path: '',
	poster_url: '',
	backdrop_path: '',
	backdrop_url: '',
	first_air_date: '2008-01-20',
	last_air_date: '2013-09-29',
	in_production: false,
	genres: [{ tmdb_id: 18, name: 'Drama' }],
	seasons: [],
	vote_average: 8.9,
	vote_count: 15000
};

const season = (season_number: number, episode_count: number, poster_url = '') => ({
	tmdb_id: 3570 + season_number,
	season_number,
	name: season_number === 0 ? 'Specials' : `Season ${season_number}`,
	overview: '',
	poster_path: '',
	poster_url,
	air_date: '2008-01-20',
	episode_count,
	episodes_pending: false
});

const withSeasons: Show = {
	...breakingBad,
	seasons: [season(0, 4), season(2, 13), season(1, 7, 'https://img.test/s1.jpg')]
};

const seen = (tmdb_id: number, season_number: number, plays = 1): EpisodeWatched => ({
	tmdb_id,
	season_number,
	plays,
	last_watched_at: '2026-11-03T18:14:00Z',
	last_watched_on: null
});

/** The show page's HTML, as the server renderer writes it (comments removed). */
function html(show: Show = breakingBad, watched: EpisodeWatched[] | null = []): string {
	const props = {
		data: { show, watched, watchedError: watched ? '' : 'Server error' }
	};
	return render(Page, { props: props as never }).body.replace(/<!--[^]*?-->/g, '');
}

/** Every button: its opening tag and its text, tags removed and spaces collapsed. */
const buttons = (body: string) =>
	[...body.matchAll(/(<button\b[^>]*>)([^]*?)<\/button>/g)].map(([, tag, inner]) => ({
		tag,
		text: inner
			.replace(/<[^>]*>/g, ' ')
			.replace(/\s+/g, ' ')
			.trim()
	}));

describe('show page', () => {
	it('keeps its facts and overview, without the ID facts', () => {
		const body = html();
		for (const label of ['Aired', 'Status', 'Genres', 'Language']) {
			expect(body).toMatch(new RegExp(`<dt[^>]*>\\s*${label}\\s*</dt>`));
		}
		expect(body).toContain('Change the equation.');
		expect(body).not.toMatch(/<dt[^>]*>\s*(IMDb|TMDB)\s*<\/dt>/);
		expect(body).not.toMatch(/>\s*tt0903747\s*</);
	});

	it('links to IMDb and TMDB as icon links', () => {
		const body = html();
		expect(body).toContain('href="https://www.imdb.com/title/tt0903747/"');
		expect(body).toContain('href="https://www.themoviedb.org/tv/1396"');
		expect(body).toContain('Breaking Bad on IMDb (opens in a new tab)');
		expect(body).toContain('Breaking Bad on TMDB (opens in a new tab)');
	});

	it("shows TMDB's score in the ratings band", () => {
		expect(html()).toContain('TMDB rating 8.9 out of 10, from 15,000 votes');
	});

	it('offers "Mark show watched" with a + until a regular episode is watched', () => {
		// Watched specials don't count towards the show.
		const body = html(withSeasons, [seen(1, 0)]);
		const mark = buttons(body).find((b) => b.text === 'Mark show watched');
		expect(mark?.tag).toContain('id="mark-show"');
		expect(body).toContain('aria-label="Choose when you watched Breaking Bad"');
		expect(body).toContain('except specials');
		expect(body).not.toMatch(/% watched/);
	});

	it('shows the progress button once something is watched: regular seasons, every play', () => {
		const body = html(withSeasons, [seen(1, 1, 2), seen(2, 1), seen(3, 2), seen(4, 0, 5)]);
		const progress = buttons(body).find((b) => b.tag.includes('id="show-progress"'));
		// 3 of 20 regular episodes is 15%; specials' plays aren't counted.
		expect(progress?.text).toBe('15% watched 3 of 20 episodes · 4 plays');
		expect(progress?.tag).toContain(
			'aria-label="15% watched: 3 of 20 episodes, 4 plays. Your history of Breaking Bad"'
		);
		expect(progress?.tag).toMatch(/popovertarget="[^"]+-menu"/);
		expect(body).toContain('aria-label="Mark the rest of Breaking Bad watched, choosing when"');
		expect(body).not.toContain('Mark show watched');
		// Its menu: the history link and confirmed removal of every play.
		expect(body).toMatch(/<a href="\/history"[^>]*data-item/);
		expect(body).toContain('View history');
		const removeAll = buttons(body).find((b) => b.tag.includes('aria-label="Remove all plays"'));
		expect(removeAll?.tag).toContain('aria-expanded="false"');
		expect(removeAll?.tag).not.toContain('aria-disabled');
		// Watch remaining, then Rewatch and Drop as placeholders.
		const remaining = buttons(body).find((b) => b.text.startsWith('Watch remaining'));
		expect(remaining?.tag).not.toContain('aria-disabled');
		for (const name of ['Rewatch', 'Drop']) {
			const row = buttons(body).find((b) => b.text === `${name} Coming in a later update`);
			expect(row?.tag).toContain('aria-disabled="true"');
		}
	});

	it('keeps the mark off while episodes are being fetched, and says why', () => {
		const fetching = { ...withSeasons, seasons: [{ ...season(1, 7), episodes_pending: true }] };
		for (const body of [html(fetching), html(fetching, [seen(1, 1)])]) {
			expect(body).toContain('Some episodes are still being fetched from TMDB.');
		}
	});

	it("shows the mark and the error when the watched episodes can't be loaded", () => {
		const body = html(withSeasons, null);
		expect(buttons(body).some((b) => b.text === 'Mark show watched')).toBe(true);
		expect(body).toContain("Couldn't load which episodes you've watched: Server error");
		expect(buttons(body).some((b) => b.text === 'Try again')).toBe(true);
	});

	it('shows watchlist, rating, and where to watch as placeholders in the action column', () => {
		const later = buttons(html()).filter(
			(b) => b.tag.includes('aria-disabled="true"') && !b.tag.includes('aria-label=')
		);
		expect(later.map((b) => b.text)).toEqual(['Add to watchlist', 'Rate it', 'Where to watch']);
	});

	it('has its sections in order, each in the section list', () => {
		const body = html(withSeasons);
		const ids = ['recent', 'people', 'seasons', 'stats', 'videos', 'recommendations'];
		let last = body.indexOf('id="overview"');
		expect(last).toBeGreaterThan(-1);
		for (const id of ids) {
			const at = body.search(new RegExp(`<section[^>]*id="${id}"`));
			expect(at).toBeGreaterThan(last);
			last = at;
			expect(body).toContain(`href="#${id}"`);
		}
		expect(body).toMatch(/<h2[^>]*id="recent-title"[^>]*>\s*Recent episodes\s*<\/h2>/);
		expect(body).toMatch(/<h3[^>]*>\s*Up next\s*<\/h3>/);
		expect(body).toMatch(/<h3[^>]*>\s*Recently aired\s*<\/h3>/);
		expect(body).toContain('come with show progress in a later update.');
	});

	it('lists the seasons as cards linking to their pages, specials last, with progress', () => {
		const body = html(withSeasons, [seen(1, 1), seen(2, 1), seen(3, 0)]);
		const grid = body.slice(body.indexOf('id="seasons"'), body.indexOf('id="stats"'));
		const links = [...grid.matchAll(/href="(\/shows\/1396\/seasons\/\d+)"/g)].map((m) => m[1]);
		expect(links).toEqual([
			'/shows/1396/seasons/1',
			'/shows/1396/seasons/2',
			'/shows/1396/seasons/0'
		]);
		expect(grid).toContain('2 of 7 watched');
		expect(grid).toContain('13 episodes');
		expect(grid).toContain('1 of 4 watched');
		// The season's own poster, else the show's.
		expect(grid).toContain('src="https://img.test/s1.jpg"');
		// No inline episode list on the show page.
		expect(body).not.toContain('?season=');
		// Each card's tooltip names the show and the season.
		expect(grid).toContain('title="Breaking Bad – Season 1"');
		expect(grid).toContain('title="Breaking Bad – Specials"');
	});

	it('counts the seasons first, specials left out, in the list and the heading', () => {
		const body = html(withSeasons);
		expect(body).toMatch(/href="#seasons"[^]*?>\s*2 Seasons\s*<\/span>\s*<svg/);
		expect(body).toMatch(/<h2[^>]*id="seasons-title"[^>]*>\s*2 Seasons\s*<\/h2>/);
		const one = { ...withSeasons, seasons: [season(1, 7), season(0, 4)] };
		expect(html(one)).toMatch(/<h2[^>]*id="seasons-title"[^>]*>\s*1 Season\s*<\/h2>/);
	});

	it('marks a season from its card: all of it, the rest, or not yet, and why', () => {
		const upcoming = { ...season(3, 8), air_date: '2099-01-01' };
		const pendingSeason = { ...season(4, 8), episodes_pending: true };
		const show = { ...withSeasons, seasons: [...withSeasons.seasons, upcoming, pendingSeason] };
		// Season 1: 1 of 7 watched; specials: all 4 watched; season 2: none.
		const watched = [seen(1, 1), ...[11, 12, 13, 14].map((id) => seen(id, 0))];
		const grid = html(show, watched).slice(html(show, watched).indexOf('id="seasons"'));
		const all = buttons(grid);
		// The check itself opens the when-choice: no separate chevron.
		const one = all.find((b) =>
			b.tag.includes('aria-label="Watch remaining episodes of Season 1…"')
		);
		expect(one?.tag).toContain('aria-haspopup="dialog"');
		expect(one?.tag).toContain('aria-expanded="false"');
		const two = all.find((b) => b.tag.includes('aria-label="Mark Season 2 watched…"'));
		expect(two?.tag).toContain('aria-haspopup="dialog"');
		expect(two?.tag).not.toContain('aria-disabled');
		expect(grid).not.toContain('Choose when you watched');
		// Partly watched is the sage tint; nothing watched isn't.
		expect(one?.tag).toContain('bg-watched-part');
		expect(two?.tag).not.toContain('bg-watched-part');
		// Every special watched: a button opens the shared removal menu.
		const done = all.find((b) => b.tag.includes('aria-label="Remove or add plays of Specials…"'));
		expect(done?.tag).toContain('aria-haspopup="dialog"');
		expect(done?.tag).toContain('bg-watched');
		expect(grid).toContain('Remove from history?');
		expect(grid).toContain('Watch again');
		expect(grid).toContain('Watch remaining');
		expect(all.some((b) => b.tag.includes('Mark Specials watched'))).toBe(false);
		// Off, with the reason as its description and tooltip.
		const three = all.find((b) => b.tag.includes('aria-label="Mark Season 3 watched…"'));
		expect(three?.tag).toContain('aria-disabled="true"');
		expect(three?.tag).toContain('title="Nothing in this season has aired yet."');
		const four = all.find((b) => b.tag.includes('aria-label="Mark Season 4 watched…"'));
		expect(four?.tag).toContain('aria-disabled="true"');
		expect(four?.tag).toMatch(/title="This season(&#39;|')s episodes are still being fetched/);
		// The watchlist placeholder on every card.
		expect(grid).toContain('aria-label="Add Season 2 to watchlist (coming in a later update)"');
	});

	it("has no season marks while the watched episodes can't be loaded", () => {
		const body = html(withSeasons, null);
		const grid = body.slice(body.indexOf('id="seasons"'), body.indexOf('id="stats"'));
		expect(grid).not.toMatch(/aria-label="Mark [^"]* watched"/);
	});

	it('has sort, filter, and hide-watched placeholders over the season grid', () => {
		const body = html(withSeasons);
		const grid = body.slice(body.indexOf('id="seasons"'), body.indexOf('id="stats"'));
		expect(grid).toContain('aria-label="Sort and filter seasons"');
		for (const label of ['Sort seasons by number', 'Hide watched seasons']) {
			expect(buttons(grid).find((b) => b.tag.includes(`aria-label="${label}"`))?.tag).toContain(
				'aria-disabled="true"'
			);
		}
		expect(grid).toMatch(
			/<input[^>]*readonly[^>]*aria-disabled="true"[^>]*placeholder="Filter by name"/
		);
	});
});
