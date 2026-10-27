// The signed-in user's own account: password and API tokens. These need a
// browser session; the server refuses them to API tokens.
import { api } from './client';
import type { APIToken, CreatedToken } from './types';

/** Ends every other session of this user. Wrong current password: ApiError "invalid_credentials". */
export function changePassword(
	currentPassword: string,
	newPassword: string,
	fetchFn?: typeof fetch
): Promise<void> {
	return api<void>(
		'PUT',
		'/api/v1/me/password',
		{ current_password: currentPassword, new_password: newPassword },
		fetchFn
	);
}

export async function listTokens(fetchFn?: typeof fetch): Promise<APIToken[]> {
	return (await api<{ tokens: APIToken[] }>('GET', '/api/v1/tokens', undefined, fetchFn)).tokens;
}

/** Needs the current password. Wrong password: ApiError "invalid_credentials". */
export function createToken(
	name: string,
	password: string,
	fetchFn?: typeof fetch
): Promise<CreatedToken> {
	return api<CreatedToken>('POST', '/api/v1/tokens', { name, password }, fetchFn);
}

export function revokeToken(id: number, fetchFn?: typeof fetch): Promise<void> {
	return api<void>('DELETE', `/api/v1/tokens/${id}`, undefined, fetchFn);
}
