import { afterEach, describe, expect, it, vi } from 'vitest';
import { api, ApiError, setUnauthenticatedHandler } from './client';

function respond(status: number, body?: unknown, contentType = 'application/json'): typeof fetch {
	return vi.fn(
		async () =>
			new Response(
				body === undefined ? null : typeof body === 'string' ? body : JSON.stringify(body),
				{
					status,
					headers: { 'Content-Type': contentType }
				}
			)
	) as unknown as typeof fetch;
}

afterEach(() => setUnauthenticatedHandler(() => {}));

describe('api', () => {
	it('sends JSON and decodes the response', async () => {
		const fetchFn = respond(200, { user: { id: 1 } });
		const got = await api<{ user: { id: number } }>('POST', '/api/v1/x', { a: 1 }, fetchFn);
		expect(got.user.id).toBe(1);
		const [path, init] = vi.mocked(fetchFn).mock.calls[0];
		expect(path).toBe('/api/v1/x');
		expect(init?.method).toBe('POST');
		expect(init?.credentials).toBe('same-origin');
		expect(init?.headers).toEqual({ 'Content-Type': 'application/json' });
		expect(init?.body).toBe('{"a":1}');
	});

	it('sends no body or content type without one', async () => {
		const fetchFn = respond(200, {});
		await api('GET', '/api/v1/x', undefined, fetchFn);
		const init = vi.mocked(fetchFn).mock.calls[0][1];
		expect(init?.body).toBeUndefined();
		expect(init?.headers).toEqual({});
	});

	it('resolves 204 with undefined', async () => {
		await expect(api('POST', '/api/v1/x', undefined, respond(204))).resolves.toBeUndefined();
	});

	it('turns the standard error shape into an ApiError', async () => {
		const fetchFn = respond(400, { error: { code: 'invalid_input', message: 'bad name' } });
		const err = await api('POST', '/api/v1/x', {}, fetchFn).catch((e: unknown) => e);
		expect(err).toBeInstanceOf(ApiError);
		expect(err).toMatchObject({ status: 400, code: 'invalid_input', message: 'bad name' });
	});

	it('handles a non-JSON error page', async () => {
		const err = await api(
			'GET',
			'/api/v1/x',
			undefined,
			respond(502, '<html>Bad Gateway</html>', 'text/html')
		).catch((e: unknown) => e);
		expect(err).toMatchObject({ status: 502, code: 'http_error' });
	});

	it('handles JSON without the error shape', async () => {
		const err = await api('GET', '/api/v1/x', undefined, respond(500, { message: 'oops' })).catch(
			(e: unknown) => e
		);
		expect(err).toMatchObject({ status: 500, code: 'http_error' });
	});

	it('reports a success response that is not JSON as unreachable', async () => {
		const err = await api(
			'GET',
			'/api/v1/x',
			undefined,
			respond(200, '<html>login portal</html>', 'text/html')
		).catch((e: unknown) => e);
		expect(err).toMatchObject({ status: 0, code: 'bad_response' });
	});

	it('reports a network failure', async () => {
		const fetchFn = vi.fn(async () => {
			throw new TypeError('Failed to fetch');
		}) as unknown as typeof fetch;
		const err = await api('GET', '/api/v1/x', undefined, fetchFn).catch((e: unknown) => e);
		expect(err).toMatchObject({ status: 0, code: 'network' });
	});

	it('calls the unauthenticated handler only for a lost session', async () => {
		const handler = vi.fn();
		setUnauthenticatedHandler(handler);
		await api(
			'GET',
			'/x',
			undefined,
			respond(401, { error: { code: 'unauthenticated', message: '' } })
		).catch(() => {});
		expect(handler).toHaveBeenCalledTimes(1);
		// A wrong password at login is a 401 too, but not a lost session.
		await api(
			'POST',
			'/x',
			{},
			respond(401, { error: { code: 'invalid_credentials', message: '' } })
		).catch(() => {});
		await api(
			'GET',
			'/x',
			undefined,
			respond(403, { error: { code: 'forbidden', message: '' } })
		).catch(() => {});
		expect(handler).toHaveBeenCalledTimes(1);
	});
});
