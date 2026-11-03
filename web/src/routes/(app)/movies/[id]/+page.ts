import { error } from '@sveltejs/kit';
import { getMovie } from '#lib/api/catalog.js';
import { movieWatches } from '#lib/api/tracking.js';
import { catalogLoadError, optionalLoad } from '#lib/load-error.js';
import { parseId } from '#lib/media.js';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params, fetch, depends }) => {
	depends('app:movie');
	const id = parseId(params.id);
	if (id === null) error(404, "There's no movie at this address.");
	try {
		// The user's watches come with it; marking or removing one reloads
		// just them (MovieActions), not the movie. If they fail, the movie
		// still shows, with the error in the action column.
		const [movie, watches] = await Promise.all([
			getMovie(id, fetch),
			optionalLoad(movieWatches(id, fetch))
		]);
		return { movie, watches: watches.value, watchesError: watches.error };
	} catch (e) {
		catalogLoadError(e, `TMDB has no movie with ID ${id}.`);
	}
};
