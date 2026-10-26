import { describe, expect, it } from 'vitest';
import { guard, safeNext } from './guard';
import type { SessionStatus } from './api/types';

const fresh: SessionStatus = { setup_required: true, user: null };
const signedOut: SessionStatus = { setup_required: false, user: null };
const signedIn: SessionStatus = {
	setup_required: false,
	user: { id: 1, username: 'a', is_admin: true }
};

const at = (path: string) => new URL(path, 'http://localhost');

describe('guard', () => {
	it.each([
		// before setup, everything goes to /setup
		[fresh, '/', '/setup'],
		[fresh, '/login', '/setup'],
		[fresh, '/history?page=2', '/setup'],
		[fresh, '/setup', null],
		// signed out: /login, remembering where they were going
		[signedOut, '/', '/login'],
		[signedOut, '/login', null],
		[signedOut, '/setup', '/login'],
		[signedOut, '/history?page=2', '/login?next=%2Fhistory%3Fpage%3D2'],
		// signed in: the sign-in pages go to the dashboard
		[signedIn, '/', null],
		[signedIn, '/history', null],
		[signedIn, '/login', '/'],
		[signedIn, '/setup', '/']
	])('%o at %s goes to %s', (session, path, want) => {
		expect(guard(session, at(path))).toBe(want);
	});
});

describe('safeNext', () => {
	it.each([
		[null, '/'],
		['', '/'],
		['/history', '/history'],
		['/history?page=2#top', '/history?page=2#top'],
		['/a/../b', '/b'],
		// anything that leaves the site goes to the dashboard instead
		['//evil.example', '/'],
		['//evil.example/path', '/'],
		['/\\evil.example', '/'],
		['https://evil.example', '/'],
		['javascript:alert(1)', '/'],
		['history', '/'],
		['/%2F%2Fevil.example', '/%2F%2Fevil.example']
	])('%s → %s', (next, want) => {
		expect(safeNext(next)).toBe(want);
	});
});
