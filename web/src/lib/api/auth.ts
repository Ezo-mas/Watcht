import { api } from './client';
import type { SessionStatus, User } from './types';

export function getSession(fetchFn?: typeof fetch): Promise<SessionStatus> {
	return api<SessionStatus>('GET', '/api/v1/auth/session', undefined, fetchFn);
}

export async function setup(username: string, password: string): Promise<User> {
	return (await api<{ user: User }>('POST', '/api/v1/auth/setup', { username, password })).user;
}

export async function login(username: string, password: string): Promise<User> {
	return (await api<{ user: User }>('POST', '/api/v1/auth/login', { username, password })).user;
}

export function logout(): Promise<void> {
	return api<void>('POST', '/api/v1/auth/logout');
}
