import { error } from '@sveltejs/kit';
import { getShow } from '#lib/api/catalog.js';
import { catalogLoadError } from '#lib/load-error.js';
import { parseId } from '#lib/media.js';
import type { LayoutLoad } from './$types';

// The show, for its page and its season and episode pages: moving between
// seasons or episodes doesn't fetch it again. Opening it also stores the
// show, which the season and episode reads need, so they wait for it
// (parent()). invalidate('app:show') reloads it.
export const load: LayoutLoad = async ({ params, fetch, depends }) => {
	depends('app:show');
	const id = parseId(params.id);
	if (id === null) error(404, "There's no show at this address.", { missing: 'show' });
	try {
		return { show: await getShow(id, fetch) };
	} catch (e) {
		catalogLoadError(e, `TMDB has no show with ID ${id}.`, 'show');
	}
};
