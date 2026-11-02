import { error } from '@sveltejs/kit';
import { getShow } from '#lib/api/catalog.js';
import { catalogLoadError } from '#lib/load-error.js';
import { parseId } from '#lib/media.js';
import type { PageLoad } from './$types';

// The season shown (?season=N) isn't read here, so switching seasons
// doesn't reload the show; Seasons.svelte fetches the season itself.
export const load: PageLoad = async ({ params, fetch, depends }) => {
	depends('app:show');
	const id = parseId(params.id);
	if (id === null) error(404, "There's no show at this address.");
	try {
		return { show: await getShow(id, fetch) };
	} catch (e) {
		catalogLoadError(e, `TMDB has no show with ID ${id}.`);
	}
};
