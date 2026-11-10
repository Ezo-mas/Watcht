import { error } from '@sveltejs/kit';
import { getSeason } from '#lib/api/catalog.js';
import { showWatched } from '#lib/api/tracking.js';
import { catalogLoadError, optionalLoad } from '#lib/load-error.js';
import { parseSeasonParam, seasonLabel } from '#lib/media.js';
import type { PageLoad } from './$types';

// A season of a show: the show from the layout, the season with its
// episodes, and which of them the user watched (as on the show page, a
// failure there leaves the page up with the error in the action column).
// While the server is still fetching the episodes from TMDB, the season
// comes back pending and the page polls for it.
export const load: PageLoad = async ({ params, fetch, parent }) => {
	const n = parseSeasonParam(params.n);
	if (n === null) {
		error(404, "There's no season at this address.", { missing: 'season' });
	}
	// The show is stored once the layout has it, and the season read needs that.
	const { show } = await parent();
	const name = seasonLabel(n).toLowerCase();
	if (!show.seasons.some((s) => s.season_number === n)) {
		error(404, `TMDB lists no ${name} for ${show.name}.`, { missing: 'season' });
	}
	try {
		const [season, watched] = await Promise.all([
			getSeason(show.tmdb_id, n, fetch),
			optionalLoad(showWatched(show.tmdb_id, fetch))
		]);
		return { season, watched: watched.value, watchedError: watched.error };
	} catch (e) {
		catalogLoadError(e, `TMDB has no ${name} for ${show.name}.`, 'season');
	}
};
