<script lang="ts">
	import Kbd from '#lib/components/Kbd.svelte';
	import Panel from '#lib/components/Panel.svelte';
	import TmdbAttribution from '#lib/components/TmdbAttribution.svelte';
</script>

<svelte:head>
	<title>About · watcht</title>
</svelte:head>

<div class="flex max-w-3xl flex-col gap-5">
	<div>
		<h1 class="text-xl font-semibold tracking-tight text-ink">About watcht</h1>
		<p class="mt-1 max-w-[68ch] text-sm text-ink">
			watcht is a self-hosted tracker for the films and TV you watch, for one person or a household.
			It keeps your watch history, ratings, watchlist, and episode progress in one database on your
			own server.
		</p>
		<p class="mt-2 text-sm text-muted">
			To see which version this is, run <code class="font-mono text-ink">watcht version</code> on the
			server.
		</p>
	</div>

	<Panel title="Data and images">
		<p class="mb-3 max-w-[68ch] text-sm text-muted">
			Titles, artwork, and episode details come from TMDB, using the API key set on this server.
		</p>
		<TmdbAttribution />
	</Panel>

	<Panel title="Keyboard shortcuts">
		<dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-2 text-sm">
			<dt><Kbd>/</Kbd></dt>
			<dd class="text-ink">Search for a show or movie</dd>
			<dt><Kbd>↑</Kbd> <Kbd>↓</Kbd></dt>
			<dd class="text-ink">Choose a search result</dd>
			<dt><Kbd>Enter</Kbd></dt>
			<dd class="text-ink">Open the chosen result, or all results on the search page</dd>
			<dt><Kbd>Esc</Kbd></dt>
			<dd class="text-ink">Close search or a menu</dd>
		</dl>
	</Panel>
</div>
