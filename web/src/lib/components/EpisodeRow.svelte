<script lang="ts">
	import type { Episode } from '#lib/api/types.js';
	import { formatDay } from '#lib/format.js';
	import { episodeCode, formatRuntime } from '#lib/media.js';
	import Artwork from './Artwork.svelte';
	import Badge from './Badge.svelte';

	interface Props {
		episode: Episode;
		/** Today as YYYY-MM-DD, to tell aired episodes from upcoming ones. */
		today: string;
	}

	let { episode, today }: Props = $props();

	const code = $derived(episodeCode(episode.season_number, episode.episode_number));
	const runtime = $derived(formatRuntime(episode.runtime));
	const upcoming = $derived(episode.air_date !== '' && episode.air_date > today);
</script>

<!--
	One episode in the classic layout: still, code and title, date and
	runtime, overview. Its heading is an h4, under a season's h3.
-->
<li
	class="grid grid-cols-[7.5rem_minmax(0,1fr)] gap-3 py-3 sm:grid-cols-[13rem_minmax(0,1fr)] sm:gap-4"
>
	<Artwork src={episode.still_url} kind="still" lazy class="self-start rounded-sm" />
	<div class="min-w-0">
		<h4 class="text-[0.9375rem] leading-snug font-semibold text-ink">
			<span class="mr-1 font-bold text-accent-text tabular-nums">{code}</span>
			{episode.name}
		</h4>
		<div class="mt-1 flex flex-wrap gap-1">
			{#if episode.air_date === ''}
				<Badge tone="outline">No air date</Badge>
			{:else if upcoming}
				<Badge tone="outline">Airs {formatDay(episode.air_date)}</Badge>
			{:else}
				<Badge tone="accent"
					><time datetime={episode.air_date}>{formatDay(episode.air_date)}</time></Badge
				>
			{/if}
			{#if runtime}
				<Badge><span class="sr-only">Runtime </span>{runtime}</Badge>
			{/if}
		</div>
		{#if episode.overview}
			<p class="mt-1.5 line-clamp-3 max-w-[72ch] text-sm text-muted sm:line-clamp-none">
				{episode.overview}
			</p>
		{/if}
	</div>
</li>
