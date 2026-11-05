<script lang="ts">
	import type { ResolvedPathname } from '$app/types';
	import type { Snippet } from 'svelte';
	import type { ClassValue } from 'svelte/elements';
	import Artwork from './Artwork.svelte';

	interface Props {
		/** Where the card leads (a resolve() result): the poster and the caption both open it. */
		href: ResolvedPathname;
		/** The poster URL from the API; "" shows the placeholder. */
		poster: string;
		/** The title, for the placeholder when there's no poster. */
		label: string;
		/** An id for the link, so a page can move focus to the card. */
		linkId?: string;
		/** A short mark in the poster's corner ("Plex"); decoration, so say it in the caption too. */
		corner?: string;
		/** The caption under the card, inside the link: title, then meta. */
		children: Snippet;
		/** The action bar attached under the poster: buttons, never inside the link. */
		bar?: Snippet;
		class?: ClassValue;
	}

	let { href, poster, label, linkId, corner, children, bar, class: className }: Props = $props();
</script>

<!--
	A poster card: poster, an optional dark action bar attached under it,
	and a centred caption. One link per card: the caption's link stretches
	over the whole card (its ::after), so the poster opens it too, and the
	bar sits above it. The focus ring goes round the whole card.
	`isolate` keeps the bar's z-index inside the card, under sticky headings.
-->
<div
	class={[
		'relative isolate flex flex-col rounded-sm has-[[data-card-link]:focus-visible]:outline-2 has-[[data-card-link]:focus-visible]:outline-offset-4 has-[[data-card-link]:focus-visible]:outline-focus',
		className
	]}
>
	<div class="relative">
		<Artwork
			src={poster}
			kind="poster"
			{label}
			lazy
			class={['w-full', bar ? 'rounded-t-sm' : 'rounded-sm']}
		/>
		{#if corner}
			<span
				aria-hidden="true"
				class="absolute top-1.5 left-1.5 rounded-sm border border-white/20 bg-screen/80 px-1.5 py-px text-[0.6875rem] leading-4 font-semibold text-on-screen"
				>{corner}</span
			>
		{/if}
	</div>
	{#if bar}
		<!-- Dark in both themes: the screen's text colours on `strip`. -->
		<div class="relative z-10 flex items-center gap-1 rounded-b-sm bg-strip p-1">
			{@render bar()}
		</div>
	{/if}
	<!-- The type makes callers pass a resolve()d path; the rule can't see through props. -->
	<!-- eslint-disable svelte/no-navigation-without-resolve -->
	<a
		id={linkId}
		{href}
		data-card-link
		data-sveltekit-preload-data="tap"
		class="group/link mt-2 px-1 text-center after:absolute after:inset-0 after:content-[''] focus-visible:outline-none"
	>
		{@render children()}
	</a>
	<!-- eslint-enable svelte/no-navigation-without-resolve -->
</div>
