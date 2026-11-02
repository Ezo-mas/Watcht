import type { SearchResult, SearchResults } from './api/types';

/** Searches start at this many characters. */
export const minQueryLength = 2;

export interface SearchState {
	/** The query the state is for, trimmed with runs of spaces collapsed. */
	query: string;
	/**
	 * idle: the query is too short to search; loading: a first page is on
	 * its way (results still hold the previous query's until it lands).
	 */
	status: 'idle' | 'loading' | 'done' | 'error';
	results: SearchResult[];
	page: number;
	totalPages: number;
	totalResults: number;
	/** Loading the next page, independent of status. */
	more: 'idle' | 'loading' | 'error';
	/** The failure behind status "error" or more "error". */
	error: unknown;
}

export const initialSearchState: SearchState = {
	query: '',
	status: 'idle',
	results: [],
	page: 0,
	totalPages: 0,
	totalResults: 0,
	more: 'idle',
	error: null
};

/** Trims a query and collapses whitespace, as the server does. */
export function normalizeQuery(query: string): string {
	return query.trim().replace(/\s+/g, ' ');
}

type SearchFn = (query: string, page: number) => Promise<SearchResults>;

/**
 * Runs title searches as someone types: debounced, only from
 * `minQueryLength` characters, and answers to an older query (which can
 * arrive after a newer one's) are dropped. It holds no UI state of its
 * own; every change goes to `onChange` as a fresh object.
 */
export class Searcher {
	#search: SearchFn;
	#onChange: (state: SearchState) => void;
	#delay: number;
	#state: SearchState = initialSearchState;
	#timer: ReturnType<typeof setTimeout> | undefined;
	/** Bumped for every new query; a response for an older one is ignored. */
	#generation = 0;

	constructor(search: SearchFn, onChange: (state: SearchState) => void, delay = 300) {
		this.#search = search;
		this.#onChange = onChange;
		this.#delay = delay;
	}

	get state(): SearchState {
		return this.#state;
	}

	/** The input changed: search after a pause in typing. */
	set(query: string): void {
		this.#schedule(query, this.#delay);
	}

	/** Search straight away (a query from the URL, or Enter). */
	now(query: string): void {
		this.#schedule(query, 0);
	}

	/** Loads the next page and appends it. */
	async more(): Promise<void> {
		const s = this.#state;
		if (s.status !== 'done' || s.page >= s.totalPages || s.more === 'loading') return;
		const generation = this.#generation;
		this.#emit({ ...s, more: 'loading', error: null });
		try {
			const res = await this.#search(s.query, s.page + 1);
			if (generation !== this.#generation) return;
			this.#emit({
				...this.#state,
				results: unique([...this.#state.results, ...res.results]),
				page: res.page,
				// Keep what the first page said; an empty last page can report 0.
				totalPages: Math.max(res.total_pages, res.page),
				more: 'idle'
			});
		} catch (e) {
			if (generation !== this.#generation) return;
			this.#emit({ ...this.#state, more: 'error', error: e });
		}
	}

	/** Stops a pending search; nothing is reported afterwards. */
	dispose(): void {
		clearTimeout(this.#timer);
		this.#generation++;
	}

	#schedule(raw: string, delay: number): void {
		const query = normalizeQuery(raw);
		const s = this.#state;
		if (query === s.query && delay > 0 && (s.status === 'loading' || s.status === 'done')) return;
		clearTimeout(this.#timer);
		const generation = ++this.#generation;
		if ([...query].length < minQueryLength) {
			this.#emit({ ...initialSearchState, query });
			return;
		}
		this.#emit({ ...s, query, status: 'loading', more: 'idle', error: null });
		const run = () => void this.#run(query, generation);
		if (delay > 0) this.#timer = setTimeout(run, delay);
		else run();
	}

	async #run(query: string, generation: number): Promise<void> {
		try {
			const res = await this.#search(query, 1);
			if (generation !== this.#generation) return;
			this.#emit({
				query,
				status: 'done',
				results: unique(res.results),
				page: res.page,
				totalPages: res.total_pages,
				totalResults: res.total_results,
				more: 'idle',
				error: null
			});
		} catch (e) {
			if (generation !== this.#generation) return;
			this.#emit({ ...initialSearchState, query, status: 'error', error: e });
		}
	}

	#emit(state: SearchState): void {
		this.#state = state;
		this.#onChange(state);
	}
}

/**
 * Drops repeated results, keeping the first: TMDB pages can overlap, and
 * a page can list the same title twice. The list is keyed by `key`, so a
 * repeat would break it.
 */
function unique(results: SearchResult[]): SearchResult[] {
	const seen = new Set<string>();
	return results.filter((r) => {
		const k = key(r);
		if (seen.has(k)) return false;
		seen.add(k);
		return true;
	});
}

/** Identifies a result across pages: a movie and a show can share an ID. */
export function key(r: Pick<SearchResult, 'media_type' | 'tmdb_id'>): string {
	return `${r.media_type}-${r.tmdb_id}`;
}
