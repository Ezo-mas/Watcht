import { error } from '@sveltejs/kit';
import { getSeason } from '#lib/api/catalog.js';
import { episodeWatches } from '#lib/api/tracking.js';
import type { SeasonDetails } from '#lib/api/types.js';
import { catalogLoadError, optionalLoad } from '#lib/load-error.js';
import { episodeCode, parseSeasonParam, seasonLabel } from '#lib/media.js';
import type { PageLoad } from './$types';

// An episode, by its season and episode number in the address. It's found
// in its season's stored episodes; from there on everything (the plays,
// marks) uses its TMDB episode ID, never the numbers. While the server is
// still fetching the season's episodes, `episode` is null and the page
// polls, then reloads the show (invalidate('app:show')), which runs this
// again through parent().
export const load: PageLoad = async ({ params, fetch, parent }) => {
	const n = parseSeasonParam(params.n);
	const e = parseSeasonParam(params.e);
	if (n === null || e === null) {
		error(404, "There's no episode at this address.", { missing: 'episode' });
	}
	const { show } = await parent();
	const name = seasonLabel(n).toLowerCase();
	if (!show.seasons.some((s) => s.season_number === n)) {
		error(404, `TMDB lists no ${name} for ${show.name}.`, { missing: 'season' });
	}
	let season: SeasonDetails;
	try {
		season = await getSeason(show.tmdb_id, n, fetch);
	} catch (err) {
		catalogLoadError(err, `TMDB has no ${name} for ${show.name}.`, 'season');
	}
	if (season.episodes_pending) {
		return { season, episodeNumber: e, episode: null, watches: null, watchesError: '' };
	}
	const episode = season.episodes?.find((x) => x.episode_number === e);
	if (!episode) {
		error(404, `TMDB lists no episode ${episodeCode(n, e)} for ${show.name}.`, {
			missing: 'episode'
		});
	}
	const watches = await optionalLoad(episodeWatches(episode.tmdb_id, fetch));
	return {
		season,
		episodeNumber: e,
		episode,
		watches: watches.value,
		watchesError: watches.error
	};
};
