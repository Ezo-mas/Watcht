import { error } from '@sveltejs/kit';
import { getShow } from '#lib/api/catalog.js';
import { showWatched } from '#lib/api/tracking.js';
import { catalogLoadError, optionalLoad } from '#lib/load-error.js';
import { parseId } from '#lib/media.js';
import type { PageLoad } from './$types';

// The season shown (?season=N) isn't read here, so switching seasons
// doesn't reload the show; Seasons.svelte fetches the season itself.
export const load: PageLoad = async ({ params, fetch, depends }) => {
	depends('app:show');
	const id = parseId(params.id);
	if (id === null) error(404, "There's no show at this address.");
	try {
		// Which episodes the user watched comes with it; a mark reloads just
		// that (the page's refreshWatched), not the show. If it fails, the
		// show still shows, with the error in the action column.
		const [show, watched] = await Promise.all([
			getShow(id, fetch),
			optionalLoad(showWatched(id, fetch))
		]);
		return { show, watched: watched.value, watchedError: watched.error };
	} catch (e) {
		catalogLoadError(e, `TMDB has no show with ID ${id}.`);
	}
};
