import { listTokens } from '#lib/api/account.js';
import { loadError } from '#lib/load-error.js';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, depends }) => {
	// invalidate('app:tokens') after creating or revoking one.
	depends('app:tokens');
	try {
		return { tokens: await listTokens(fetch) };
	} catch (e) {
		loadError(e);
	}
};
