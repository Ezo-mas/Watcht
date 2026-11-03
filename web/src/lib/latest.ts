/**
 * Keeps only the newest of overlapping requests for the same data. Wrap
 * each request with the function this returns: it resolves with
 * `{ current: true, value }` for the latest one started, and
 * `{ current: false }` for any older one, whether that succeeded or failed,
 * so a slow early answer can't overwrite a newer one. An error from the
 * latest request is thrown.
 */
export function latestOnly<T>(): (
	request: Promise<T>
) => Promise<{ current: true; value: T } | { current: false }> {
	let latest = 0;
	return async (request) => {
		const mine = ++latest;
		try {
			const value = await request;
			return mine === latest ? { current: true, value } : { current: false };
		} catch (e) {
			if (mine !== latest) return { current: false };
			throw e;
		}
	};
}
