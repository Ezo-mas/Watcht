<script lang="ts">
	// A small text mark for a ratings or link source: "IMDb" in a filled
	// box, "TMDB" in an outlined pill. watcht's own marks in its own
	// colours, not the sites' logos (TMDB's logo is far too wide to read
	// at this size, and only appears in the About attribution). Decoration:
	// the link or rating around it carries the name.
	interface Props {
		site: 'imdb' | 'tmdb';
		/** page: on the page's own surfaces; screen: on the dark title hero. */
		tone?: 'page' | 'screen';
	}

	let { site, tone = 'page' }: Props = $props();
</script>

{#if site === 'imdb'}
	<span
		aria-hidden="true"
		class={[
			'inline-block rounded-xs px-1 py-0.5 text-[0.625rem] leading-none font-black tracking-tight',
			tone === 'page' ? 'bg-ink text-surface' : 'bg-on-screen text-screen'
		]}>IMDb</span
	>
{:else}
	<span
		aria-hidden="true"
		class={[
			'inline-block rounded-full border-[1.5px] px-1 py-px text-[0.5625rem] leading-none font-extrabold tracking-wide',
			tone === 'page' ? 'border-ink text-ink' : 'border-on-screen text-on-screen'
		]}>TMDB</span
	>
{/if}
