<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import type { PathnameWithSearchOrHash } from '$app/types';
	import CardPage from '#lib/components/CardPage.svelte';

	const notFound = $derived(page.status === 404);
	// Retrying reloads the page in full, so the session check runs again
	// against the server rather than from a half-loaded app.
	const here = $derived(
		resolve((page.url.pathname + page.url.search).slice(1) as PathnameWithSearchOrHash)
	);
</script>

<svelte:head>
	<title>{notFound ? 'Page not found' : 'Error'} · watcht</title>
</svelte:head>

{#if notFound}
	<CardPage title="Page not found">
		<p class="text-sm text-ink">
			There's nothing at <span class="font-medium wrap-anywhere">{page.url.pathname}</span>. Check
			the address, or start again from the dashboard.
		</p>
		<a
			href={resolve('/(app)')}
			class="mt-5 inline-flex h-10 w-full items-center justify-center rounded-md bg-accent px-4 text-sm font-medium text-on-accent transition-colors hover:bg-accent-hover"
		>
			Go to the dashboard
		</a>
	</CardPage>
{:else}
	<CardPage title="Can't load this page">
		<p class="text-sm text-ink">
			The watcht server didn't answer, or it ran into a problem. If it's restarting, try again in a
			moment.
		</p>
		<p class="mt-2 text-xs text-muted">
			Error {page.status}{page.error?.message ? `: ${page.error.message}` : ''}
		</p>
		<a
			href={here}
			data-sveltekit-reload
			class="mt-5 inline-flex h-10 w-full items-center justify-center rounded-md bg-accent px-4 text-sm font-medium text-on-accent transition-colors hover:bg-accent-hover"
		>
			Try again
		</a>
	</CardPage>
{/if}
