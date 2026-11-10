import { getShowEpisodes } from '#lib/api/catalog.js';
import { showWatched } from '#lib/api/tracking.js';
import { catalogLoadError, optionalLoad } from '#lib/load-error.js';
import type { PageLoad } from './$types';

// Every episode of a show, specials included, in one request: the show
// from the layout (which also stores it, and starts fetching any season
// whose episodes aren't stored yet), then its episodes and which of them
// the user watched (a failure there leaves the page up with the error in
// the action column). While any season is still being fetched, the
// episodes come back pending and the page polls for them. A static route,
// so "all" never reaches the [n] route's number check.
export const load: PageLoad = async ({ fetch, parent }) => {
	const { show } = await parent();
	try {
		const [episodes, watched] = await Promise.all([
			getShowEpisodes(show.tmdb_id, fetch),
			optionalLoad(showWatched(show.tmdb_id, fetch))
		]);
		return { episodes, watched: watched.value, watchedError: watched.error };
	} catch (e) {
		catalogLoadError(e, `TMDB has no show with ID ${show.tmdb_id}.`, 'show');
	}
};
