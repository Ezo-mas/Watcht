<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { login } from '#lib/api/auth.js';
	import { ApiError } from '#lib/api/client.js';
	import { safeNext } from '#lib/guard.js';
	import Button from '#lib/components/Button.svelte';
	import CardPage from '#lib/components/CardPage.svelte';
	import Field from '#lib/components/Field.svelte';
	import type { PathnameWithSearchOrHash } from '$app/types';

	let username = $state('');
	let password = $state('');
	let error = $state('');
	let busy = $state(false);

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		busy = true;
		error = '';
		try {
			await login(username, password);
		} catch (e) {
			error = e instanceof ApiError ? e.message : 'Something went wrong.';
			password = '';
			busy = false;
			return;
		}
		// Signed in. Read ?next= before navigating: once the session check
		// knows we're signed in, it moves away from /login. safeNext only
		// returns a path on this site ("/history"); resolve() takes it
		// without the leading slash and adds any base path.
		const next = safeNext(page.url.searchParams.get('next')).slice(1);
		const session = { invalidate: ['app:session'] };
		try {
			await goto(resolve(next as PathnameWithSearchOrHash), session);
		} catch {
			// The page is gone (an old bookmark): go to the dashboard instead.
			await goto(resolve('/(app)'), session);
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head>
	<title>Sign in · watcht</title>
</svelte:head>

<CardPage title="Sign in">
	<form onsubmit={submit} class="flex flex-col gap-4">
		<Field
			label="Username"
			name="username"
			autocomplete="username"
			autocapitalize="none"
			spellcheck="false"
			required
			bind:value={username}
		/>
		<Field
			label="Password"
			name="password"
			type="password"
			autocomplete="current-password"
			required
			bind:value={password}
		/>
		{#if error}
			<p
				role="alert"
				class="rounded-md border border-danger/40 bg-danger/10 px-3 py-2 text-sm text-danger first-letter:uppercase"
			>
				{error}
			</p>
		{/if}
		<Button type="submit" {busy} class="mt-1 w-full">{busy ? 'Signing in…' : 'Sign in'}</Button>
	</form>
</CardPage>
