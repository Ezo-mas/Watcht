<script lang="ts">
	import { afterNavigate, goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { untrack } from 'svelte';
	import SearchView from '#lib/components/SearchView.svelte';
	import { normalizeQuery } from '#lib/search.js';
	import { QuerySync } from '#lib/search-url.js';

	// The same search as the overlay, as a page: /search?q=… can be linked,
	// and Back returns to it. Typing replaces the history entry rather than
	// adding one per word.

	const urlQuery = $derived(page.url.searchParams.get('q') ?? '');
	let query = $state(untrack(() => page.url.searchParams.get('q') ?? ''));
	let input = $state<HTMLInputElement>();

	// Back, Forward, or a link puts the URL's query in the box; the page's
	// own URL writes don't, so typing during one isn't undone.
	const sync = new QuerySync();
	afterNavigate(({ to }) => {
		if (!to) return;
		const next = sync.arrived(to.url.searchParams.get('q') ?? '', query);
		if (next !== null) query = next;
	});

	// The box's query goes into the URL after a pause in typing. It's
	// compared with what the URL holds or is about to hold, so clearing the
	// box while a write is in flight still writes the empty query.
	$effect(() => {
		const q = normalizeQuery(query);
		if (q === sync.target(untrack(() => urlQuery))) return;
		const timer = setTimeout(() => {
			const target = q
				? resolve(`/(app)/search?q=${encodeURIComponent(q)}`)
				: resolve('/(app)/search');
			sync.wrote(q);
			void goto(target, { replace: true, reset: false });
		}, 400);
		return () => clearTimeout(timer);
	});

	$effect(() => {
		if (!untrack(() => query)) input?.focus();
	});

	const title = $derived(urlQuery ? `“${normalizeQuery(urlQuery)}” · Search` : 'Search');
</script>

<svelte:head>
	<title>{title} · watcht</title>
</svelte:head>

<div data-search-page class="flex max-w-4xl flex-col gap-4">
	<h1 class="text-xl font-semibold tracking-tight text-ink">Search</h1>
	<SearchView variant="page" bind:query bind:input />
</div>
