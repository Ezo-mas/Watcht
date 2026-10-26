import type { ErrorBody } from './types';

/** An error response from the API, or a failure to reach it. */
export class ApiError extends Error {
	constructor(
		/** HTTP status, or 0 when the server couldn't be reached. */
		readonly status: number,
		/** The API's error code, e.g. "invalid_credentials". */
		readonly code: string,
		message: string
	) {
		super(message);
		this.name = 'ApiError';
	}
}

let onUnauthenticated: () => void = () => {};

/**
 * Sets what happens when a request finds the session gone (401
 * "unauthenticated"); the root layout re-checks the session, which sends
 * the user to /login.
 */
export function setUnauthenticatedHandler(handler: () => void): void {
	onUnauthenticated = handler;
}

/**
 * Calls the JSON API. Resolves with the decoded body (undefined for 204),
 * rejects with an ApiError. Cookies are same-origin only; there's no CORS.
 */
export async function api<T>(
	method: 'GET' | 'POST' | 'PUT' | 'DELETE',
	path: string,
	body?: unknown,
	fetchFn: typeof fetch = fetch
): Promise<T> {
	let res: Response;
	try {
		res = await fetchFn(path, {
			method,
			credentials: 'same-origin',
			headers: body === undefined ? {} : { 'Content-Type': 'application/json' },
			body: body === undefined ? undefined : JSON.stringify(body)
		});
	} catch {
		throw new ApiError(0, 'network', "Can't reach the watcht server. Check your connection.");
	}

	if (res.ok) {
		if (res.status === 204) return undefined as T;
		try {
			return (await res.json()) as T;
		} catch {
			// Not watcht's JSON: something in between (a misrouted proxy)
			// answered instead.
			throw new ApiError(
				0,
				'bad_response',
				"The server's answer wasn't from watcht. Check the proxy."
			);
		}
	}

	const err = await errorFrom(res);
	if (err.status === 401 && err.code === 'unauthenticated') onUnauthenticated();
	throw err;
}

async function errorFrom(res: Response): Promise<ApiError> {
	try {
		const body = (await res.json()) as Partial<ErrorBody>;
		if (body.error?.code) return new ApiError(res.status, body.error.code, body.error.message);
	} catch {
		// Not JSON: e.g. a reverse proxy's own error page.
	}
	return new ApiError(res.status, 'http_error', `The server answered ${res.status}.`);
}
