<script lang="ts">
	import type { ResolvedPathname } from '$app/types';
	import type { Snippet } from 'svelte';
	import type { ClassValue } from 'svelte/elements';
	import { stripBar } from '#lib/strip.js';
	import Artwork from './Artwork.svelte';

	interface Props {
		/** Where the card leads (a resolve() result): the poster and the caption both open it. */
		href: ResolvedPathname;
		/** The poster URL from the API; "" shows the placeholder. */
		poster: string;
		/** The title, for the placeholder when there's no poster. */
		label: string;
		/** The card's tooltip (the link's title): the full name when the caption is short ("Breaking Bad – Season 1"). */
		tooltip?: string;
		/** An id for the link, so a page can move focus to the card. */
		linkId?: string;
		/** A short mark in the poster's corner ("Plex"); decoration, so say it in the caption too. */
		corner?: string;
		/** The caption under the card, inside the link: title, then meta. */
		children: Snippet;
		/** The action bar attached under the poster: buttons, never inside the link. */
		bar?: Snippet;
		/**
		 * How much of it is watched, 0 to 1: a thin sage bar along the
		 * poster's bottom edge (a season's progress). Decoration: say it in
		 * the caption too.
		 */
		progress?: number;
		class?: ClassValue;
	}

	let {
		href,
		poster,
		label,
		tooltip,
		linkId,
		corner,
		children,
		bar,
		progress,
		class: className
	}: Props = $props();
</script>

<!--
	A poster card: poster, an optional dark action bar attached under it,
	and a centred caption. One link per card: the caption's link stretches
	over the whole card (its ::after), so the poster opens it too, and the
	bar sits above it. The focus ring goes round the whole card.
	`isolate` keeps the bar's z-index inside the card, under sticky headings;
	while a disclosure in the bar is open (a when-choice), the card rises
	over its neighbours (z-20, still under the sticky header), so they
	don't cover it.
-->
<div
	class={[
		'relative isolate flex flex-col rounded-sm has-[[aria-expanded=true]]:z-20 has-[[data-card-link]:focus-visible]:outline-2 has-[[data-card-link]:focus-visible]:outline-offset-4 has-[[data-card-link]:focus-visible]:outline-focus',
		className
	]}
>
	<div class="relative">
		<Artwork
			src={poster}
			kind="poster"
			{label}
			lazy
			class={['w-full', bar || progress !== undefined ? 'rounded-t-sm' : 'rounded-sm']}
		/>
		{#if corner}
			<span
				aria-hidden="true"
				class="absolute top-1.5 left-1.5 rounded-sm border border-white/20 bg-screen/80 px-1.5 py-px text-[0.6875rem] leading-4 font-semibold text-on-screen"
				>{corner}</span
			>
		{/if}
	</div>
	{#if progress !== undefined}
		<div aria-hidden="true" class={['h-1 overflow-hidden bg-line', !bar && 'rounded-b-sm']}>
			<div
				class="h-full bg-watched"
				style:width="{Math.min(Math.max(progress, 0), 1) * 100}%"
			></div>
		</div>
	{/if}
	{#if bar}
		<!--
			Dark in both themes: the screen's text colours on `strip`. No
			padding: the check sits flush in its bottom-left corner.
		-->
		<div class={stripBar}>
			{@render bar()}
		</div>
	{/if}
	<!-- The type makes callers pass a resolve()d path; the rule can't see through props. -->
	<!-- eslint-disable svelte/no-navigation-without-resolve -->
	<a
		id={linkId}
		{href}
		title={tooltip}
		data-card-link
		data-sveltekit-preload-data="tap"
		class="group/link mt-2 px-1 text-center after:absolute after:inset-0 after:content-[''] focus-visible:outline-none"
	>
		{@render children()}
	</a>
	<!-- eslint-enable svelte/no-navigation-without-resolve -->
</div>
