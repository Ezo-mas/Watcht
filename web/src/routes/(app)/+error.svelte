<script lang="ts">
	import { refreshAll } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import Button from '#lib/components/Button.svelte';
	import { searchOverlay } from '#lib/search-overlay.svelte.js';

	// Errors from signed-in pages show here, inside the header and footer,
	// so search and the menu stay one click away.

	const notFound = $derived(page.status === 404);
	const isTitle = $derived(
		page.route.id === '/(app)/movies/[id]' || page.route.id === '/(app)/shows/[id]'
	);
	const heading = $derived(
		notFound ? (isTitle ? 'Title not found' : 'Page not found') : "Can't load this page"
	);

	let retrying = $state(false);
	async function retry() {
		retrying = true;
		try {
			await refreshAll();
		} finally {
			retrying = false;
		}
	}
</script>

<svelte:head>
	<title>{heading} · watcht</title>
</svelte:head>

<div class="max-w-xl py-6 sm:py-10">
	<p class="text-sm font-semibold text-accent-text tabular-nums">Error {page.status}</p>
	<h1 class="mt-1 text-2xl font-semibold tracking-tight text-ink">{heading}</h1>
	<p class="mt-2 text-sm text-ink first-letter:uppercase">
		{page.error?.message || 'Something went wrong.'}
	</p>
	<div class="mt-5 flex flex-wrap gap-2">
		{#if notFound}
			<Button onclick={() => (searchOverlay.open = true)}>Search for a title</Button>
			<a
				href={resolve('/(app)')}
				class="inline-flex h-10 items-center rounded-md border border-line px-4 text-sm font-medium text-ink transition-colors hover:border-line-strong hover:bg-surface"
				>Go to the dashboard</a
			>
		{:else}
			<Button busy={retrying} onclick={retry}>Try again</Button>
		{/if}
	</div>
</div>
