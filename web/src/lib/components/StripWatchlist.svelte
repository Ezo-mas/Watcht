<script lang="ts">
	import { stripSquare } from '#lib/strip.js';

	interface Props {
		/** What it would add, for its name ("S01E02", "Season 1"); empty for the card's own title. */
		what?: string;
	}

	let { what = '' }: Props = $props();

	const label = $derived(
		what
			? `Add ${what} to watchlist (coming in a later update)`
			: 'Add to watchlist (coming in a later update)'
	);
</script>

<!--
	The watchlist button on a card's bar. The watchlist arrives in slice 3;
	until then it does nothing and says so (DESIGN.md, Placeholders).
-->
<button
	type="button"
	aria-disabled="true"
	aria-label={label}
	title="Add to watchlist (coming in a later update)"
	class={[stripSquare, 'cursor-not-allowed text-on-screen-muted/40']}
>
	<svg
		aria-hidden="true"
		viewBox="0 0 16 16"
		class="size-4"
		fill="none"
		stroke="currentColor"
		stroke-width="1.8"
		stroke-linecap="round"
		><path d="M6 4h7.5M6 8h7.5M6 12h7.5M2.5 4h.01M2.5 8h.01M2.5 12h.01" /></svg
	>
</button>
