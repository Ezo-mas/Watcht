import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { SearchResult, SearchResults } from './api/types';
import { Searcher, normalizeQuery, type SearchState } from './search';

function result(id: number, media_type: 'movie' | 'show' = 'movie'): SearchResult {
	return {
		media_type,
		tmdb_id: id,
		title: `t${id}`,
		original_title: '',
		overview: '',
		date: '',
		poster_path: '',
		poster_url: '',
		vote_average: 0
	};
}

function page(n: number, total: number, ...ids: number[]): SearchResults {
	return {
		page: n,
		total_pages: total,
		total_results: ids.length,
		results: ids.map((i) => result(i))
	};
}

// A search whose answers the test releases by hand, in any order.
function manual() {
	const pending: {
		q: string;
		p: number;
		resolve: (r: SearchResults) => void;
		reject: (e: unknown) => void;
	}[] = [];
	const search = vi.fn(
		(q: string, p: number) =>
			new Promise<SearchResults>((resolve, reject) => pending.push({ q, p, resolve, reject }))
	);
	const states: SearchState[] = [];
	const s = new Searcher(search, (st) => states.push(st));
	const last = () => states[states.length - 1];
	return { s, search, pending, last };
}

describe('Searcher', () => {
	beforeEach(() => vi.useFakeTimers());
	afterEach(() => vi.useRealTimers());

	it('waits for a pause in typing and searches once', async () => {
		const m = manual();
		m.s.set('ma');
		await vi.advanceTimersByTimeAsync(200);
		m.s.set('mat');
		await vi.advanceTimersByTimeAsync(200);
		m.s.set('matrix');
		expect(m.last().status).toBe('loading');
		await vi.advanceTimersByTimeAsync(299);
		expect(m.search).not.toHaveBeenCalled();
		await vi.advanceTimersByTimeAsync(1);
		expect(m.search).toHaveBeenCalledExactlyOnceWith('matrix', 1);
		m.pending[0].resolve(page(1, 1, 603));
		await vi.advanceTimersByTimeAsync(0);
		expect(m.last()).toMatchObject({
			query: 'matrix',
			status: 'done',
			results: [{ tmdb_id: 603 }]
		});
	});

	it('does not search under two characters', async () => {
		const m = manual();
		m.s.set('m');
		m.s.now(' m ');
		await vi.runAllTimersAsync();
		expect(m.search).not.toHaveBeenCalled();
		expect(m.last()).toMatchObject({ query: 'm', status: 'idle', results: [] });
	});

	it('drops an answer to an older query that arrives late', async () => {
		const m = manual();
		m.s.now('first');
		m.s.now('second');
		m.pending[1].resolve(page(1, 1, 2));
		await vi.advanceTimersByTimeAsync(0);
		m.pending[0].resolve(page(1, 1, 1));
		await vi.advanceTimersByTimeAsync(0);
		expect(m.last()).toMatchObject({ query: 'second', results: [{ tmdb_id: 2 }] });
	});

	it('drops a late failure for an older query', async () => {
		const m = manual();
		m.s.now('first');
		m.s.now('second');
		m.pending[1].resolve(page(1, 1, 2));
		m.pending[0].reject(new Error('late'));
		await vi.advanceTimersByTimeAsync(0);
		expect(m.last()).toMatchObject({ status: 'done', error: null });
	});

	it('cancels a scheduled search when the query gets too short', async () => {
		const m = manual();
		m.s.set('matrix');
		m.s.set('m');
		await vi.runAllTimersAsync();
		expect(m.search).not.toHaveBeenCalled();
		expect(m.last().status).toBe('idle');
	});

	it('appends the next page without repeating results', async () => {
		const m = manual();
		m.s.now('matrix');
		m.pending[0].resolve(page(1, 2, 1, 2));
		await vi.advanceTimersByTimeAsync(0);
		const more = m.s.more();
		expect(m.last().more).toBe('loading');
		m.pending[1].resolve(page(2, 2, 2, 3));
		await more;
		expect(m.search).toHaveBeenLastCalledWith('matrix', 2);
		expect(m.last().results.map((r) => r.tmdb_id)).toEqual([1, 2, 3]);
		expect(m.last()).toMatchObject({ page: 2, more: 'idle' });
		await m.s.more(); // last page: nothing more to load
		expect(m.search).toHaveBeenCalledTimes(2);
	});

	it('ignores a next page that lands after a new query', async () => {
		const m = manual();
		m.s.now('matrix');
		m.pending[0].resolve(page(1, 2, 1));
		await vi.advanceTimersByTimeAsync(0);
		const more = m.s.more();
		m.s.now('alien');
		m.pending[1].resolve(page(2, 2, 9));
		await more;
		expect(m.last()).toMatchObject({ query: 'alien', status: 'loading' });
		expect(m.last().results.some((r) => r.tmdb_id === 9)).toBe(false);
	});

	it('reports a failed search', async () => {
		const m = manual();
		m.s.now('matrix');
		m.pending[0].reject(new Error('down'));
		await vi.advanceTimersByTimeAsync(0);
		expect(m.last()).toMatchObject({ status: 'error', error: new Error('down') });
	});

	it('drops a title repeated within a page and across pages', async () => {
		const m = manual();
		m.s.now('matrix');
		m.pending[0].resolve(page(1, 2, 1, 2, 1));
		await vi.advanceTimersByTimeAsync(0);
		expect(m.last().results.map((r) => r.tmdb_id)).toEqual([1, 2]);
		const more = m.s.more();
		m.pending[1].resolve(page(2, 2, 3, 3, 2));
		await more;
		expect(m.last().results.map((r) => r.tmdb_id)).toEqual([1, 2, 3]);
	});

	it('keeps a movie and a show with the same ID', async () => {
		const m = manual();
		m.s.now('matrix');
		m.pending[0].resolve({
			page: 1,
			total_pages: 1,
			total_results: 2,
			results: [result(7, 'movie'), result(7, 'show')]
		});
		await vi.advanceTimersByTimeAsync(0);
		expect(m.last().results).toHaveLength(2);
	});

	it('does not search again when typing leaves the query the same', async () => {
		const m = manual();
		m.s.now('matrix');
		m.pending[0].resolve(page(1, 1, 603));
		await vi.advanceTimersByTimeAsync(0);
		m.s.set('matrix '); // a trailing space normalizes to the same query
		expect(m.last().status).toBe('done');
		await vi.runAllTimersAsync();
		expect(m.search).toHaveBeenCalledTimes(1);
	});

	it('loads the next page once when asked twice', async () => {
		const m = manual();
		m.s.now('matrix');
		m.pending[0].resolve(page(1, 3, 1));
		await vi.advanceTimersByTimeAsync(0);
		const first = m.s.more();
		const second = m.s.more();
		expect(m.search).toHaveBeenCalledTimes(2);
		m.pending[1].resolve(page(2, 3, 2));
		await Promise.all([first, second]);
		expect(m.last()).toMatchObject({ page: 2, more: 'idle' });
	});

	it('normalizes queries like the server', () => {
		expect(normalizeQuery('  the   matrix ')).toBe('the matrix');
	});
});
