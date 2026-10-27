<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { logout } from '#lib/api/auth.js';
	import { ApiError } from '#lib/api/client.js';
	import Button from '#lib/components/Button.svelte';
	import Wordmark from '#lib/components/Wordmark.svelte';
	import type { LayoutProps } from './$types';

	let { data, children }: LayoutProps = $props();
	const user = $derived(data.session.user);

	const settingsPath = resolve('/(app)/settings');
	const usersPath = resolve('/(app)/admin/users');
	const links = $derived([
		{ href: settingsPath, label: 'Settings' },
		...(user?.is_admin ? [{ href: usersPath, label: 'Users' }] : [])
	]);
	function current(href: string): 'page' | undefined {
		const path = page.url.pathname;
		return path === href || path.startsWith(href + '/') ? 'page' : undefined;
	}

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
		<div class="mx-auto flex h-12 w-full max-w-7xl items-center gap-3 px-4 sm:gap-6 sm:px-6">
			<a href={resolve('/(app)')} class="-mx-1 rounded-sm px-1 py-1">
				<Wordmark />
			</a>
			<!-- Search, History and Watchlist join these as their routes arrive. -->
			<nav aria-label="Main" class="flex flex-1 items-stretch gap-1 self-stretch">
				{#each links as link (link.href)}
					<!--
						The current page carries the wordmark's amber bar on the
						header's bottom edge.
					-->
					<a
						href={link.href}
						aria-current={current(link.href)}
						class="relative flex items-center rounded-sm px-2 text-sm text-muted transition-colors after:absolute after:inset-x-2 after:-bottom-px after:h-0.5 after:rounded-full after:bg-accent after:opacity-0 hover:text-ink focus-visible:-outline-offset-2 aria-[current=page]:font-medium aria-[current=page]:text-ink aria-[current=page]:after:opacity-100"
					>
						{link.label}
					</a>
				{/each}
			</nav>
			{#if user}
				<div class="flex min-w-0 items-center gap-3 text-sm">
					<span class="flex max-w-64 min-w-0 items-center gap-2" title={user.username}>
						<span
							aria-hidden="true"
							class="grid size-6 shrink-0 place-items-center rounded-full bg-accent/20 text-xs font-semibold text-accent-text uppercase"
						>
							{user.username.charAt(0)}
						</span>
						<!-- On a phone only the initial shows; the name stays for screen readers. -->
						<span class="truncate text-ink max-sm:sr-only"
							><span class="sr-only">Signed in as&nbsp;</span>{user.username}</span
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
