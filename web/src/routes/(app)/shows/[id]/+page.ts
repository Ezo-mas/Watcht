import { error, redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { showWatched } from '#lib/api/tracking.js';
import { optionalLoad } from '#lib/load-error.js';
import { parseId, parseSeasonParam } from '#lib/media.js';
import type { PageLoad } from './$types';

// The show itself comes from the layout. Which episodes the user watched
// comes with it; a mark reloads just that (the page's refreshWatched). If
// it fails, the show still shows, with the error in the action column.
export const load: PageLoad = async ({ params, url, fetch }) => {
	const id = parseId(params.id);
	if (id === null) error(404, "There's no show at this address.");
	// Seasons used to open here as ?season=N; they have their own pages now.
	const season = parseSeasonParam(url.searchParams.get('season'));
	if (season !== null) {
		redirect(301, resolve('/(app)/shows/[id]/seasons/[n]', { id: String(id), n: String(season) }));
	}
	const watched = await optionalLoad(showWatched(id, fetch));
	return { watched: watched.value, watchedError: watched.error };
};
