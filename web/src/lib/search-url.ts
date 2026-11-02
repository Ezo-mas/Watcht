import { normalizeQuery } from './search';

/**
 * Keeps the /search page's box and its ?q= in step without one
 * overwriting the other. The page writes the box's query into the URL
 * after a pause in typing; that navigation lands a moment later (after
 * the layout's session check), by which time the box may have moved on.
 * So a navigation that lands on the query the page last wrote leaves the
 * box alone, and any other one (Back, Forward, a link, the overlay's
 * "see all") puts its query in the box.
 *
 * Only the last write matters: a newer goto supersedes an older one, and
 * SvelteKit never reports the superseded navigation as landed.
 */
export class QuerySync {
	/** The query last written to the URL whose navigation hasn't landed yet. */
	#pending: string | null = null;

	/** The page is about to write this query into the URL. */
	wrote(query: string): void {
		this.#pending = normalizeQuery(query);
	}

	/**
	 * The query the URL holds or is about to hold: the pending write if
	 * there is one, else `urlQuery`. Compare the box against this, not the
	 * URL, which is stale while a write is in flight.
	 */
	target(urlQuery: string): string {
		return this.#pending ?? normalizeQuery(urlQuery);
	}

	/**
	 * A navigation landed on ?q=`urlQuery` while the box holds `boxQuery`:
	 * returns the query to put in the box, or null to leave it as it is.
	 */
	arrived(urlQuery: string, boxQuery: string): string | null {
		const url = normalizeQuery(urlQuery);
		const ours = this.#pending === url;
		this.#pending = null;
		if (ours) return null;
		return url === normalizeQuery(boxQuery) ? null : urlQuery;
	}
}
