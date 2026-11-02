<script lang="ts">
	import { resolve } from '$app/paths';
	import type { SearchResult } from '#lib/api/types.js';
	import { yearOf } from '#lib/media.js';
	import Artwork from './Artwork.svelte';
	import Badge from './Badge.svelte';

	interface Props {
		result: SearchResult;
		/** overlay: a compact listbox option; page: a plain link with the overview. */
		variant: 'overlay' | 'page';
		/** Overlay only: the option's ID and whether arrow keys have selected it. */
		id?: string;
		active?: boolean;
		onclick?: () => void;
	}

	let { result, variant, id, active = false, onclick }: Props = $props();

	const tmdbId = $derived(String(result.tmdb_id));
	const year = $derived(yearOf(result.date));
	// Search results carry no vote count; an average of 0 means no votes.
	const score = $derived(result.vote_average > 0 ? result.vote_average.toFixed(1) : '');
	const original = $derived(
		result.original_title && result.original_title !== result.title ? result.original_title : ''
	);
</script>

<!--
	Detail pages fetch and store the title from TMDB, so results don't
	preload on hover (data-sveltekit-preload-data="tap"): only a click
	or tap that's about to open one does.
-->
<a
	href={result.media_type === 'movie'
		? resolve('/(app)/movies/[id]', { id: tmdbId })
		: resolve('/(app)/shows/[id]', { id: tmdbId })}
	{id}
	{onclick}
	role={variant === 'overlay' ? 'option' : undefined}
	aria-selected={variant === 'overlay' ? active : undefined}
	tabindex={variant === 'overlay' ? -1 : undefined}
	data-sveltekit-preload-data="tap"
	class={[
		'group grid items-center gap-3 rounded-md px-2 py-1.5 transition-colors focus-visible:-outline-offset-2',
		variant === 'overlay'
			? 'grid-cols-[2.5rem_minmax(0,1fr)]'
			: 'grid-cols-[3.5rem_minmax(0,1fr)] sm:grid-cols-[4rem_minmax(0,1fr)]',
		active ? 'bg-accent/15 ring-1 ring-accent/60' : 'hover:bg-raised/70'
	]}
>
	<Artwork src={result.poster_url} kind="poster" lazy class="rounded-sm" />
	<span class="min-w-0">
		<span class="flex min-w-0 items-baseline gap-2">
			<span class="truncate font-semibold text-ink group-hover:underline">{result.title}</span>
			{#if year}<span class="shrink-0 text-sm text-muted tabular-nums">{year}</span>{/if}
		</span>
		<span class="mt-1 flex flex-wrap items-center gap-1">
			<Badge tone="accent">{result.media_type === 'movie' ? 'Movie' : 'Show'}</Badge>
			{#if score}
				<Badge title="TMDB rating">TMDB {score}</Badge>
			{/if}
			{#if original}
				<span class="truncate text-xs text-muted">{original}</span>
			{/if}
		</span>
		{#if variant === 'page' && result.overview}
			<span class="mt-1 line-clamp-2 max-w-[80ch] text-sm text-muted">{result.overview}</span>
		{/if}
	</span>
</a>
