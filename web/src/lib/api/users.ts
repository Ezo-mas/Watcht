// Admin user management. The server checks the caller is an admin; these
// fail with ApiError "forbidden" otherwise.
import { api } from './client';
import type { Account } from './types';

export async function listUsers(fetchFn?: typeof fetch): Promise<Account[]> {
	return (await api<{ users: Account[] }>('GET', '/api/v1/users', undefined, fetchFn)).users;
}

export async function createUser(
	username: string,
	password: string,
	isAdmin: boolean,
	fetchFn?: typeof fetch
): Promise<Account> {
	const body = { username, password, is_admin: isAdmin };
	return (await api<{ user: Account }>('POST', '/api/v1/users', body, fetchFn)).user;
}

/** Deletes the account and everything it owns. Not allowed on your own ("own_account"). */
export function deleteUser(id: number, fetchFn?: typeof fetch): Promise<void> {
	return api<void>('DELETE', `/api/v1/users/${id}`, undefined, fetchFn);
}

/** Sets another account's password and signs it out everywhere. */
export function resetPassword(id: number, password: string, fetchFn?: typeof fetch): Promise<void> {
	return api<void>('PUT', `/api/v1/users/${id}/password`, { password }, fetchFn);
}
