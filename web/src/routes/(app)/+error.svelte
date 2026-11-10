<script lang="ts">
	import { refreshAll } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import Button from '#lib/components/Button.svelte';
	import { parseSeasonParam, seasonLabel } from '#lib/media.js';
	import { searchOverlay } from '#lib/search-overlay.svelte.js';

	// Errors from signed-in pages show here, inside the header and footer,
	// so search and the menu stay one click away.

	const notFound = $derived(page.status === 404);
	// What a 404 is about comes from the error (`missing`), so a missing show
	// reached through a season's address still says so. Title pages without
	// it are told by their route.
	const missing = $derived(page.error?.missing);
	const missingHeadings = {
		show: 'Title not found',
		season: 'Season not found',
		episode: 'Episode not found'
	};
	const routeHeadings: Record<string, string> = {
		'/(app)/movies/[id]': 'Title not found',
		'/(app)/shows/[id]': 'Title not found'
	};
	const heading = $derived(
		!notFound
			? "Can't load this page"
			: missing
				? missingHeadings[missing]
				: (routeHeadings[page.route.id ?? ''] ?? 'Page not found')
	);

	// A missing season or episode can still be left for its show (it loaded),
	// and an episode for its season, when the address has a usable number.
	const showId = $derived(page.params.id ?? '');
	const seasonNumber = $derived(parseSeasonParam(page.params.n ?? null));
	const linkClass =
		'inline-flex h-10 items-center rounded-md border border-line px-4 text-sm font-medium text-ink transition-colors hover:border-line-strong hover:bg-surface';

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
			{#if missing === 'season' || missing === 'episode'}
				{#if missing === 'episode' && seasonNumber !== null}
					<a
						href={resolve('/(app)/shows/[id]/seasons/[n]', {
							id: showId,
							n: String(seasonNumber)
						})}
						class={linkClass}>Go to {seasonLabel(seasonNumber)}</a
					>
				{/if}
				<a href={resolve('/(app)/shows/[id]', { id: showId })} class={linkClass}>Go to the show</a>
			{/if}
			<Button onclick={() => (searchOverlay.open = true)}>Search for a title</Button>
			<a href={resolve('/(app)')} class={linkClass}>Go to the dashboard</a>
		{:else}
			<Button busy={retrying} onclick={retry}>Try again</Button>
		{/if}
	</div>
</div>
