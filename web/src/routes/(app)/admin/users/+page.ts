import { redirect } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { listUsers } from '#lib/api/users.js';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, depends, parent }) => {
	// Only admins get here; the server refuses everyone else anyway.
	const { session } = await parent();
	if (!session.user?.is_admin) redirect(307, resolve('/(app)/settings'));
	// invalidate('app:users') after creating, deleting, or resetting.
	depends('app:users');
	return { users: await listUsers(fetch) };
};
