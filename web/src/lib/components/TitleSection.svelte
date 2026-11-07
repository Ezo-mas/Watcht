<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		/** The element ID the section list scrolls to. */
		id: string;
		title: string;
		/**
		 * What isn't there yet, said under the heading in muted text
		 * ("Cast and crew are coming in a later update."). Placeholder
		 * controls in the section use `{id}-later` as their description.
		 */
		later?: string;
		/** Controls on the right of the heading (a placeholder filter). */
		filter?: Snippet;
		children: Snippet;
	}

	let { id, title, later, filter, children }: Props = $props();
</script>

<!--
	A section under a title's overview (Stats, People, Videos,
	Recommendations): the section heading (no hairline under it), a filter
	on the right, then the content.
-->
<section {id} aria-labelledby="{id}-title" class="mt-14">
	<div class="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
		<h2 id="{id}-title" class="text-lg font-semibold tracking-tight text-ink">{title}</h2>
		{#if filter}{@render filter()}{/if}
	</div>
	{#if later}
		<p id="{id}-later" class="mt-3 text-sm text-muted">{later}</p>
	{/if}
	{@render children()}
</section>
