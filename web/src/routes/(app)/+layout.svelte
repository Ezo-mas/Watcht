<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { logout } from '#lib/api/auth.js';
	import { ApiError } from '#lib/api/client.js';
	import Button from '#lib/components/Button.svelte';
	import Wordmark from '#lib/components/Wordmark.svelte';
	import type { LayoutProps } from './$types';

	let { data, children }: LayoutProps = $props();
	const user = $derived(data.session.user);

	let signingOut = $state(false);
	let signOutError = $state('');

	// Only leave once the server has ended the session (or it was already
	// gone): on a shared device, "signed out" must be true.
	async function signOut() {
		signingOut = true;
		signOutError = '';
		try {
			await logout();
		} catch (e) {
			if (!(e instanceof ApiError && e.status === 401)) {
				signOutError = "Couldn't sign out. Try again.";
				signingOut = false;
				return;
			}
		}
		await goto(resolve('/login'), { invalidate: ['app:session'] });
		signingOut = false;
	}
</script>

<div class="flex min-h-dvh flex-col">
	<header class="sticky top-0 z-10 border-b border-line bg-surface">
		<div class="mx-auto flex h-12 w-full max-w-7xl items-center gap-6 px-4 sm:px-6">
			<a href={resolve('/(app)')} class="-mx-1 rounded-sm px-1 py-1">
				<Wordmark />
			</a>
			<!-- Search, History, Watchlist and Settings go here as their routes arrive. -->
			<div class="flex-1"></div>
			{#if user}
				<div class="flex min-w-0 items-center gap-3 text-sm">
					<span class="flex max-w-64 min-w-0 items-center gap-2" title={user.username}>
						<span
							aria-hidden="true"
							class="grid size-6 shrink-0 place-items-center rounded-full bg-accent/20 text-xs font-semibold text-accent-text uppercase"
						>
							{user.username.charAt(0)}
						</span>
						<span class="truncate text-ink"
							><span class="sr-only">Signed in as </span>{user.username}</span
						>
					</span>
					{#if signOutError}
						<span role="alert" class="text-danger">{signOutError}</span>
					{/if}
					<Button variant="quiet" busy={signingOut} onclick={signOut}>Sign out</Button>
				</div>
			{/if}
		</div>
	</header>

	<main class="mx-auto w-full max-w-7xl flex-1 px-4 py-6 sm:px-6">
		{@render children()}
	</main>
</div>
