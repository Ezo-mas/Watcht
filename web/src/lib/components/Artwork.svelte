<script lang="ts">
	import type { ClassValue } from 'svelte/elements';

	interface Props {
		/** A URL the API returned (poster_url, still_url, backdrop_url); "" when TMDB has none. */
		src: string;
		/** poster 2:3, still 16:9. Sets the box, so the layout never jumps. */
		kind: 'poster' | 'still';
		/** The title, shown on a missing poster big enough to read it. */
		label?: string;
		/** Below the fold: let the browser load it when it's near. */
		lazy?: boolean;
		class?: ClassValue;
	}

	let { src, kind, label = '', lazy = false, class: className }: Props = $props();

	// A URL that failed to load (offline, TMDB down) falls back to the
	// placeholder; a new URL gets a fresh try.
	let failed = $state('');
	const show = $derived(src !== '' && failed !== src);
	const size = $derived(kind === 'poster' ? { w: 342, h: 513 } : { w: 300, h: 169 });
</script>

<!--
	Artwork is decoration next to its title, so it has no alt text. The
	placeholder keeps the same box: a frame glyph, and the title on posters.
-->
<div
	class={[
		'relative overflow-hidden bg-raised',
		kind === 'poster' ? 'aspect-[2/3]' : 'aspect-video',
		className
	]}
>
	{#if show}
		<img
			{src}
			alt=""
			width={size.w}
			height={size.h}
			loading={lazy ? 'lazy' : 'eager'}
			decoding="async"
			referrerpolicy="no-referrer"
			onerror={() => (failed = src)}
			class="absolute inset-0 size-full object-cover"
		/>
	{:else}
		<div
			aria-hidden="true"
			class="absolute inset-0 flex flex-col items-center justify-center gap-2 p-2 text-center text-muted"
		>
			<svg
				viewBox="0 0 24 24"
				class="size-6 max-w-[40%] shrink-0 opacity-70"
				fill="none"
				stroke="currentColor"
				stroke-width="1.6"
				stroke-linejoin="round"
				><rect x="3" y="4" width="18" height="16" rx="2" /><path
					d="M7 4v16M17 4v16M3 8h4M3 12h4M3 16h4M17 8h4M17 12h4M17 16h4"
				/></svg
			>
			{#if label && kind === 'poster'}
				<span class="line-clamp-3 text-xs leading-tight font-medium max-sm:hidden">{label}</span>
			{/if}
		</div>
	{/if}
</div>
