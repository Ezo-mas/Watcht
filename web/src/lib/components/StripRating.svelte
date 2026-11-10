<script lang="ts">
	interface Props {
		/** TMDB's score as a percentage ("67%", tmdbPercent). */
		rating: string;
	}

	let { rating }: Props = $props();
</script>

<!-- TMDB's score on a card's bar, at its right end: display only. -->
<span
	title="TMDB rating"
	class="ml-auto flex items-center gap-1 pr-2 text-xs font-semibold text-on-screen tabular-nums"
>
	<svg
		aria-hidden="true"
		viewBox="0 0 16 16"
		class="size-3.5 text-on-screen-accent"
		fill="currentColor"
		><path
			d="M8 14s-5.75-3.4-5.75-7.6A3.15 3.15 0 0 1 8 4.5a3.15 3.15 0 0 1 5.75 1.9C13.75 10.6 8 14 8 14z"
		/></svg
	>
	<span class="sr-only">TMDB rating</span>
	{rating}
</span>
