import { describe, expect, it, vi } from 'vitest';
import { changePassword, createToken, listTokens, revokeToken } from './account';
import { createUser, deleteUser, listUsers, resetPassword } from './users';

// Each wrapper must send exactly what the Go handlers expect
// (internal/httpapi/accounts.go and tokens.go).

function recorder(status: number, body?: unknown) {
	return vi.fn(
		async () =>
			new Response(body === undefined ? null : JSON.stringify(body), {
				status,
				headers: { 'Content-Type': 'application/json' }
			})
	) as unknown as typeof fetch & ReturnType<typeof vi.fn>;
}

function sent(fetchFn: ReturnType<typeof vi.fn>) {
	const [path, init] = fetchFn.mock.calls[0] as [string, RequestInit];
	return {
		path,
		method: init.method,
		body: init.body ? JSON.parse(init.body as string) : undefined
	};
}

describe('account endpoints', () => {
	it('changePassword', async () => {
		const f = recorder(204);
		await changePassword('old pw', 'new pw', f);
		expect(sent(f)).toEqual({
			path: '/api/v1/me/password',
			method: 'PUT',
			body: { current_password: 'old pw', new_password: 'new pw' }
		});
	});

	it('listTokens unwraps the list', async () => {
		const tokens = [{ id: 1, name: 'jellyfin', created_at: '2026-10-27T10:00:00Z' }];
		const f = recorder(200, { tokens });
		expect(await listTokens(f)).toEqual(tokens);
		expect(sent(f)).toMatchObject({ path: '/api/v1/tokens', method: 'GET' });
	});

	it('createToken sends the name and password and returns the secret', async () => {
		const created = {
			token: { id: 2, name: 'plex', created_at: '2026-10-27T10:00:00Z' },
			secret: 'wt_x'
		};
		const f = recorder(201, created);
		expect(await createToken('plex', 'my pw', f)).toEqual(created);
		expect(sent(f)).toEqual({
			path: '/api/v1/tokens',
			method: 'POST',
			body: { name: 'plex', password: 'my pw' }
		});
	});

	it('revokeToken', async () => {
		const f = recorder(204);
		await revokeToken(7, f);
		expect(sent(f)).toMatchObject({ path: '/api/v1/tokens/7', method: 'DELETE', body: undefined });
	});
});

describe('user endpoints', () => {
	it('listUsers unwraps the list', async () => {
		const users = [{ id: 1, username: 'a', is_admin: true, created_at: '2026-10-27T10:00:00Z' }];
		const f = recorder(200, { users });
		expect(await listUsers(f)).toEqual(users);
		expect(sent(f)).toMatchObject({ path: '/api/v1/users', method: 'GET' });
	});

	it('createUser', async () => {
		const user = { id: 2, username: 'b', is_admin: false, created_at: '2026-10-27T10:00:00Z' };
		const f = recorder(201, { user });
		expect(await createUser('b', 'pw', false, f)).toEqual(user);
		expect(sent(f)).toEqual({
			path: '/api/v1/users',
			method: 'POST',
			body: { username: 'b', password: 'pw', is_admin: false }
		});
	});

	it('deleteUser', async () => {
		const f = recorder(204);
		await deleteUser(3, f);
		expect(sent(f)).toMatchObject({ path: '/api/v1/users/3', method: 'DELETE' });
	});

	it('resetPassword', async () => {
		const f = recorder(204);
		await resetPassword(3, 'new pw', f);
		expect(sent(f)).toEqual({
			path: '/api/v1/users/3/password',
			method: 'PUT',
			body: { password: 'new pw' }
		});
	});
});
