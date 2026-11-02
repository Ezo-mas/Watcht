import { error } from '@sveltejs/kit';
import { getMovie } from '#lib/api/catalog.js';
import { catalogLoadError } from '#lib/load-error.js';
import { parseId } from '#lib/media.js';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params, fetch, depends }) => {
	depends('app:movie');
	const id = parseId(params.id);
	if (id === null) error(404, "There's no movie at this address.");
	try {
		return { movie: await getMovie(id, fetch) };
	} catch (e) {
		catalogLoadError(e, `TMDB has no movie with ID ${id}.`);
	}
};
