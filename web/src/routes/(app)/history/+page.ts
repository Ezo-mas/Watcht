import { historyPage } from '#lib/api/tracking.js';
import { loadError } from '#lib/load-error.js';
import type { PageLoad } from './$types';

// The first page of history; "Load more" fetches the rest on the page.
export const load: PageLoad = async ({ fetch, depends }) => {
	depends('app:history');
	try {
		return { history: await historyPage(undefined, undefined, fetch) };
	} catch (e) {
		loadError(e);
	}
};
